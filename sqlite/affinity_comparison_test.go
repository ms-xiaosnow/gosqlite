package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

// Intact ordinary databases, statements, rollback and close/reopen only.
func TestAffinityComparisonPublicRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "affinity.db")
	db, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if db != nil {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	for _, q := range []string{"CREATE TABLE f(id INTEGER PRIMARY KEY,t TEXT,b BLOB,u)", "CREATE TABLE one(y INTEGER)", "INSERT INTO one VALUES(1)", "INSERT INTO f VALUES(1,1,1,1),(2,1.0,1.0,1.0),(3,'1','1','1'),(4,NULL,NULL,NULL)", "CREATE INDEX ft ON f(t)", "CREATE INDEX fb ON f(b)"} {
		if _, err := db.Exec(context.Background(), q); err != nil {
			t.Fatal(q, err)
		}
	}
	check := func(q string, want [][]any, args ...any) {
		t.Helper()
		rows, err := db.Query(context.Background(), q, args...)
		if err != nil {
			t.Fatal(q, err)
		}
		var got [][]any
		for rows.Next() {
			got = append(got, rows.Values())
		}
		err = rows.Err()
		cerr := rows.Close()
		if err != nil || cerr != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v (%v,%v), want %#v", q, got, err, cerr, want)
		}
	}
	all := [][]any{{int64(1), int64(0), int64(0), int64(1), int64(1), "integer", "1", int64(1)}, {int64(2), int64(0), int64(0), int64(1), int64(1), "real", "1.0", float64(1)}, {int64(3), int64(1), int64(1), int64(1), int64(1), "text", "1", "1"}, {int64(4), nil, nil, nil, nil, "null", nil, nil}}
	for _, from := range []string{"f", "f NOT INDEXED", "f,one"} {
		check("SELECT id,t=b,t=u,t=+b,t=+u,typeof(b),t,b FROM "+from+" ORDER BY id", all)
		check("SELECT id FROM "+from+" WHERE t=b ORDER BY id", [][]any{{int64(3)}})
		check("SELECT id FROM "+from+" WHERE b=1 AND t=b ORDER BY id", nil)
		check("SELECT id FROM "+from+" WHERE b=1 AND b=t ORDER BY id", nil)
		check("SELECT id FROM "+from+" WHERE b=1 AND b!=t ORDER BY id", [][]any{{int64(1)}, {int64(2)}})
		check("SELECT id FROM "+from+" WHERE t=1.0 AND t=u ORDER BY id", nil)
		check("SELECT id,b AS alias FROM "+from+" WHERE t=alias ORDER BY id", [][]any{{int64(3), "1"}})
		check("SELECT id,+b AS alias FROM "+from+" WHERE t=alias ORDER BY id", [][]any{{int64(1), int64(1)}, {int64(2), float64(1)}, {int64(3), "1"}})
		check("SELECT id,t,b,typeof(b) FROM "+from+" WHERE t=?1 ORDER BY id", [][]any{{int64(2), "1.0", float64(1), "real"}}, float64(1))
		check("SELECT id FROM "+from+" WHERE b=?1 AND t=b ORDER BY id", nil, int64(1))
	}
	check("SELECT f.id,one.y FROM f LEFT JOIN one ON t=b ORDER BY f.id", [][]any{{int64(1), nil}, {int64(2), nil}, {int64(3), int64(1)}, {int64(4), nil}})
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	res, err := tx.Exec(context.Background(), "UPDATE f SET u='changed' WHERE t=b")
	if err != nil {
		t.Fatal(err)
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		t.Fatalf("UPDATE affected %d: %v", n, err)
	}
	res, err = tx.Exec(context.Background(), "DELETE FROM f WHERE t!=b")
	if err != nil {
		t.Fatal(err)
	}
	n, err = res.RowsAffected()
	if err != nil || n != 2 {
		t.Fatalf("DELETE affected %d: %v", n, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	check("SELECT id,t=b,t=u,t=+b,t=+u,typeof(b),t,b FROM f ORDER BY id", all)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = nil
	db, err = Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	check("SELECT id,t=b,t=u,t=+b,t=+u,typeof(b),t,b FROM f ORDER BY id", all)
}
