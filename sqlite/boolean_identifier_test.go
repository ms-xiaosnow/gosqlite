package sqlite

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// Intact SQL-created files only, using normal commit/rollback and close/reopen.
func TestBooleanIdentifierNativeLifecycle(t *testing.T) {
	db, path := reviewVectorDB(t, Options{})
	for _, q := range []string{
		`CREATE TABLE true(id INTEGER PRIMARY KEY, TrUe INTEGER DEFAULT TRUE, FaLsE INTEGER DEFAULT FALSE, "NULL" TEXT)`,
		`CREATE TABLE false(id INTEGER PRIMARY KEY, true INTEGER)`,
		`CREATE INDEX truth_index ON true(true,false)`,
		`INSERT INTO true(id,"NULL") VALUES(1,'n')`,
		`INSERT INTO true VALUES(2,0,2,NULL),(3,NULL,NULL,'x')`,
		`INSERT INTO false VALUES(1,7)`,
	} {
		reviewVectorExec(t, db, q)
	}
	for _, tc := range []struct {
		q    string
		want [][]any
	}{
		{`SELECT true,false,"NULL",2 IS TRUE,2 IS FALSE FROM true ORDER BY id`, [][]any{{int64(1), int64(0), "n", int64(0), int64(0)}, {int64(0), int64(2), nil, int64(0), int64(1)}, {nil, nil, "x", int64(0), int64(0)}}},
		{`SELECT false AS true FROM true WHERE true ORDER BY true`, [][]any{{int64(0)}}},
		{`SELECT -id AS true FROM true ORDER BY true`, [][]any{{int64(-3)}, {int64(-2)}, {int64(-1)}}},
		{`SELECT 0 AS true FROM false WHERE true`, [][]any{{int64(0)}}},
		{`SELECT true.true,false.true FROM true AS true LEFT JOIN false AS false ON true.id=false.id ORDER BY true.id`, [][]any{{int64(1), int64(7)}, {int64(0), nil}, {nil, nil}}},
		{`SELECT 0 AS true, true, 1 AS false, false, NULL`, [][]any{{int64(0), int64(1), int64(1), int64(0), nil}}},
		{`SELECT 2 IS TRUE,2 IS FALSE,2 IS +TRUE,NULL IS TRUE,NULL IS NOT FALSE`, [][]any{{int64(1), int64(0), int64(0), int64(0), int64(1)}}},
	} {
		got, _, _ := reviewVectorQuery(t, db, tc.q)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %#v; want %#v", tc.q, got, tc.want)
		}
	}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `UPDATE true SET true=false,false=?1 WHERE id=2`, int64(9)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `DELETE FROM true WHERE false IS NULL`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	reviewVectorExec(t, db, `UPDATE true SET false=true+1 WHERE id=1`)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, _, _ := reviewVectorQuery(t, db, `SELECT id,true,false FROM true ORDER BY id`)
	want := [][]any{{int64(1), int64(1), int64(2)}, {int64(2), int64(0), int64(2)}, {int64(3), nil, nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	for _, q := range []string{`SELECT true()`, `SELECT false(NULL)`, `SELECT TrUe(1) FROM true LIMIT 0`} {
		rows, err := db.Query(context.Background(), q)
		if err == nil {
			rows.Close()
			t.Error("unregistered function accepted", q)
		} else if !strings.Contains(err.Error(), "function") {
			t.Error(q, err)
		}
	}
	for _, q := range []string{`SELECT true.true FROM false`, `SELECT "true"`, `SELECT [false]`, `SELECT true FROM true JOIN false ON 1 LIMIT 0`} {
		rows, err := db.Query(context.Background(), q)
		if err == nil {
			rows.Close()
			t.Error("invalid or ambiguous column accepted", q)
		}
	}
}
