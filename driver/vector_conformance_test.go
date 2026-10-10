package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverDenseVectorBindingsAndTransactions(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		name := "rollback"
		if shared {
			name = "shared"
		}
		t.Run(name, func(t *testing.T) {
			connector, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "vectors.db"), sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			db := sql.OpenDB(connector)
			defer db.Close()
			db.SetMaxOpenConns(1)
			if _, err = db.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),label TEXT)"); err != nil {
				t.Fatal(err)
			}
			values := []float32{math.Float32frombits(1 << 31), math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32}
			v, err := vector.New(values)
			if err != nil {
				t.Fatal(err)
			}
			stmt, err := db.Prepare("INSERT INTO items VALUES(?,?,?)")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = stmt.Exec(1, v, "bits"); err != nil {
				t.Fatal(err)
			}
			if _, err = stmt.Exec(2, vector.NullVector{}, "NULL"); err != nil {
				t.Fatal(err)
			}
			if _, err = stmt.Exec(3, "[1,2,3]", "text"); err != nil {
				t.Fatal(err)
			}
			if err = stmt.Close(); err != nil {
				t.Fatal(err)
			}
			var got vector.NullVector
			if err = db.QueryRow("SELECT embedding FROM items WHERE id=1").Scan(&got); err != nil || !got.Valid {
				t.Fatalf("scan %v %+v", err, got)
			}
			for i, x := range got.Vector.Values() {
				if math.Float32bits(x) != math.Float32bits(values[i]) {
					t.Fatalf("component%d bits %08x want%08x", i, math.Float32bits(x), math.Float32bits(values[i]))
				}
			}
			if err = db.QueryRow("SELECT embedding FROM items WHERE id=2").Scan(&got); err != nil || got.Valid || got.Vector.Valid() {
				t.Fatalf("NULL scan %v %+v", err, got)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec("INSERT INTO items VALUES(4,'[4,5,6]','prior')"); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec("INSERT INTO items VALUES(5,'[1,2,3]','partial'),(6,'[1,2]','invalid')"); err == nil {
				t.Fatal("accepted dimensions")
			}
			ids := func(q *sql.Rows) []int64 {
				t.Helper()
				defer q.Close()
				var result []int64
				for q.Next() {
					var id int64
					if err := q.Scan(&id); err != nil {
						t.Fatal(err)
					}
					result = append(result, id)
				}
				if err := q.Err(); err != nil {
					t.Fatal(err)
				}
				return result
			}
			rows, err := tx.Query("SELECT id FROM items ORDER BY id")
			if err != nil {
				t.Fatal(err)
			}
			if actual := ids(rows); !reflect.DeepEqual(actual, []int64{1, 2, 3, 4}) {
				t.Fatalf("statement atomicity %v", actual)
			}
			if _, err = tx.Exec("UPDATE items SET embedding='[9,8,7]' WHERE id=3"); err != nil {
				t.Fatal(err)
			}
			var text string
			if err = tx.QueryRow("SELECT vector_to_text(embedding) FROM items WHERE id=3").Scan(&text); err != nil || text != "[9,8,7]" {
				t.Fatalf("own write %q %v", text, err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if err = db.QueryRow("SELECT vector_to_text(embedding) FROM items WHERE id=3").Scan(&text); err != nil || text != "[1,2,3]" {
				t.Fatalf("rollback %q %v", text, err)
			}
			for _, bad := range []any{"[1,2]", "[1,NaN,3]", "[1,Inf,3]", []byte("[1,2,3]"), vector.Vector{}} {
				if _, err = db.Exec("UPDATE items SET embedding=? WHERE id=3", bad); err == nil {
					t.Fatalf("accepted invalid value %T", bad)
				}
			}
			var l2, l1, ip, negative, cosine float64
			if err = db.QueryRow("SELECT '[1,2,3]' <-> '[2,3,4]', '[1,2,3]' <+> '[2,3,4]', inner_product('[1,2,3]','[2,3,4]'), '[1,2,3]' <#> '[2,3,4]', '[1,2,3]' <=> '[1,2,3]'").Scan(&l2, &l1, &ip, &negative, &cosine); err != nil {
				t.Fatal(err)
			}
			if math.Abs(l2-math.Sqrt(3)) > 1e-14 || l1 != 3 || ip != 20 || negative != -20 || math.Abs(cosine) > 1e-14 {
				t.Fatalf("metrics %g %g %g %g %g", l2, l1, ip, negative, cosine)
			}
			if err = db.QueryRow("SELECT '[0,0,0]' <=> '[1,2,3]'").Scan(&cosine); !errors.Is(err, vector.ErrZeroNorm) {
				t.Fatalf("cosine zero %v", err)
			}
			rows, err = db.Query("SELECT id FROM items ORDER BY embedding <-> '[1,2,3]' LIMIT 2")
			if err != nil {
				t.Fatal(err)
			}
			if actual := ids(rows); !reflect.DeepEqual(actual, []int64{2, 3}) {
				t.Fatalf("SQL NULL-first KNN %v", actual)
			}
			rows, err = db.Query("SELECT id FROM items WHERE embedding IS NOT NULL ORDER BY embedding <-> ? LIMIT ? OFFSET ?", "[1,2,3]", 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if actual := ids(rows); !reflect.DeepEqual(actual, []int64{1}) {
				t.Fatalf("KNN offset %v", actual)
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if _, err = db.QueryContext(canceled, "SELECT l2_distance('[1,2,3]','[2,3,4]')"); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation %v", err)
			}
		})
	}
}

func TestDriverVectorQuotaErrorClassification(t *testing.T) {
	db, err := sql.Open("gosqlite", filepath.Join(t.TempDir(), "quota.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3))"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO items VALUES(1,'[1,2,3]')"); err != nil {
		t.Fatal(err)
	}
	oversized := strings.Repeat(" ", vector.MaxTextBytes+1)
	for _, q := range []string{"INSERT INTO items VALUES(2,?)", "UPDATE items SET embedding=? WHERE id=1"} {
		_, err = db.Exec(q, oversized)
		var public *sqlite.Error
		if !errors.Is(err, vector.ErrLimit) || !errors.Is(err, sqlite.ErrMemoryLimit) || !errors.As(err, &public) || public.Code != sqlite.CodeNoMem {
			t.Fatalf("vector quota classification for %q: %v %+v", q, err, public)
		}
	}
	var text string
	if err = db.QueryRow("SELECT vector_to_text(embedding) FROM items WHERE id=1").Scan(&text); err != nil || text != "[1,2,3]" {
		t.Fatalf("quota modified prior value: %q %v", text, err)
	}
}
