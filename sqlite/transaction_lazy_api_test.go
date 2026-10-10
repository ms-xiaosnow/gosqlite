package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestTransactionLazyNativeReadEntryPoints(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),note TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2]','数据库')")
	if err := db.CreateGramsIndex(ctx, "grams", "items", []string{"note"}); err != nil {
		t.Fatal(err)
	}
	for _, native := range []bool{false, true} {
		for _, api := range []string{"schema", "vector", "similarity", "check", "match"} {
			t.Run(fmt.Sprintf("native%t/%s", native, api), func(t *testing.T) {
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				switch api {
				case "schema":
					_, err = db.Schema(ctx)
				case "vector":
					if native {
						_, err = tx.VectorSearch(ctx, "items", "v", "[1,2]", sqlite.VectorSearchOptions{Limit: 1})
					} else {
						_, err = db.VectorSearch(ctx, "items", "v", "[1,2]", sqlite.VectorSearchOptions{Limit: 1})
					}
				case "similarity":
					opts := sqlite.SimilarityOptions{Threshold: 0.5, Limit: 1}
					if native {
						_, err = tx.SimilaritySearch(ctx, "grams", "note", "数据库", opts)
					} else {
						_, err = db.SimilaritySearch(ctx, "grams", "note", "数据库", opts)
					}
				case "check":
					if native {
						err = tx.CheckExtensionIndex(ctx, "grams")
					} else {
						err = db.CheckExtensionIndex(ctx, "grams")
					}
				case "match":
					var rows *sqlite.Rows
					if native {
						rows, err = tx.Query(ctx, "SELECT grams13_match('grams','数据库','数据库')")
					} else {
						rows, err = db.Query(ctx, "SELECT grams13_match('grams','数据库','数据库')")
					}
					if err == nil {
						if !rows.Next() || rows.Values()[0] != int64(1) {
							t.Fatal(rows.Values(), rows.Err())
						}
						err = rows.Close()
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestTransactionSavepointsEncryptedOrdinaryReopen(t *testing.T) {
	ctx := context.Background()
	for _, wal := range []bool{false, true} {
		t.Run(fmt.Sprint(wal), func(t *testing.T) {
			cfg := encryptionOptions(t, encryption.RawKey256)
			if wal {
				cfg.Mode = sqlite.EncryptionWAL
			}
			opts := sqlite.Options{Encryption: &cfg, PageSize: 512}
			path := filepath.Join(t.TempDir(), "savepoints.gse")
			db, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
			execute(t, db, "INSERT INTO t VALUES(1,'base')")
			execute(t, db, "SAVEPOINT root")
			execute(t, db, "UPDATE t SET v='kept' WHERE id=1")
			execute(t, db, "SAVEPOINT nested")
			execute(t, db, "UPDATE t SET v='undone' WHERE id=1")
			execute(t, db, "CREATE TABLE gone(id INTEGER PRIMARY KEY)")
			execute(t, db, "ROLLBACK TO nested")
			if got := query(t, db, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"kept"}}) {
				t.Fatal(got)
			}
			execute(t, db, "RELEASE root")
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if got := query(t, db, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"kept"}}) {
				t.Fatal(got)
			}
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
