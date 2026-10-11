//go:build oracle

package btree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
)

// Only ordinary successful transactions and the official read/check oracle
// execute here. No database crash, disk corruption, or I/O-failure workflow.
func mutableOracle(t *testing.T, path, sql string) string {
	t.Helper()
	exe := os.Getenv("SQLITE_ORACLE")
	if exe == "" {
		t.Fatal("oracle acceptance requires explicit SQLITE_ORACLE")
	}
	cmd := testoracle.ForTest(t, exe, "-batch", "-bail", "-noheader", "-list", path)
	cmd.Stdin = strings.NewReader(sql + "\n")
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("oracle %s: %v\n%s", sql, e, out)
	}
	return strings.TrimSpace(string(out))
}
func TestMutableDurableOracle(t *testing.T) {
	ctx := context.Background()
	exe := os.Getenv("SQLITE_ORACLE")
	if exe == "" {
		t.Fatal("oracle acceptance requires explicit SQLITE_ORACLE")
	}
	if version, e := testoracle.ForTest(t, exe, "-version").Output(); e != nil || !strings.HasPrefix(string(version), buildinfo.SQLiteBaseline+" ") {
		t.Fatalf("wrong official oracle: %s %v", version, e)
	}
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "native.db")
			tx, e := pager.OpenTx(ctx, path, pager.Options{Create: true, PageSize: size, MaxDirtyPages: 16384})
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Close()
			var root, index uint32
			e = tx.Statement(ctx, func() error {
				var err error
				root, err = CreateTree(ctx, tx, TableTree)
				if err != nil {
					return err
				}
				index, err = CreateTree(ctx, tx, IndexTree)
				if err != nil {
					return err
				}
				if err = InsertTable(ctx, tx, 1, 1, []any{"table", "records", "records", int64(root), "CREATE TABLE records(id INTEGER PRIMARY KEY, label TEXT)"}); err != nil {
					return err
				}
				if err = InsertTable(ctx, tx, 1, 2, []any{"index", "labels", "records", int64(index), "CREATE INDEX labels ON records(label)"}); err != nil {
					return err
				}
				return tx.BumpSchemaCookie(ctx)
			})
			if e != nil {
				t.Fatal(e)
			}
			model := map[int64]string{}
			for i := int64(1); i <= 1000; i++ {
				label := fmt.Sprintf("%04d:%s", i%113, strings.Repeat("界", int(i%180)))
				if i == 500 {
					label = strings.Repeat("overflow", 26000)
				}
				e = tx.Statement(ctx, func() error {
					if err := InsertTable(ctx, tx, root, i, []any{nil, label}); err != nil {
						return err
					}
					return InsertIndex(ctx, tx, index, KeyInfo{1}, []any{label, i})
				})
				if e != nil {
					t.Fatalf("insert %d: %v", i, e)
				}
				model[i] = label
			}
			if e = tx.Commit(ctx); e != nil {
				t.Fatal(e)
			}
			if got := mutableOracle(t, path, "PRAGMA integrity_check; SELECT count(*) FROM records; SELECT count(*) FROM records INDEXED BY labels;"); got != "ok\n1000\n1000" {
				t.Fatal(got)
			}
			// Official SQLite mutates the Go-generated native file successfully.
			if got := mutableOracle(t, path, "INSERT INTO records VALUES(1001,'official'); DELETE FROM records WHERE id=1; PRAGMA integrity_check;"); got != "ok" {
				t.Fatal(got)
			}
			delete(model, 1)
			model[1001] = "official"
			tx, e = pager.OpenTx(ctx, path, pager.Options{MaxDirtyPages: 16384})
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Close()
			// Delete alternating rows, including the large overflow record, then update
			// the remaining table records and every associated index key.
			for i := int64(2); i <= 1001; i++ {
				old := model[i]
				e = tx.Statement(ctx, func() error {
					if ok, err := DeleteIndex(ctx, tx, index, KeyInfo{1}, []any{old, i}); err != nil || !ok {
						return fmt.Errorf("index delete %d: %v %v", i, ok, err)
					}
					if i%2 == 0 {
						ok, err := DeleteTable(ctx, tx, root, i)
						if err != nil || !ok {
							return fmt.Errorf("table delete %d: %v %v", i, ok, err)
						}
						delete(model, i)
						return nil
					}
					label := fmt.Sprintf("changed:%04d:%s", i, strings.Repeat("a", int(i%220)))
					if err := ReplaceTable(ctx, tx, root, i, []any{nil, label}); err != nil {
						return err
					}
					if err := InsertIndex(ctx, tx, index, KeyInfo{1}, []any{label, i}); err != nil {
						return err
					}
					model[i] = label
					return nil
				})
				if e != nil {
					t.Fatalf("mutate %d: %v", i, e)
				}
			}
			if e = tx.Commit(ctx); e != nil {
				t.Fatal(e)
			}
			if got := mutableOracle(t, path, "PRAGMA integrity_check; SELECT count(*) FROM records; SELECT count(*) FROM records INDEXED BY labels;"); got != "ok\n500\n500" {
				t.Fatal(got)
			}
			tx, e = pager.OpenTx(ctx, path, pager.Options{MaxDirtyPages: 16384})
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Close()
			for i, label := range model {
				e = tx.Statement(ctx, func() error {
					if _, err := DeleteTable(ctx, tx, root, i); err != nil {
						return err
					}
					_, err := DeleteIndex(ctx, tx, index, KeyInfo{1}, []any{label, i})
					return err
				})
				if e != nil {
					t.Fatal(e)
				}
			}
			if e = tx.Commit(ctx); e != nil {
				t.Fatal(e)
			}
			if got := mutableOracle(t, path, "PRAGMA integrity_check; SELECT count(*) FROM records;"); got != "ok\n0" {
				t.Fatal(got)
			}
		})
	}
}
