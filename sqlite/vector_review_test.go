package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"path/filepath"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Review fixtures use ordinary native statements only. Invalid typed candidate
// values are exercised below through an in-memory source, never damaged files.
func reviewVectorDB(t *testing.T, options Options) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "review.db")
	db, err := Open(path, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, path
}

func reviewVectorExec(t *testing.T, db *DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(query, err)
	}
}

func reviewVectorQuery(t *testing.T, db *DB, query string, args ...any) ([][]any, QueryPlan, QueryStats) {
	t.Helper()
	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		t.Fatal(query, err)
	}
	defer rows.Close()
	var result [][]any
	for rows.Next() {
		result = append(result, rows.Values())
	}
	if err := rows.Err(); err != nil {
		t.Fatal(query, err)
	}
	return result, rows.Plan(), rows.Stats()
}

// This arithmetic deliberately calls no production metric. Every component is
// a quarter-integer, so products and sums are exact in float64. Only sqrt and
// the final cosine division need a rounding tolerance. Ranking uses full sort.
func reviewVectorDistance(a, b []float32, metric vector.MetricKind) float64 {
	var l2, l1, dot, aa, bb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		l2 += (x - y) * (x - y)
		l1 += math.Abs(x - y)
		dot += x * y
		aa += x * x
		bb += y * y
	}
	switch metric {
	case vector.L1:
		return l1
	case vector.NegativeIP:
		return -dot
	case vector.Cosine:
		return 1 - math.Max(-1, math.Min(1, dot/math.Sqrt(aa*bb)))
	default:
		return math.Sqrt(l2)
	}
}

func TestVectorReviewHeapAndNativeIndependentReference(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE vectors(id INTEGER PRIMARY KEY,embedding VECTOR(4),active INTEGER)")
	type candidate struct {
		id     int64
		v      []float32
		active int
	}
	candidates := []candidate{{math.MinInt64, []float32{1, 2, 3, 4}, 1}, {math.MaxInt64, []float32{1, 2, 3, 4}, 1}, {-1000, nil, 1}, {1000, nil, 0}}
	rng := rand.New(rand.NewSource(6101027))
	for i := 0; i < 72; i++ {
		v := []float32{float32(rng.Intn(63)+1) / 4, float32(rng.Intn(63)-31) / 4, float32(rng.Intn(63)-31) / 4, float32(rng.Intn(63)-31) / 4}
		if i%9 == 0 {
			v = []float32{1, 2, 3, 4}
		}
		candidates = append(candidates, candidate{int64(i) - 36, v, i % 3})
	}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range candidates {
		var bound any
		if c.v != nil {
			v, err := vector.New(c.v)
			if err != nil {
				t.Fatal(err)
			}
			bound = v
		}
		if _, err := tx.Exec(context.Background(), "INSERT INTO vectors VALUES(?,?,?)", c.id, bound, c.active); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	type referenceHit struct {
		id       int64
		distance float64
		null     bool
	}
	for qi, q := range [][]float32{{1, 2, 3, 4}, {-2, .25, 3, -1}, {15, 8, -8, .5}} {
		bound, err := vector.New(q)
		if err != nil {
			t.Fatal(err)
		}
		for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
			op := map[vector.MetricKind]string{vector.L2: "<->", vector.L1: "<+>", vector.Cosine: "<=>", vector.NegativeIP: "<#>"}[metric]
			for _, filter := range []int{-1, 0, 1} {
				where := "1"
				if filter >= 0 {
					where = fmt.Sprintf("active=%d", filter)
				}
				var want []referenceHit
				for _, c := range candidates {
					if filter >= 0 && c.active != filter {
						continue
					}
					h := referenceHit{id: c.id, null: c.v == nil}
					if !h.null {
						h.distance = reviewVectorDistance(c.v, q, metric)
					}
					want = append(want, h)
				}
				sort.Slice(want, func(i, j int) bool {
					if want[i].null != want[j].null {
						return want[i].null
					}
					if want[i].distance != want[j].distance {
						return want[i].distance < want[j].distance
					}
					return want[i].id < want[j].id
				})
				for _, window := range [][2]int{{1, 0}, {7, 0}, {3, 2}, {13, 5}, {200, 0}, {3, 200}} {
					limit, offset := window[0], window[1]
					expr := "v.embedding " + op + " ?"
					if qi == 1 {
						expr = "? " + op + " v.embedding"
					}
					if qi == 2 {
						fn := map[vector.MetricKind]string{vector.L2: "l2_distance", vector.L1: "l1_distance", vector.Cosine: "cosine_distance", vector.NegativeIP: "negative_inner_product"}[metric]
						expr = fn + "(v.embedding,?)"
					}
					prefix := "SELECT v.id," + expr + " AS distance FROM vectors AS v WHERE " + where + " ORDER BY distance"
					suffix := fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
					got, plan, stats := reviewVectorQuery(t, db, prefix+suffix, bound)
					fallback, fp, _ := reviewVectorQuery(t, db, prefix+",v.id"+suffix, bound)
					if !reflect.DeepEqual(got, fallback) || plan.VectorRanking != "bounded_heap" || fp.VectorRanking != "" || !plan.VectorExact || plan.VectorTopK != int64(limit+offset) {
						t.Fatalf("metric=%s query=%d filter=%d window=%v: heap=%v fallback=%v plans=%+v/%+v", metric, qi, filter, window, got, fallback, plan, fp)
					}
					begin := min(offset, len(want))
					selected := want[begin:min(begin+limit, len(want))]
					if len(got) != len(selected) || stats.BaseRowFetches != int64(len(candidates)) || stats.ReturnedRows != int64(len(got)) {
						t.Fatalf("wrong cardinality/work: got=%v want=%v stats=%+v", got, selected, stats)
					}
					for i, h := range selected {
						if got[i][0] != h.id {
							t.Fatalf("rank %d: got=%v want=%v", i, got[i], h)
						}
						if h.null {
							if got[i][1] != nil {
								t.Fatal("NULL became scored", got[i])
							}
						} else if d, ok := got[i][1].(float64); !ok || math.Abs(d-h.distance) > 1e-13*math.Max(1, math.Abs(h.distance)) {
							t.Fatalf("distance got=%v want=%v", got[i], h)
						}
					}
					// Native Top-K deliberately excludes SQL NULL candidates. Its
					// independent expected list therefore removes them before truncation.
					var nativeWant []referenceHit
					for _, h := range want {
						if !h.null {
							nativeWant = append(nativeWant, h)
						}
					}
					nativeWant = nativeWant[:min(limit, len(nativeWant))]
					native, err := db.VectorSearch(context.Background(), "vectors", "embedding", bound, VectorSearchOptions{Metric: metric, Limit: limit, Where: where})
					if err != nil || len(native.Hits) != len(nativeWant) {
						t.Fatalf("native %+v %v want=%v", native, err, nativeWant)
					}
					for i, h := range nativeWant {
						if native.Hits[i].RowID != h.id || math.Abs(native.Hits[i].Distance-h.distance) > 1e-13*math.Max(1, math.Abs(h.distance)) {
							t.Fatalf("native rank %d: got=%+v want=%v", i, native.Hits[i], h)
						}
					}
				}
			}
		}
	}
}

type reviewVectorSource struct {
	rows   [][]any
	next   int
	closed bool
}

func (s *reviewVectorSource) Next() (int64, []any, error) {
	if s.next == len(s.rows) {
		return 0, nil, io.EOF
	}
	row := append([]any(nil), s.rows[s.next]...)
	s.next++
	return int64(s.next), row, nil
}
func (s *reviewVectorSource) Close() error { s.closed = true; return nil }

func reviewMemoryVectorRows(t *testing.T, candidate any, query any, heap bool) *Rows {
	t.Helper()
	tab := &table{name: "items", root: 2, ipk: 0, index: map[string]int{"id": 0, "embedding": 1, "active": 2}, columns: []sqlast.ColumnDef{{Name: "id", Type: "INTEGER", PrimaryKey: true}, {Name: "embedding", Type: "VECTOR(2)"}, {Name: "active", Type: "INTEGER"}}}
	expression := &sqlast.Binary{Op: "<->", Left: &sqlast.Column{Name: "embedding"}, Right: &sqlast.Literal{Value: query}}
	r := &Rows{db: &DB{opts: Options{MaxSortBytes: 1 << 20}}, ctx: context.Background(), table: tab, sel: &sqlast.Select{}, exprs: []sqlast.Expr{&sqlast.Column{Name: "id"}}, aliases: map[string]int{}, order: []orderExpr{{expr: expression, column: -1}}, source: &reviewVectorSource{rows: [][]any{{nil, candidate, int64(1)}}}, limit: 1}
	if heap {
		if err := r.prepareVectorRank(); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestVectorReviewTypedNullOperandParity(t *testing.T) {
	wrongDim, _ := vector.Parse("[1]")
	wrongBlob, _ := wrongDim.MarshalBinary()
	for _, bad := range []any{"[1]", wrongBlob, "invalid", int64(1), []byte("[1,2]")} {
		for _, heap := range []bool{false, true} {
			r := reviewMemoryVectorRows(t, bad, nil, heap)
			if err := r.materialize(); !errors.Is(err, ErrMismatch) {
				t.Fatalf("heap=%t candidate=%T must validate referenced typed column despite NULL query: %v", heap, bad, err)
			}
			r.source.Close()
		}
	}
	// SQL NULL candidate still suppresses malformed query decoding on either
	// path, and ordinary valid text and BLOB values both retain NULL distance.
	valid, _ := vector.Parse("[1,2]")
	blob, _ := valid.MarshalBinary()
	for _, pair := range [][2]any{{nil, "invalid"}, {"[1,2]", nil}, {blob, nil}, {nil, nil}} {
		for _, heap := range []bool{false, true} {
			r := reviewMemoryVectorRows(t, pair[0], pair[1], heap)
			if err := r.materialize(); err != nil || len(r.sorted) != 1 || r.sorted[0].keys[0] != nil {
				t.Fatalf("heap=%t NULL pair=%T/%T rows=%v error=%v", heap, pair[0], pair[1], r.sorted, err)
			}
		}
	}
}

func TestVectorReviewSQLHeapBudgetsCountNullAndFilteredRows(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		r := reviewMemoryVectorRows(t, nil, nil, true)
		r.vectorRank.limits.MaxCandidates = 3
		r.source = &reviewVectorSource{rows: [][]any{{nil, nil, int64(0)}, {nil, nil, int64(0)}, {nil, nil, int64(0)}, {nil, nil, int64(0)}}}
		if filtered {
			r.sel.Where = &sqlast.Column{Name: "active"}
		}
		if err := r.materialize(); !errors.Is(err, vector.ErrLimit) || len(r.sorted) != 0 {
			t.Fatalf("filtered=%t lost full-scan quota: %v %+v", filtered, err, r.queryStats)
		}
		if r.queryStats.BaseRowFetches != 4 {
			t.Fatal(r.queryStats)
		}
		r.source.Close()
	}
	for _, components := range []uint64{1, 2, 3, 5} {
		r := reviewMemoryVectorRows(t, "[1,2]", "[1,2]", true)
		r.vectorRank.limits.MaxComponents = components
		r.source = &reviewVectorSource{rows: [][]any{{nil, "[1,2]", int64(1)}, {nil, "[1,2]", int64(1)}}}
		if err := r.materialize(); !errors.Is(err, vector.ErrLimit) || len(r.sorted) != 0 {
			t.Fatalf("components=%d quota: %v %+v", components, err, r.queryStats)
		}
		r.source.Close()
	}
}

type reviewVectorCancelContext struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int64
}

func (c *reviewVectorCancelContext) Err() error {
	if c.checks.Add(1) == 70 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestVectorReviewHeapCancellationReleasesSnapshot(t *testing.T) {
	db, path := reviewVectorDB(t, Options{WALShared: true})
	reviewVectorExec(t, db, "CREATE TABLE vectors(id INTEGER PRIMARY KEY,embedding VECTOR(2))")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 80; i++ {
		if _, err := tx.Exec(context.Background(), "INSERT INTO vectors VALUES(?,?)", i, fmt.Sprintf("[%d,1]", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &reviewVectorCancelContext{Context: base, cancel: cancel}
	rows, err := db.Query(ctx, "SELECT id FROM vectors ORDER BY embedding <-> '[1,1]' LIMIT 5")
	if !errors.Is(err, context.Canceled) || rows != nil {
		if rows != nil {
			rows.Close()
		}
		t.Fatalf("cancellation returned partial success: %v %v", rows, err)
	}
	other, err := Open(path, Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	reviewVectorExec(t, other, "UPDATE vectors SET embedding='[1,1]' WHERE id=79")
	if err := other.Checkpoint(context.Background()); err != nil {
		t.Fatalf("canceled heap leaked reader: %v", err)
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT id FROM vectors ORDER BY embedding <-> '[1,1]' LIMIT 2")
	if !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(79)}}) {
		t.Fatal(got)
	}
}

func TestVectorReviewHeapSnapshotRowsRemainStable(t *testing.T) {
	db, path := reviewVectorDB(t, Options{WALShared: true})
	reviewVectorExec(t, db, "CREATE TABLE vectors(id INTEGER PRIMARY KEY,embedding VECTOR(2))")
	reviewVectorExec(t, db, "INSERT INTO vectors VALUES(1,'[1,1]'),(2,'[2,1]'),(3,'[3,1]')")
	rows, err := db.Query(context.Background(), "SELECT id,embedding <-> '[1,1]' AS distance FROM vectors ORDER BY distance LIMIT 3")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	other, err := Open(path, Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	reviewVectorExec(t, other, "UPDATE vectors SET embedding='[1,1]' WHERE id=3")
	var got [][]any
	for rows.Next() {
		got = append(got, rows.Values())
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, [][]any{{int64(1), float64(0)}, {int64(2), float64(1)}, {int64(3), float64(2)}}) {
		t.Fatalf("materialized snapshot changed: %v", got)
	}
	if err := other.Checkpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	fresh, _, _ := reviewVectorQuery(t, db, "SELECT id,embedding <-> '[1,1]' AS distance FROM vectors ORDER BY distance LIMIT 3")
	if !reflect.DeepEqual(fresh, [][]any{{int64(1), float64(0)}, {int64(3), float64(0)}, {int64(2), float64(1)}}) {
		t.Fatalf("fresh snapshot stale: %v", fresh)
	}
}

func TestVectorReviewCloseReleasesRankStorage(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE vectors(id INTEGER PRIMARY KEY,embedding VECTOR(2))")
	reviewVectorExec(t, db, "INSERT INTO vectors VALUES(1,'[1,2]')")
	bound, _ := vector.Parse("[1,2]")
	rows, err := db.Query(context.Background(), "SELECT id FROM vectors ORDER BY embedding <-> ? LIMIT 1", bound)
	if err != nil {
		t.Fatal(err)
	}
	if rows.vectorRank == nil || !rows.vectorRank.parsed.Valid() {
		t.Fatal("review setup did not populate parsed rank storage")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if rows.vectorRank != nil || rows.params != nil || rows.sorted != nil {
		t.Fatal("closed Rows retained bound/decoded rank storage")
	}
	if rows.Plan().VectorRanking != "bounded_heap" || rows.Stats().VectorRankedRows != 1 {
		t.Fatal("close lost durable diagnostics")
	}
}
