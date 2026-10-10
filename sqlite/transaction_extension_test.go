package sqlite_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func savepointTableImages(t *testing.T, db *sqlite.DB) map[string][][]any {
	t.Helper()
	schema, err := db.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][][]any{}
	for _, entry := range schema {
		if entry.Type == "table" {
			result[entry.Name] = query(t, db, `SELECT * FROM "`+entry.Name+`" ORDER BY rowid`)
		}
	}
	return result
}

func TestTransactionSavepointsPreserveGramsHNSWAndIVF(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			db, path := writable(t, sqlite.Options{WALShared: shared})
			execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),note TEXT)")
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for id := 1; id <= 24; id++ {
				txExecute(t, tx, "INSERT INTO items VALUES(?,?,?)", id, fmt.Sprintf("[%d,%d]", id, id%5), fmt.Sprintf("数据库%d", id))
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateGramsIndex(ctx, "grams", "items", []string{"note"}); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateHNSWIndex(ctx, "hnsw", "items", "v"); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateIVFFlatIndex(ctx, "ivf", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 2}); err != nil {
				t.Fatal(err)
			}
			execute(t, db, "SAVEPOINT whole")
			// Reads are the first access after SAVEPOINT. Native integrity APIs must
			// materialize the same lazy snapshot as SQL, then retain it across rollback.
			for _, name := range []string{"grams", "hnsw", "ivf"} {
				if err = db.CheckExtensionIndex(ctx, name); err != nil {
					t.Fatal(name, err)
				}
			}
			original := savepointTableImages(t, db)
			for round := 0; round < 2; round++ {
				execute(t, db, "SAVEPOINT changes")
				execute(t, db, "UPDATE items SET id=-100,v='[99,99]',note='改变数据库' WHERE id=1")
				execute(t, db, "DELETE FROM items WHERE id=2")
				execute(t, db, "INSERT INTO items VALUES(2,'[88,88]','复用行号'),(55,'[55,55]','新增行')")
				for _, name := range []string{"grams", "hnsw", "ivf"} {
					if err = db.CheckExtensionIndex(ctx, name); err != nil {
						t.Fatal(name, err)
					}
				}
				execute(t, db, "SAVEPOINT rebuilding")
				if err = db.CreateGramsIndex(ctx, "additional", "items", []string{"note"}); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"grams", "hnsw", "ivf"} {
					execute(t, db, "REINDEX "+name)
				}
				execute(t, db, "CREATE TABLE added(id INTEGER PRIMARY KEY)")
				execute(t, db, "CREATE INDEX added_ix ON added(id)")
				execute(t, db, "ROLLBACK TO changes")
				for _, name := range []string{"grams", "hnsw", "ivf"} {
					if err = db.CheckExtensionIndex(ctx, name); err != nil {
						t.Fatal(name, err)
					}
				}
				if got := savepointTableImages(t, db); !reflect.DeepEqual(got, original) {
					t.Fatal("source or extension generation/pages differ after savepoint rollback")
				}
				execute(t, db, "RELEASE changes")
			}
			execute(t, db, "UPDATE items SET note='committed数据库' WHERE id=3")
			execute(t, db, "RELEASE whole")
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sqlite.Open(path, sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, name := range []string{"grams", "hnsw", "ivf"} {
				if err = db.CheckExtensionIndex(ctx, name); err != nil {
					t.Fatal(name, err)
				}
			}
			if got := query(t, db, "SELECT id FROM items WHERE note LIKE '%committed%' ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(3)}}) {
				t.Fatal(got)
			}
		})
	}
}
