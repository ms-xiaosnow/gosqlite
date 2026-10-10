package sqlite_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func fixture(size int) string {
	return filepath.Join("..", "testdata", "readonly", fmt.Sprintf("page%d.db", size))
}
func openFixture(t *testing.T, options sqlite.Options) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(fixture(4096), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}
func query(t *testing.T, db *sqlite.DB, text string, args ...any) [][]any {
	t.Helper()
	rows, err := db.Query(context.Background(), text, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := [][]any{}
	for rows.Next() {
		result = append(result, rows.Values())
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestOracleFixturesUnchanged(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "readonly", "samples.json"))
	if err != nil {
		t.Fatal(err)
	}
	var oracle []struct {
		ID          int64    `json:"id"`
		Label       *string  `json:"label"`
		PayloadHex  string   `json:"payload_hex"`
		PayloadNull int      `json:"payload_null"`
		Score       *float64 `json:"score"`
		Optional    *string  `json:"optional"`
	}
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			before, err := os.ReadFile(fixture(size))
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlite.Open(fixture(size), sqlite.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if db.Info().PageSize != uint32(size) {
				t.Fatal(db.Info())
			}
			rows, err := db.Query(context.Background(), "SELECT id,label,payload,score,optional FROM samples ORDER BY id")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if want := []string{"id", "label", "payload", "score", "optional"}; !reflect.DeepEqual(rows.Columns(), want) {
				t.Fatalf("columns %v", rows.Columns())
			}
			i := 0
			for rows.Next() {
				if i >= len(oracle) {
					t.Fatal("extra row")
				}
				o := oracle[i]
				v := rows.Values()
				if v[0] != o.ID {
					t.Fatalf("row %d id=%v want=%v", i, v[0], o.ID)
				}
				checkNullable(t, i, "label", v[1], o.Label)
				if o.PayloadNull != 0 {
					if v[2] != nil {
						t.Fatalf("row %d non-NULL BLOB", i)
					}
				} else {
					b, ok := v[2].([]byte)
					if !ok || strings.ToUpper(hex.EncodeToString(b)) != o.PayloadHex {
						t.Fatalf("row %d BLOB mismatch %T", i, v[2])
					}
				}
				if o.Score == nil {
					if v[3] != nil {
						t.Fatalf("row %d score mismatch", i)
					}
				} else {
					f, ok := v[3].(float64)
					if !ok || f != *o.Score {
						t.Fatalf("row %d score %T %v want %v", i, v[3], v[3], *o.Score)
					}
				}
				checkNullable(t, i, "optional", v[4], o.Optional)
				i++
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if i != len(oracle) {
				t.Fatalf("rows=%d want=%d", i, len(oracle))
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(fixture(size))
			if err != nil {
				t.Fatal(err)
			}
			if sha256.Sum256(before) != sha256.Sum256(after) {
				t.Fatal("source database modified")
			}
		})
	}
}
func checkNullable(t *testing.T, row int, column string, value any, want *string) {
	t.Helper()
	if want == nil {
		if value != nil {
			t.Fatalf("row %d %s not NULL", row, column)
		}
	} else if value != *want {
		t.Fatalf("row %d %s mismatch", row, column)
	}
}
func TestSelectBindingOrderingAndAffinity(t *testing.T) {
	db := openFixture(t, sqlite.Options{})
	cases := []struct {
		sql  string
		args []any
		want [][]any
	}{
		{"SELECT id+1 AS n FROM samples WHERE n>2 AND id<4 ORDER BY n", nil, [][]any{{int64(3)}, {int64(4)}}},
		{"SELECT -id AS id FROM samples WHERE id>=1 AND id<=3 ORDER BY id+0", nil, [][]any{{int64(-1)}, {int64(-2)}, {int64(-3)}}},
		{"SELECT -id AS id FROM samples WHERE id>=1 AND id<=3 ORDER BY id", nil, [][]any{{int64(-3)}, {int64(-2)}, {int64(-1)}}},
		{"SELECT id, score, typeof(score) FROM samples WHERE id = '8'", nil, [][]any{{int64(8), float64(1), "real"}}},
		{"SELECT s.id AS n, s.label FROM samples AS s WHERE s.id >= ? AND s.id < ? ORDER BY n DESC LIMIT ? OFFSET ?", []any{1, 6, 2, 1}, [][]any{{int64(4), "row-0004"}, {int64(3), "row-0003"}}},
		{"SELECT value FROM numbers ORDER BY 1 DESC LIMIT 2", nil, [][]any{{int64(math.MaxInt64)}, {int64(140737488355328)}}},
		{"SELECT :a, :a+@b, ?3", []any{sql.Named("a", 4), sql.Named("b", 5), 6}, [][]any{{int64(4), int64(9), int64(6)}}},
		{"SELECT ?3", []any{1, 2, 3}, [][]any{{int64(3)}}},
		{"SELECT 1 WHERE NULL", nil, [][]any{}},
		{"SELECT id FROM samples ORDER BY id LIMIT 0", nil, [][]any{}},
		{"SELECT id FROM samples WHERE id >= 1 AND id < 3 LIMIT '2.0' OFFSET -1", nil, [][]any{{int64(1)}, {int64(2)}}},
		{"SELECT rowid, _rowid_, oid, id FROM samples WHERE id=1", nil, [][]any{{int64(1), int64(1), int64(1), int64(1)}}},
		{"SELECT \"a\"\"b\", \"中文\" FROM quotes", nil, [][]any{{"it's \"quoted\"", "日本語 한국어 中文 🙂"}}},
		{"SELECT id FROM samples WHERE label LIKE ? ORDER BY id LIMIT 1", []any{"%数据库%"}, [][]any{{int64(29)}}},
	}
	for _, tc := range cases {
		t.Run(tc.sql, func(t *testing.T) {
			got := query(t, db, tc.sql, tc.args...)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
		})
	}
}
func TestSchemaAndWildcard(t *testing.T) {
	db := openFixture(t, sqlite.Options{})
	schema, err := db.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema) != 4 {
		t.Fatalf("schema %v", schema)
	}
	got := query(t, db, "SELECT name FROM sqlite_schema WHERE type = ? ORDER BY name", "table")
	if want := [][]any{{"numbers"}, {"quotes"}, {"samples"}}; !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	rows, err := db.Query(context.Background(), "SELECT s.* FROM samples AS s WHERE id=1")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() || len(rows.Values()) != 5 {
		t.Fatal("wildcard")
	}
	values := rows.Values()
	values[2].([]byte)[0] = 127
	if rows.Values()[2].([]byte)[0] != 0 {
		t.Fatal("Values leaked mutable BLOB")
	}
	columns := rows.Columns()
	columns[0] = "changed"
	if rows.Columns()[0] != "id" {
		t.Fatal("Columns leaked slice")
	}
}

// The unqualified JOIN form was formerly rejected before the relational slice.
// Keep a positive regression while USING remains an explicit unsupported form.
func TestRelationalReadonlyFixtureJoin(t *testing.T) {
	db := openFixture(t, sqlite.Options{})
	got := query(t, db, "SELECT id FROM samples JOIN numbers WHERE id=1 LIMIT 1")
	if want := [][]any{{int64(1)}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRejectionsAndCodes(t *testing.T) {
	db := openFixture(t, sqlite.Options{})
	for _, text := range []string{"SELECT absent FROM samples WHERE 0", "SELECT absent() FROM samples WHERE 0", "SELECT length() FROM samples WHERE 0", "SELECT count(*) FROM samples", "SELECT id FROM samples ORDER BY 0", "SELECT id FROM samples ORDER BY -1", "SELECT id FROM samples LIMIT NULL", "SELECT id FROM samples LIMIT 1.5", "SELECT *", "SELECT samples.id FROM samples AS s", "SELECT ?", "SELECT 1; SELECT 2", "SELECT id FROM samples JOIN numbers USING(id)"} {
		t.Run(text, func(t *testing.T) {
			rows, err := db.Query(context.Background(), text)
			if err == nil {
				rows.Close()
				t.Fatal("accepted invalid/unsupported query")
			}
		})
	}
	for _, text := range []string{"CREATE TABLE t(a)", "INSERT INTO samples(id) VALUES(5)", "DELETE FROM samples", "BEGIN"} {
		_, err := db.Query(context.Background(), text)
		if !errors.Is(err, sqlite.ErrReadOnly) {
			t.Fatalf("%s: %v", text, err)
		}
		var typed *sqlite.Error
		if !errors.As(err, &typed) || typed.Code != sqlite.CodeReadOnly {
			t.Fatal(err)
		}
	}
	for _, args := range [][]any{{1, 2}, {sql.Named("missing", 1)}} {
		if _, err := db.Query(context.Background(), "SELECT ?", args...); err == nil {
			t.Fatal("bad binding accepted")
		}
	}
}
func TestStreamingMemoryCancellationAndClosure(t *testing.T) {
	db := openFixture(t, sqlite.Options{MaxSortBytes: 1})
	rows, err := db.Query(context.Background(), "SELECT id FROM samples LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	rows.Close()
	if rows.Next() || rows.Err() != nil {
		t.Fatal("closed iterator")
	}
	if _, err := db.Query(context.Background(), "SELECT id FROM samples ORDER BY id"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rows, err = db.Query(ctx, "SELECT id FROM samples")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	cancel()
	if rows.Next() || !errors.Is(rows.Err(), context.Canceled) {
		t.Fatal(rows.Err())
	}
	if _, err := db.Query(ctx, "SELECT 1"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	rows, err = db.Query(context.Background(), "SELECT id FROM samples")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrClosed) {
		t.Fatal(rows.Err())
	}
	if _, err := db.Query(context.Background(), "SELECT 1"); !errors.Is(err, sqlite.ErrClosed) {
		t.Fatal(err)
	}
}
func TestConcurrentQueries(t *testing.T) {
	db := openFixture(t, sqlite.Options{})
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				rows, err := db.Query(context.Background(), "SELECT id FROM samples WHERE id > ? LIMIT 4", i)
				if err != nil {
					t.Error(err)
					return
				}
				count := 0
				for rows.Next() {
					count++
				}
				if err := rows.Err(); err != nil {
					t.Error(err)
				}
				rows.Close()
				if count != 4 {
					t.Errorf("rows %d", count)
				}
			}
		}()
	}
	wg.Wait()
}

func TestSchemaEdgesAndResultLabels(t *testing.T) {
	db, err := sqlite.Open(filepath.Join("testdata", "schema.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cases := []struct {
		text string
		want [][]any
	}{
		{"SELECT TRUE,FALSE,x IS TRUE,x IS FALSE FROM flags", [][]any{{int64(7), int64(8), int64(1), int64(0)}}},
		{"SELECT rowid,oid,_rowid_,id FROM shadow", [][]any{{"shadow", int64(17), "internal", int64(9)}}},
		{"SELECT rowid,oid,_rowid_ FROM shadow_one", [][]any{{"named", int64(1), int64(1)}}},
		{"SELECT id,n,t,r,typeof(n),typeof(t),typeof(r) FROM defaults", [][]any{{int64(3), int64(7), "7", float64(7), "integer", "text", "real"}}},
		{"SELECT rowid,id FROM noninteger", [][]any{{int64(1), int64(7)}}},
		{"SELECT \"İ\", i, I FROM unicode_names", [][]any{{"capital dotted", "ascii", "ascii"}}},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := query(t, db, tc.text); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
		})
	}
	for _, text := range []string{"SELECT * FROM descpk", "SELECT missing FROM empty", "SELECT unknown(x) FROM empty", "SELECT length(x,x) FROM empty"} {
		if rows, err := db.Query(context.Background(), text); err == nil {
			rows.Close()
			t.Fatalf("accepted %s", text)
		}
	}
	rows, err := db.Query(context.Background(), "SELECT 1+2, 'x', length('abc'), n.x AS a FROM empty AS n")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if got, want := rows.Columns(), []string{"1+2", "'x'", "length('abc')", "a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("labels %q want %q", got, want)
	}
}

func TestCanonicalColumnMetadata(t *testing.T) {
	db := openFixture(t, sqlite.Options{})
	rows, err := db.Query(context.Background(), "SELECT ID,rowid,_rowid_,oid FROM samples LIMIT 0")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if want := []string{"id", "id", "id", "id"}; !reflect.DeepEqual(rows.Columns(), want) {
		t.Fatal(rows.Columns())
	}
}
