package sqlite

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
)

type typedANNFixture struct {
	kind, declaration string
	values            []any
	numeric           [][]float32
	bits              []string
	query             any
	queryNumeric      []float32
	queryBits         string
}

func newTypedANNFixture(t *testing.T, kind string, count int) typedANNFixture {
	t.Helper()
	f := typedANNFixture{kind: kind, queryNumeric: []float32{4.25, -2.5, 3.25}}
	switch kind {
	case "halfvec":
		f.declaration = "HALFVEC(3)"
		f.query = "[4.25,-2.5,3.25]"
	case "sparsevec":
		f.declaration = "SPARSEVEC(1000000000)"
		f.query = "{1:4.25,1000:-2.5,1000000000:3.25}/1000000000"
	case "bit":
		f.declaration = "BIT(33)"
		f.queryBits = "110101001110101010010001101010101"
		f.query = f.queryBits
	}
	for i := 0; i < count; i++ {
		n := []float32{float32((i*19)%53) / 4, float32((i*37)%67-33) / 4, float32(1+(i*7)%19) / 4}
		f.numeric = append(f.numeric, n)
		var val any
		switch kind {
		case "halfvec":
			val = fmt.Sprintf("[%g,%g,%g]", n[0], n[1], n[2])
		case "sparsevec":
			val = fmt.Sprintf("{1:%g,1000:%g,1000000000:%g}/1000000000", n[0], n[1], n[2])
		case "bit":
			var b strings.Builder
			state := uint64(i+1) * 0x9e3779b97f4a7c15
			for j := 0; j < 33; j++ {
				state ^= state >> 12
				state ^= state << 25
				state ^= state >> 27
				if (state*0x2545f4914f6cdd1d)>>63 != 0 {
					b.WriteByte('1')
				} else {
					b.WriteByte('0')
				}
			}
			f.bits = append(f.bits, b.String())
			val = b.String()
		}
		f.values = append(f.values, val)
	}
	return f
}
func (f typedANNFixture) exact(metric string, index int) float64 {
	if f.kind != "bit" {
		return reviewVectorDistance(f.queryNumeric, f.numeric[index], vector.MetricKind(metric))
	}
	var diff, inter, union int
	for j := range f.queryBits {
		a, b := f.queryBits[j], f.bits[index][j]
		if a != b {
			diff++
		}
		if a == '1' && b == '1' {
			inter++
		}
		if a == '1' || b == '1' {
			union++
		}
	}
	if metric == string(vector.Hamming) {
		return float64(diff)
	}
	if inter == 0 {
		return 1
	}
	return 1 - float64(inter)/float64(union)
}
func typedFixtureDB(t *testing.T, f typedANNFixture) (*DB, string) {
	t.Helper()
	db, path := reviewVectorDB(t, Options{MaxTransactionPages: 20000})
	reviewVectorExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v "+f.declaration+" NOT NULL,active INTEGER)")
	reviewVectorExec(t, db, "BEGIN")
	for i, val := range f.values {
		reviewVectorExec(t, db, "INSERT INTO items VALUES(?,?,?)", i+1, val, i%3)
	}
	reviewVectorExec(t, db, "COMMIT")
	return db, path
}
func typedNative(t *testing.T, db *DB, f typedANNFixture, metric string, mode VectorSearchMode, index string, limit int) ([]vector.Hit, QueryPlan, QueryStats, bool) {
	t.Helper()
	ctx := context.Background()
	switch f.kind {
	case "halfvec":
		r, e := db.HalfSearch(ctx, "items", "v", f.query, HalfSearchOptions{Metric: vector.MetricKind(metric), Mode: mode, Index: index, Limit: limit})
		if e != nil {
			t.Fatal(e)
		}
		return r.Hits, r.Plan.QueryPlan, r.Stats.QueryStats, r.Plan.Exact
	case "sparsevec":
		r, e := db.SparseSearch(ctx, "items", "v", f.query, SparseSearchOptions{Metric: vector.MetricKind(metric), Mode: mode, Index: index, Limit: limit})
		if e != nil {
			t.Fatal(e)
		}
		return r.Hits, r.Plan.QueryPlan, r.Stats.QueryStats, r.Plan.Exact
	default:
		r, e := db.BitSearch(ctx, "items", "v", f.query, BitSearchOptions{Metric: vector.BitMetricKind(metric), Mode: mode, Index: index, Limit: limit})
		if e != nil {
			t.Fatal(e)
		}
		return r.Hits, r.Plan.QueryPlan, r.Stats.QueryStats, r.Plan.Exact
	}
}
func TestTypedANNHNSWAllMetricsPersistentSQLAndNative(t *testing.T) {
	for _, kind := range []string{"halfvec", "bit", "sparsevec"} {
		metrics := []string{string(vector.L2), string(vector.L1), string(vector.NegativeIP), string(vector.Cosine)}
		if kind == "bit" {
			metrics = []string{string(vector.Hamming), string(vector.Jaccard)}
		}
		for _, metric := range metrics {
			t.Run(kind+"/"+metric, func(t *testing.T) {
				f := newTypedANNFixture(t, kind, 96)
				db, path := typedFixtureDB(t, f)
				ctx := context.Background()
				cfgKind, _, _ := annColumnType(f.declaration)
				opclass, e := ann.OpclassFor(cfgKind, vector.MetricKind(metric))
				if e != nil {
					t.Fatal(e)
				}
				reviewVectorExec(t, db, "CREATE INDEX typed_knn ON items(v) USING hnsw WITH(opclass='"+opclass+"')")
				if e = db.CheckExtensionIndex(ctx, "typed_knn"); e != nil {
					t.Fatal(e)
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				db, e = Open(path, Options{})
				if e != nil {
					t.Fatal(e)
				}
				defer db.Close()
				exact, _, _, isExact := typedNative(t, db, f, metric, VectorModeExact, "", 10)
				if !isExact {
					t.Fatal("exact default changed")
				}
				want := make([]vector.Hit, len(f.values))
				for i := range want {
					want[i] = vector.Hit{RowID: int64(i + 1), Distance: f.exact(metric, i)}
				}
				sort.Slice(want, func(i, j int) bool { return bitHitBetter(want[i], want[j]) })
				for i, h := range exact {
					if h.RowID != want[i].RowID || math.Abs(h.Distance-want[i].Distance) > 1e-12 {
						t.Fatal("independent exact", h, want[i])
					}
				}
				hits, p, s, isExact := typedNative(t, db, f, metric, VectorModeApproximate, "typed_knn", 10)
				if isExact || p.Method != "hnsw" || p.MethodVersion != 2 || p.ANNOpclass != opclass || s.ANNNodeReads == 0 || s.ANNEdgeReads == 0 || s.ANNDistanceEvaluations == 0 || s.ANNExactRescores == 0 || s.BaseRowFetches >= 96 {
					t.Fatal(p, s, isExact)
				}
				matched := 0
				for _, h := range hits {
					if math.Abs(h.Distance-f.exact(metric, int(h.RowID)-1)) > 1e-12 {
						t.Fatal("inexact source rescore", h)
					}
					for _, w := range want[:10] {
						if h.RowID == w.RowID {
							matched++
						}
					}
				}
				t.Logf("persistent independent recall@10=%g; source=%d graph_scores=%d", float64(matched)/10, s.BaseRowFetches, s.ANNDistanceEvaluations)
				if matched < 9 {
					t.Fatal("fixed small fixture HNSW recall", matched)
				}
				op := map[string]string{string(vector.L2): "<->", string(vector.L1): "<+>", string(vector.NegativeIP): "<#>", string(vector.Cosine): "<=>", string(vector.Hamming): "<~>", string(vector.Jaccard): "<%>"}[metric]
				query := "SELECT id,v " + op + " ? AS d FROM items ORDER BY d LIMIT 7 OFFSET 2"
				rows, plan, stats := reviewVectorQuery(t, db, query, f.query)
				baseline, _, _ := reviewVectorQuery(t, db, strings.Replace(query, "FROM items", "FROM items NOT INDEXED", 1), f.query)
				if !reflect.DeepEqual(rows, baseline) || plan.Method != "hnsw" || plan.VectorExact || stats.ANNExactRescores == 0 {
					t.Fatal(rows, baseline, plan, stats)
				}
			})
		}
	}
}

func TestTypedANNOrdinaryLifecycleSavepointAndReindex(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"halfvec", "bit", "sparsevec"} {
		t.Run(kind, func(t *testing.T) {
			f := newTypedANNFixture(t, kind, 20)
			db, path := typedFixtureDB(t, f)
			if e := db.CreateHNSWIndex(ctx, "typed_knn", "items", "v"); e != nil {
				t.Fatal(e)
			}
			metric := string(vector.L2)
			if kind == "bit" {
				metric = string(vector.Hamming)
			}
			original, _, _, _ := typedNative(t, db, f, metric, VectorModeApproximate, "typed_knn", 20)
			reviewVectorExec(t, db, "SAVEPOINT outer_mark")
			reviewVectorExec(t, db, "UPDATE items SET v=? WHERE id=1", f.query)
			reviewVectorExec(t, db, "DELETE FROM items WHERE id=2")
			reviewVectorExec(t, db, "INSERT INTO items VALUES(-9,?,1)", f.query)
			changed, _, _, _ := typedNative(t, db, f, metric, VectorModeApproximate, "typed_knn", 20)
			if changed[0].RowID != -9 || len(changed) != 20 {
				t.Fatal(changed)
			}
			reviewVectorExec(t, db, "SAVEPOINT inner_mark")
			reviewVectorExec(t, db, "UPDATE items SET id=99 WHERE id=3")
			reviewVectorExec(t, db, "ROLLBACK TO inner_mark")
			reviewVectorExec(t, db, "RELEASE inner_mark")
			if e := db.CheckExtensionIndex(ctx, "typed_knn"); e != nil {
				t.Fatal(e)
			}
			if e := db.Reindex(ctx, "typed_knn"); e != nil {
				t.Fatal(e)
			}
			reviewVectorExec(t, db, "ROLLBACK TO outer_mark")
			reviewVectorExec(t, db, "RELEASE outer_mark")
			got, _, _, _ := typedNative(t, db, f, metric, VectorModeApproximate, "typed_knn", 20)
			if !reflect.DeepEqual(got, original) {
				t.Fatal(got, original)
			}
			if e := db.CheckExtensionIndex(ctx, "typed_knn"); e != nil {
				t.Fatal(e)
			}
			reviewVectorExec(t, db, "UPDATE items SET v=? WHERE id=1", f.query)
			if e := db.Reindex(ctx, "typed_knn"); e != nil {
				t.Fatal(e)
			}
			if e := db.Close(); e != nil {
				t.Fatal(e)
			}
			db, e := Open(path, Options{})
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			if e = db.CheckExtensionIndex(ctx, "typed_knn"); e != nil {
				t.Fatal(e)
			}
			final, plan, _, _ := typedNative(t, db, f, metric, VectorModeApproximate, "typed_knn", 3)
			if final[0].RowID != 1 || plan.ANNGeneration != 2 {
				t.Fatal(final, plan)
			}
		})
	}
}

func TestTypedANNCreationBoundariesAndConstraints(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	ctx := context.Background()
	for _, sql := range []string{"CREATE TABLE halfmax(id INTEGER PRIMARY KEY,v HALFVEC(4000))", "CREATE TABLE halfbad(id INTEGER PRIMARY KEY,v HALFVEC(4001))", "CREATE TABLE bitsmax(id INTEGER PRIMARY KEY,v BIT(64000))", "CREATE TABLE sparsemax(id INTEGER PRIMARY KEY,v SPARSEVEC(1000000000))"} {
		reviewVectorExec(t, db, sql)
	}
	for _, tab := range []string{"halfmax", "bitsmax", "sparsemax"} {
		if e := db.CreateHNSWIndex(ctx, tab+"_idx", tab, "v"); e != nil {
			t.Fatal(e)
		}
	}
	if e := db.CreateHNSWIndex(ctx, "too_wide", "halfbad", "v"); !errors.Is(e, ann.ErrConfig) {
		t.Fatal(e)
	}
	if e := db.CreateIVFFlatIndex(ctx, "sparse_ivf", "sparsemax", "v"); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if e := db.CreateIVFFlatIndex(ctx, "jaccard_ivf", "bitsmax", "v", IVFFlatIndexOptions{Opclass: "bit_jaccard_ops"}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if e := db.CreateHNSWIndex(ctx, "bad_kind", "halfmax", "v", HNSWIndexOptions{Opclass: "vector_l2_ops"}); !errors.Is(e, ann.ErrConfig) {
		t.Fatal(e)
	}
	entries := make([]vector.SparseEntry, 1001)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i), Value: 1}
	}
	tooMany, e := vector.NewSparse(1000000000, entries)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "INSERT INTO sparsemax VALUES(1,?)", tooMany); !errors.Is(e, ErrConstraint) {
		t.Fatal(e)
	}
	if e = db.CheckExtensionIndex(ctx, "sparsemax_idx"); e != nil {
		t.Fatal(e)
	}
	reviewVectorExec(t, db, "CREATE TABLE cosine_values(id INTEGER PRIMARY KEY,v HALFVEC(2))")
	reviewVectorExec(t, db, "INSERT INTO cosine_values VALUES(1,'[1,0]'),(2,NULL)")
	if e = db.CreateHNSWIndex(ctx, "cos_idx", "cosine_values", "v", HNSWIndexOptions{Metric: vector.Cosine}); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "UPDATE cosine_values SET v='[0,0]' WHERE id=1"); !errors.Is(e, vector.ErrZeroNorm) {
		t.Fatal(e)
	}
	if e = db.CheckExtensionIndex(ctx, "cos_idx"); e != nil {
		t.Fatal(e)
	}
}

func TestTypedANNIVFPersistentListsAllMetricsAndDML(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"halfvec", "bit"} {
		metrics := []string{string(vector.L2), string(vector.NegativeIP), string(vector.Cosine)}
		if kind == "bit" {
			metrics = []string{string(vector.Hamming)}
		}
		for _, metric := range metrics {
			t.Run(kind+"/"+metric, func(t *testing.T) {
				f := newTypedANNFixture(t, kind, 128)
				db, path := typedFixtureDB(t, f)
				k, _, _ := annColumnType(f.declaration)
				opclass, e := ann.OpclassFor(k, vector.MetricKind(metric))
				if e != nil {
					t.Fatal(e)
				}
				reviewVectorExec(t, db, "CREATE INDEX typed_ivf ON items(v) USING ivfflat WITH(opclass='"+opclass+"',lists=4)")
				if e = db.CheckExtensionIndex(ctx, "typed_ivf"); e != nil {
					t.Fatal(e)
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				db, e = Open(path, Options{})
				if e != nil {
					t.Fatal(e)
				}
				defer db.Close()
				hits, p, s, exact := typedNative(t, db, f, metric, VectorModeApproximate, "typed_ivf", 10)
				if exact || p.Method != "ivfflat" || p.MethodVersion != 2 || p.ANNOpclass != opclass || s.ANNCentroidScores != 4 || (s.ANNListsOpened < 1 || s.ANNListsOpened >= 4) || s.ANNNodeReads != 0 || s.ANNDistanceEvaluations != 0 || s.ANNMemberKeysRead == 0 || s.BaseRowFetches >= 128 {
					t.Fatal(p, s, exact)
				}
				want := make([]vector.Hit, len(f.values))
				for i := range want {
					want[i] = vector.Hit{RowID: int64(i + 1), Distance: f.exact(metric, i)}
				}
				sort.Slice(want, func(i, j int) bool { return bitHitBetter(want[i], want[j]) })
				matched := 0
				for _, h := range hits {
					if math.Abs(h.Distance-f.exact(metric, int(h.RowID)-1)) > 1e-12 {
						t.Fatal(h)
					}
					for _, w := range want[:10] {
						if h.RowID == w.RowID {
							matched++
						}
					}
				}
				t.Logf("persistent IVF independent recall@10=%g source=%d centroids=%d lists=%d", float64(matched)/10, s.BaseRowFetches, s.ANNCentroidScores, s.ANNListsOpened)
				op := map[string]string{string(vector.L2): "<->", string(vector.NegativeIP): "<#>", string(vector.Cosine): "<=>", string(vector.Hamming): "<~>"}[metric]
				_, plan, stats := reviewVectorQuery(t, db, "SELECT id FROM items ORDER BY v "+op+" ? LIMIT 3", f.query)
				if plan.Method != "ivfflat" || plan.VectorExact || stats.ANNListsOpened != 1 || stats.ANNExactRescores == 0 {
					t.Fatal(plan, stats)
				}
				reviewVectorExec(t, db, "SAVEPOINT test_mark")
				reviewVectorExec(t, db, "UPDATE items SET v=? WHERE id=1", f.query)
				reviewVectorExec(t, db, "UPDATE items SET id=-8 WHERE id=2")
				reviewVectorExec(t, db, "DELETE FROM items WHERE id=3")
				if e = db.CheckExtensionIndex(ctx, "typed_ivf"); e != nil {
					t.Fatal(e)
				}
				reviewVectorExec(t, db, "ROLLBACK TO test_mark")
				reviewVectorExec(t, db, "RELEASE test_mark")
				if e = db.CheckExtensionIndex(ctx, "typed_ivf"); e != nil {
					t.Fatal(e)
				}
				after, _, _, _ := typedNative(t, db, f, metric, VectorModeApproximate, "typed_ivf", 10)
				if !reflect.DeepEqual(after, hits) {
					t.Fatal(after, hits)
				}
				if e = db.Reindex(ctx, "typed_ivf"); e != nil {
					t.Fatal(e)
				}
				if e = db.CheckExtensionIndex(ctx, "typed_ivf"); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestTypedANNQueryOptionsQuotasFallbackAndCancellation(t *testing.T) {
	ctx := context.Background()
	f := newTypedANNFixture(t, "sparsevec", 128)
	db, _ := typedFixtureDB(t, f)
	if e := db.CreateHNSWIndex(ctx, "typed_knn", "items", "v"); e != nil {
		t.Fatal(e)
	}
	success, e := db.SparseSearch(ctx, "items", "v", f.query, SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10})
	if e != nil {
		t.Fatal(e)
	}
	// Work depends on six support positions, never on the billion-dimensional metadata.
	if success.Stats.ANNWork.ComponentWork == 0 || success.Stats.ANNWork.ComponentWork > 6*success.Stats.ANNDistanceEvaluations {
		t.Fatal(success.Stats)
	}
	check := func(o SparseSearchOptions, want error) {
		t.Helper()
		r, e := db.SparseSearch(ctx, "items", "v", f.query, o)
		if !errors.Is(e, want) || !reflect.DeepEqual(r, SparseResult{}) {
			t.Fatal(r, e, want)
		}
	}
	check(SparseSearchOptions{Mode: "invalid", Limit: 0}, vector.ErrOptions)
	check(SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10, Probes: 1}, vector.ErrOptions)
	check(SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10, ANNLimits: ANNWorkLimits{MaxNodeReads: 1}}, ann.ErrLimit)
	limits := DefaultSparseSearchLimits()
	limits.MaxNonzeros = uint64(success.Stats.ScoredNonzeros + success.Stats.ANNWork.ComponentWork - 1)
	check(SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10, Limits: limits}, vector.ErrLimit)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := db.SparseSearch(cancelled, "items", "v", f.query, SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10}); !errors.Is(e, context.Canceled) || !reflect.DeepEqual(r, SparseResult{}) {
		t.Fatal(r, e)
	}
	r, e := db.SparseSearch(ctx, "items", "v", f.query, SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10, Index: "missing"})
	if e != nil || !r.Plan.Exact || r.Plan.Method != "exact_sparsevec" || r.Plan.ANNFallback == "" || r.Stats.ANNNodeReads != 0 {
		t.Fatal(r, e)
	}
	// Filtering can force explained exact fallback; prior graph work stays charged.
	r, e = db.SparseSearch(ctx, "items", "v", f.query, SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10, EFSearch: 10, MaxEFSearch: 10, Where: "id<3"})
	if e != nil || !r.Plan.Exact || r.Stats.ANNDistanceEvaluations == 0 || len(r.Hits) != 2 {
		t.Fatal(r, e)
	}
	limits = DefaultSparseSearchLimits()
	limits.MaxNonzeros = uint64(r.Stats.ScoredNonzeros + r.Stats.ANNWork.ComponentWork - 1)
	check(SparseSearchOptions{Mode: VectorModeApproximate, Limit: 10, EFSearch: 10, MaxEFSearch: 10, Where: "id<3", Limits: limits}, vector.ErrLimit)
}

func TestTypedANNMaximumValuesAndNullPolicies(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE widths(id INTEGER PRIMARY KEY,h HALFVEC(4000),b BIT(64000),s SPARSEVEC(1000000000))")
	halfValues := make([]float32, 4000)
	halfValues[3999] = 1
	half, e := vector.NewHalf(halfValues)
	if e != nil {
		t.Fatal(e)
	}
	bitBytes := make([]byte, 8000)
	bitBytes[7999] = 1
	bits, e := vector.NewBit(64000, bitBytes)
	if e != nil {
		t.Fatal(e)
	}
	entries := make([]vector.SparseEntry, 1000)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i * 999999), Value: 1}
	}
	sparse, e := vector.NewSparse(1000000000, entries)
	if e != nil {
		t.Fatal(e)
	}
	reviewVectorExec(t, db, "INSERT INTO widths VALUES(1,?,?,?),(2,NULL,NULL,NULL)", half, bits, sparse)
	for _, field := range []string{"h", "b", "s"} {
		if e = db.CreateHNSWIndex(ctx, "idx_"+field, "widths", field); e != nil {
			t.Fatal(e)
		}
	}
	h, e := db.HalfSearch(ctx, "widths", "h", half, HalfSearchOptions{Mode: VectorModeApproximate, Limit: 1})
	if e != nil || len(h.Hits) != 1 || h.Hits[0].Distance != 0 || h.Plan.Exact {
		t.Fatal(h, e)
	}
	b, e := db.BitSearch(ctx, "widths", "b", bits, BitSearchOptions{Mode: VectorModeApproximate, Limit: 1})
	if e != nil || len(b.Hits) != 1 || b.Hits[0].Distance != 0 || b.Plan.Exact {
		t.Fatal(b, e)
	}
	s, e := db.SparseSearch(ctx, "widths", "s", sparse, SparseSearchOptions{Mode: VectorModeApproximate, Limit: 1})
	if e != nil || len(s.Hits) != 1 || s.Hits[0].Distance != 0 || s.Plan.Exact || s.Stats.ANNWork.ComponentWork > 6000 {
		t.Fatal(s, e)
	}
	_, p, _ := reviewVectorQuery(t, db, "SELECT id FROM widths ORDER BY b <~> ? LIMIT 1", bits)
	if !p.VectorExact || p.ANNFallback != "SQL NULL-first ordering requires a declared NOT NULL vector column" {
		t.Fatal(p)
	}
	reviewVectorExec(t, db, "CREATE TABLE zeros(id INTEGER PRIMARY KEY,b BIT(8) NOT NULL,s SPARSEVEC(1000000000) NOT NULL)")
	reviewVectorExec(t, db, "INSERT INTO zeros VALUES(1,'00000000','{}/1000000000'),(2,'00000000','{}/1000000000')")
	if e = db.CreateHNSWIndex(ctx, "zero_bit", "zeros", "b", HNSWIndexOptions{Opclass: "bit_jaccard_ops"}); e != nil {
		t.Fatal(e)
	}
	if e = db.CreateHNSWIndex(ctx, "zero_sparse", "zeros", "s"); e != nil {
		t.Fatal(e)
	}
	bz, e := db.BitSearch(ctx, "zeros", "b", "00000000", BitSearchOptions{Mode: VectorModeApproximate, Metric: vector.Jaccard, Limit: 2})
	if e != nil || len(bz.Hits) != 2 || bz.Hits[0].Distance != 1 {
		t.Fatal(bz, e)
	}
	sz, e := db.SparseSearch(ctx, "zeros", "s", "{}/1000000000", SparseSearchOptions{Mode: VectorModeApproximate, Limit: 2, Limits: SparseSearchLimits{MaxCandidates: 2, MaxResults: 2, MaxNonzeros: 1}})
	if e != nil || len(sz.Hits) != 2 || sz.Stats.ANNWork.ComponentWork != 0 || sz.Stats.ScoredNonzeros != 0 {
		t.Fatal(sz, e)
	}
}

func TestTypedANNSharedSnapshotAndBusy(t *testing.T) {
	ctx := context.Background()
	opts := Options{WALShared: true}
	db, path := reviewVectorDB(t, opts)
	reviewVectorExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v HALFVEC(2) NOT NULL)")
	reviewVectorExec(t, db, "INSERT INTO items VALUES(1,'[1,1]'),(2,'[2,1]'),(3,'[3,1]')")
	if e := db.CreateHNSWIndex(ctx, "idx", "items", "v"); e != nil {
		t.Fatal(e)
	}
	peer, e := Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	old, e := db.Begin(ctx, TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer old.Rollback()
	before, e := old.HalfSearch(ctx, "items", "v", "[0,1]", HalfSearchOptions{Mode: VectorModeApproximate, Limit: 3})
	if e != nil {
		t.Fatal(e)
	}
	reviewVectorExec(t, peer, "UPDATE items SET v='[20,1]' WHERE id=1")
	if e = peer.Reindex(ctx, "idx"); e != nil {
		t.Fatal(e)
	}
	retained, e := old.HalfSearch(ctx, "items", "v", "[0,1]", HalfSearchOptions{Mode: VectorModeApproximate, Limit: 3})
	if e != nil || !reflect.DeepEqual(retained.Hits, before.Hits) || retained.Plan.ANNGeneration != before.Plan.ANNGeneration {
		t.Fatal(retained, e)
	}
	current, e := peer.HalfSearch(ctx, "items", "v", "[0,1]", HalfSearchOptions{Mode: VectorModeApproximate, Limit: 3})
	if e != nil || current.Hits[0].RowID != 2 || current.Plan.ANNGeneration != 2 {
		t.Fatal(current, e)
	}
	if e = old.Rollback(); e != nil {
		t.Fatal(e)
	}
	writer, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Rollback()
	if _, e = writer.Exec(ctx, "UPDATE items SET v='[30,1]' WHERE id=2"); e != nil {
		t.Fatal(e)
	}
	if _, e = peer.Exec(ctx, "UPDATE items SET v='[40,1]' WHERE id=3"); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	if e = writer.Rollback(); e != nil {
		t.Fatal(e)
	}
	reviewVectorExec(t, peer, "UPDATE items SET v='[40,1]' WHERE id=3")
	if e = peer.CheckExtensionIndex(ctx, "idx"); e != nil {
		t.Fatal(e)
	}
}

func TestTypedANNIVFUntrainedAndExplicitAllProbeFallback(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v HALFVEC(2) NOT NULL)")
	if e := db.CreateIVFFlatIndex(ctx, "idx", "items", "v", IVFFlatIndexOptions{Lists: 2}); e != nil {
		t.Fatal(e)
	}
	reviewVectorExec(t, db, "INSERT INTO items VALUES(1,'[1,1]'),(2,'[2,1]'),(3,'[20,1]'),(4,'[21,1]')")
	r, e := db.HalfSearch(ctx, "items", "v", "[1,1]", HalfSearchOptions{Mode: VectorModeApproximate, Limit: 1})
	if e != nil || !r.Plan.Exact || r.Plan.Method != "exact_halfvec" || r.Plan.ANNTrainingState != "untrained" || r.Stats.ANNCentroidReads != 0 {
		t.Fatal(r, e)
	}
	if e = db.Reindex(ctx, "idx"); e != nil {
		t.Fatal(e)
	}
	r, e = db.HalfSearch(ctx, "items", "v", "[1,1]", HalfSearchOptions{Mode: VectorModeApproximate, Limit: 1, Probes: 2})
	if e != nil || !r.Plan.Exact || r.Plan.ANNFallback != "probes equals lists" || r.Stats.ANNMemberKeysRead != 0 {
		t.Fatal(r, e)
	}
	r, e = db.HalfSearch(ctx, "items", "v", "[1,1]", HalfSearchOptions{Mode: VectorModeApproximate, Limit: 1, Probes: 1})
	if e != nil || r.Plan.Exact || r.Stats.ANNListsOpened != 1 || r.Stats.ANNWork.ComponentWork != 4 {
		t.Fatal(r, e)
	}
}

// Pure-memory source identity checks, never physical database modifications.
func TestTypedANNExactRepresentationRecheckAndSQLWork(t *testing.T) {
	for _, kind := range []ann.ValueKind{ann.HalfKind, ann.BitKind, ann.SparseKind} {
		metric := vector.L2
		dims := 2
		value := "[0,1]"
		different := "[-0,1]"
		if kind == ann.BitKind {
			metric = vector.MetricKind(vector.Hamming)
			value = "01"
			different = "10"
		}
		if kind == ann.SparseKind {
			dims = 1000000000
			value = "{1:1}/1000000000"
			different = "{2:1}/1000000000"
		}
		cfg, e := ann.DefaultTypedConfig(ann.HNSW, kind, dims, metric)
		if e != nil {
			t.Fatal(e)
		}
		original, _, e := decodeANNColumn(value, cfg)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = decodeTypedANNCandidate(value, cfg, &original); e != nil {
			t.Fatal(e)
		}
		for _, bad := range []any{nil, different, []byte("[0,1]")} {
			if _, e = decodeTypedANNCandidate(bad, cfg, &original); !errors.Is(e, ErrCorrupt) {
				t.Fatal(kind, bad, e)
			}
		}
	}
	f := newTypedANNFixture(t, "sparsevec", 32)
	db, _ := typedFixtureDB(t, f)
	ctx := context.Background()
	if e := db.CreateHNSWIndex(ctx, "idx", "items", "v"); e != nil {
		t.Fatal(e)
	}
	rows, e := db.Query(ctx, "SELECT id FROM items ORDER BY v <-> ? LIMIT 3", f.query)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for rows.Next() {
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	work := rows.ANNWork()
	if work.Unit != "nonzeros" || work.ComponentWork == 0 || work.ComponentWork > 6*rows.Stats().ANNDistanceEvaluations {
		t.Fatal(work, rows.Stats())
	}
}

func TestTypedANNSparseInputZeroEntryBudgets(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewVectorDB(t, Options{MaxSortBytes: 4096})
	reviewVectorExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v SPARSEVEC(1000000000) NOT NULL)")
	reviewVectorExec(t, db, "INSERT INTO items VALUES(1,'{}/1000000000')")
	if e := db.CreateHNSWIndex(ctx, "idx", "items", "v"); e != nil {
		t.Fatal(e)
	}
	text := "{"
	for i := 1; i <= 160; i++ {
		if i > 1 {
			text += ","
		}
		text += fmt.Sprintf("%d:0", i)
	}
	text += "}/1000000000"
	// The input canonicalizes to zero NNZ but still needs parser entry scratch.
	rows, e := db.Query(ctx, "SELECT id FROM items ORDER BY v <-> ? LIMIT 1", text)
	if e == nil {
		for rows.Next() {
		}
		e = rows.Err()
		rows.Close()
	}
	if !errors.Is(e, ErrMemoryLimit) {
		t.Fatal("unadmitted zero-bearing query", e)
	}
	empty, _, _ := reviewVectorQuery(t, db, "SELECT id FROM items WHERE 0 ORDER BY v <-> ? LIMIT 1", text)
	if len(empty) != 0 {
		t.Fatal(empty)
	}
	small := "{1:0,2:0,3:0}/1000000000"
	r, e := db.SparseSearch(ctx, "items", "v", small, SparseSearchOptions{Mode: VectorModeApproximate, Limit: 1})
	if e != nil || r.Stats.ScoredNonzeros != 6 || r.Stats.ANNWork.ComponentWork != 0 {
		t.Fatal(r, e)
	}
	limits := DefaultSparseSearchLimits()
	limits.MaxNonzeros = 5
	r, e = db.SparseSearch(ctx, "items", "v", small, SparseSearchOptions{Mode: VectorModeApproximate, Limit: 1, Limits: limits})
	if !errors.Is(e, vector.ErrLimit) || !reflect.DeepEqual(r, SparseResult{}) {
		t.Fatal(r, e)
	}
	rows, e = db.Query(ctx, "SELECT id FROM items ORDER BY v <-> ? LIMIT 1", small)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
	}
	e = rows.Err()
	s := rows.Stats()
	w := rows.ANNWork()
	rows.Close()
	if e != nil || s.VectorRankNonzeros != 6 || w.ComponentWork != 0 || rows.Plan().Method != "hnsw" {
		t.Fatal(s, w, e)
	}
}
