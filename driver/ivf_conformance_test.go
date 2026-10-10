package driver_test

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const driverIVFQuery = "SELECT id,l2_distance(v,?) AS distance FROM items ORDER BY distance LIMIT ? OFFSET ?"

type driverIVFQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type driverIVFHit struct {
	id       int64
	distance float64
}

func driverIVFVector(t *testing.T, values ...float32) vector.Vector {
	t.Helper()
	v, err := vector.New(values)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func driverIVFOpen(t *testing.T, path string, shared, existing bool) *sql.DB {
	t.Helper()
	c, err := gosqldriver.NewConnector(path, sqlite.Options{WALShared: shared, NoCreate: existing})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(c)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func driverIVFFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(3) NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO items VALUES(?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for cluster := 0; cluster < 4; cluster++ {
		for offset := 0; offset < 16; offset++ {
			v := driverIVFVector(t, float32(cluster*100+offset), float32(cluster*50)+float32(offset)/2, 1)
			if _, err = stmt.ExecContext(ctx, cluster*16+offset+1, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "CREATE INDEX items_ivf ON items(v) USING ivfflat WITH(opclass='vector_l2_ops',lists=4,training_samples=64,training_iterations=8)"); err != nil {
		t.Fatal(err)
	}
}

func driverIVFRead(t *testing.T, rows *sql.Rows, err error) []driverIVFHit {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var hits []driverIVFHit
	for rows.Next() {
		var hit driverIVFHit
		if err = rows.Scan(&hit.id, &hit.distance); err != nil {
			t.Fatal(err)
		}
		hits = append(hits, hit)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return hits
}

func driverIVFHits(t *testing.T, db driverIVFQueryer, query string, args ...any) []driverIVFHit {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), query, args...)
	return driverIVFRead(t, rows, err)
}

// The oracle uses persisted source components and elementary L2 arithmetic,
// independently of the SQL distance implementation and the ANN candidate set.
func driverIVFExact(t *testing.T, db driverIVFQueryer, q vector.Vector) []driverIVFHit {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "SELECT id,v FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	var want []driverIVFHit
	queryValues := q.Values()
	for rows.Next() {
		var id int64
		var v vector.NullVector
		if err = rows.Scan(&id, &v); err != nil || !v.Valid {
			rows.Close()
			t.Fatalf("source vector: valid=%t err=%v", v.Valid, err)
		}
		var squared float64
		for i, value := range v.Vector.Values() {
			delta := float64(value) - float64(queryValues[i])
			squared += delta * delta
		}
		want = append(want, driverIVFHit{id, math.Sqrt(squared)})
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	sort.Slice(want, func(i, j int) bool {
		return want[i].distance < want[j].distance || want[i].distance == want[j].distance && want[i].id < want[j].id
	})
	exactSQL := strings.Replace(driverIVFQuery, "FROM items", "FROM items NOT INDEXED", 1)
	exact := driverIVFHits(t, db, exactSQL, q, len(want), 0)
	if !reflect.DeepEqual(exact, want) {
		t.Fatalf("NOT INDEXED differs from component oracle: got=%v want=%v", exact, want)
	}
	return exact
}

func driverIVFAfterDML(t *testing.T, got, before []driverIVFHit, updatedID, deletedID int64, updatedDistance float64) {
	t.Helper()
	want := []driverIVFHit{{-99, 0}}
	for _, hit := range before {
		if hit.id == deletedID {
			continue
		}
		if hit.id == updatedID {
			hit.distance = updatedDistance
		}
		want = append(want, hit)
	}
	sort.Slice(want, func(i, j int) bool {
		return want[i].distance < want[j].distance || want[i].distance == want[j].distance && want[i].id < want[j].id
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("insert/update/delete source results: got=%v want=%v", got, want)
	}
}

// Approximate search need not return the globally best K. Every returned row
// must nevertheless be real, unique, correctly rescored, and correctly ordered.
func driverIVFTruthful(t *testing.T, got, exact []driverIVFHit, count int) {
	t.Helper()
	if len(got) != count {
		t.Fatalf("result count=%d want=%d: %v", len(got), count, got)
	}
	scores := make(map[int64]float64, len(exact))
	for _, h := range exact {
		scores[h.id] = h.distance
	}
	seen := make(map[int64]bool, len(got))
	for i, h := range got {
		score, exists := scores[h.id]
		if !exists || seen[h.id] || h.distance != score {
			t.Fatalf("untruthful or duplicate ANN hit %+v; exact score=%g exists=%t", h, score, exists)
		}
		seen[h.id] = true
		if i > 0 && (got[i-1].distance > h.distance || got[i-1].distance == h.distance && got[i-1].id > h.id) {
			t.Fatalf("ANN hits out of distance/rowid order: %v", got)
		}
	}
}

func driverIVFPlan(t *testing.T, db driverIVFQueryer, q vector.Vector, exact bool) string {
	t.Helper()
	query := driverIVFQuery
	if exact {
		query = strings.Replace(query, "FROM items", "FROM items NOT INDEXED", 1)
	}
	var node, parent, unused int64
	var detail string
	err := db.QueryRowContext(context.Background(), "EXPLAIN QUERY PLAN "+query, q, 5, 2).Scan(&node, &parent, &unused, &detail)
	if err != nil {
		t.Fatal(err)
	}
	if exact {
		if strings.Contains(detail, "USING items_ivf") || !strings.Contains(detail, "exact=true") {
			t.Fatal(detail)
		}
	} else {
		for _, fragment := range []string{"USING items_ivf ivfflat v1", "approximate_recall=true", "exact=false", "lists=4", "probes=1", "training_state=ready"} {
			if !strings.Contains(detail, fragment) {
				t.Fatalf("missing %q in plan %q", fragment, detail)
			}
		}
	}
	return detail
}

func TestDriverIVFFlatPreparedBindingsRollbackAndReopen(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared%t", shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ivf.db")
			db := driverIVFOpen(t, path, shared, false)
			driverIVFFixture(t, db)
			stmt, err := db.PrepareContext(ctx, driverIVFQuery)
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()
			q := driverIVFVector(t, 0, 0, 1)
			before := driverIVFExact(t, db, q)
			driverIVFPlan(t, db, q, false)
			driverIVFPlan(t, db, q, true)
			readPrepared := func(q vector.Vector, limit, offset int) []driverIVFHit {
				t.Helper()
				rows, err := stmt.QueryContext(ctx, q, limit, offset)
				return driverIVFRead(t, rows, err)
			}
			original := readPrepared(q, 7, 0)
			driverIVFTruthful(t, original, before, 7)
			if got := readPrepared(q, 5, 2); !reflect.DeepEqual(got, original[2:]) {
				t.Fatalf("prepared LIMIT/OFFSET: got=%v want=%v", got, original[2:])
			}
			other := driverIVFVector(t, 300, 150, 1)
			driverIVFTruthful(t, readPrepared(other, 3, 0), driverIVFExact(t, db, other), 3)
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			update, err := tx.PrepareContext(ctx, "UPDATE items SET v=? WHERE id=?")
			if err != nil {
				t.Fatal(err)
			}
			defer update.Close()
			if _, err = update.ExecContext(ctx, other, original[0].id); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(ctx, "DELETE FROM items WHERE id=?", original[1].id); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO items VALUES(?,?)", -99, q); err != nil {
				t.Fatal(err)
			}
			changed := driverIVFExact(t, tx, q)
			driverIVFAfterDML(t, changed, before, original[0].id, original[1].id, math.Sqrt(300*300+150*150))
			driverIVFTruthful(t, driverIVFHits(t, tx, driverIVFQuery, q, 7, 0), changed, 7)
			filtered := driverIVFHits(t, tx, "SELECT id,v <-> ? AS distance FROM items WHERE id=-99 ORDER BY distance LIMIT 1", q)
			if !reflect.DeepEqual(filtered, []driverIVFHit{{-99, 0}}) {
				t.Fatalf("transaction insert missing: %v", filtered)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if got := driverIVFExact(t, db, q); !reflect.DeepEqual(got, before) {
				t.Fatalf("DML rollback changed source scores: %v", got)
			}
			if got := readPrepared(q, 7, 0); !reflect.DeepEqual(got, original) {
				t.Fatalf("DML rollback changed ANN results: %v", got)
			}
			if _, err = db.ExecContext(ctx, "REINDEX items_ivf"); err != nil {
				t.Fatal(err)
			}
			driverIVFPlan(t, db, q, false)
			driverIVFTruthful(t, readPrepared(q, 7, 0), before, 7)
			if err = stmt.Close(); err != nil {
				t.Fatal(err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := driverIVFOpen(t, path, shared, true)
			driverIVFPlan(t, reopened, q, false)
			if got := driverIVFExact(t, reopened, q); !reflect.DeepEqual(got, before) {
				t.Fatalf("ordinary reopen changed source scores: %v", got)
			}
			driverIVFTruthful(t, driverIVFHits(t, reopened, driverIVFQuery, q, 7, 0), before, 7)
		})
	}
}

func TestDriverIVFFlatSharedWALSnapshotAcrossReindex(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ivf-snapshot.db")
	writer := driverIVFOpen(t, path, true, false)
	driverIVFFixture(t, writer)
	reader := driverIVFOpen(t, path, true, true)
	old, err := reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer old.Rollback()
	q := driverIVFVector(t, 0, 0, 1)
	before := driverIVFExact(t, old, q)
	oldHits := driverIVFHits(t, old, driverIVFQuery, q, 7, 0)
	oldPlan := driverIVFPlan(t, old, q, false)
	driverIVFTruthful(t, oldHits, before, 7)
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE items SET v=? WHERE id=?", driverIVFVector(t, 900, 450, 1), oldHits[0].id); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM items WHERE id=?", oldHits[1].id); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO items VALUES(?,?)", -99, q); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, "REINDEX items_ivf"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := driverIVFExact(t, old, q); !reflect.DeepEqual(got, before) {
		t.Fatalf("pinned reader source changed: %v", got)
	}
	if got := driverIVFHits(t, old, driverIVFQuery, q, 7, 0); !reflect.DeepEqual(got, oldHits) {
		t.Fatalf("pinned reader ANN changed: %v", got)
	}
	if got := driverIVFPlan(t, old, q, false); got != oldPlan {
		t.Fatalf("pinned reader plan changed: %s => %s", oldPlan, got)
	}
	current := driverIVFExact(t, writer, q)
	driverIVFAfterDML(t, current, before, oldHits[0].id, oldHits[1].id, math.Sqrt(900*900+450*450))
	driverIVFPlan(t, writer, q, false)
	driverIVFTruthful(t, driverIVFHits(t, writer, driverIVFQuery, q, 7, 0), current, 7)
	if err = old.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := driverIVFExact(t, reader, q); !reflect.DeepEqual(got, current) {
		t.Fatalf("new reader snapshot missed commit: %v", got)
	}
	driverIVFTruthful(t, driverIVFHits(t, reader, driverIVFQuery, q, 7, 0), current, 7)
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := driverIVFOpen(t, path, true, true)
	if got := driverIVFExact(t, reopened, q); !reflect.DeepEqual(got, current) {
		t.Fatalf("ordinary reopen missed committed DML: %v", got)
	}
	driverIVFPlan(t, reopened, q, false)
	driverIVFTruthful(t, driverIVFHits(t, reopened, driverIVFQuery, q, 7, 0), current, 7)
}
