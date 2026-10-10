package sqlite

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Intact ordinary SQL plus pre-canceled and memory/work-bounded queries only.
func TestBooleanAliasOnNativeProvenance(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	for _, q := range []string{`CREATE TABLE l(id INTEGER,flag INTEGER)`, `INSERT INTO l VALUES(1,0),(2,1),(3,NULL)`, `CREATE TABLE u(y INTEGER)`, `INSERT INTO u VALUES(7)`} {
		reviewVectorExec(t, db, q)
	}
	for _, tc := range []struct {
		q    string
		want [][]any
	}{
		{`SELECT l.id,0 AS true,u.y FROM l LEFT JOIN u ON true`, nil},
		{`SELECT l.id,NULL AS false,u.y FROM l LEFT JOIN u ON false`, nil},
		{`SELECT l.id,FALSE,u.y FROM l LEFT JOIN u ON FALSE ORDER BY l.id`, [][]any{{int64(1), int64(0), nil}, {int64(2), int64(0), nil}, {int64(3), int64(0), nil}}},
		{`SELECT l.id,flag AS z,u.y FROM l LEFT JOIN u ON z ORDER BY l.id`, [][]any{{int64(2), int64(1), int64(7)}}},
		{`SELECT l.id,flag AS z,u.y FROM l LEFT JOIN u ON +z ORDER BY l.id`, [][]any{{int64(1), int64(0), nil}, {int64(2), int64(1), int64(7)}, {int64(3), nil, nil}}},
		{`SELECT l.id,0 AS z,abs(-9223372036854775808) FROM l LEFT JOIN u ON z`, nil},
	} {
		got, _, _ := reviewVectorQuery(t, db, tc.q)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Error(tc.q, got)
		}
	}
	q := `SELECT ?1 AS true,u.y FROM l LEFT JOIN u ON true`
	for _, v := range []any{int64(0), int64(1), nil, int64(0)} {
		rows, err := db.Query(context.Background(), q, v)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for rows.Next() {
			n++
		}
		err = rows.Err()
		rows.Close()
		want := 0
		if v == int64(1) {
			want = 3
		}
		if err != nil || n != want {
			t.Fatal(v, n, err)
		}
	}
	// Full binding and forward-reference checks still precede false startup.
	for _, query := range []string{
		`SELECT later.y AS true FROM l LEFT JOIN u ON true JOIN u later ON 1 WHERE 0`,
		`SELECT later.y AS true FROM l LEFT JOIN u ON true JOIN u later ON 1 LIMIT 0`,
	} {
		rows, err := db.Query(context.Background(), query)
		if !errors.Is(err, ErrUnsupported) {
			if rows != nil {
				rows.Close()
			}
			t.Fatal("forward alias reference escaped validation", query, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rows, err := db.Query(ctx, q, int64(0)); !errors.Is(err, context.Canceled) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal(err)
	}
	ctx = WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 10})
	if rows, err := db.Query(ctx, q, int64(0)); !errors.Is(err, ErrQueryWorkLimit) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal(err)
	}
	got, _, _ := reviewVectorQuery(t, db, `SELECT l.id,1 AS z,u.y FROM l LEFT JOIN u ON z ORDER BY l.id`)
	if len(got) != 3 {
		t.Fatal(got)
	}
}
