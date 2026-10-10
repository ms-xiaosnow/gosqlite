package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Invalid typed values are descriptor-free evaluator inputs only. The SQL and
// public-search tests below create intact fixtures with ordinary valid writes.
func TestBooleanRankMemoryEntrypoints(t *testing.T) {
	const boom = "abs(-9223372036854775808)"
	for _, tc := range []struct {
		where string
		bad   bool
	}{
		{"0", false}, {"NULL", false}, {"0 AND " + boom, false},
		{boom + " AND 0", true}, {"active AND s", false}, {"s AND active", true},
	} {
		for _, heap := range []bool{false, true} {
			r := sparseReviewMemoryRows(t, "invalid sparse value", "invalid query", heap)
			r.source = &reviewVectorSource{rows: [][]any{{nil, "invalid sparse value", int64(0)}}}
			r.sel.Where, _ = sqlast.ParseExpr(tc.where)
			err := r.materialize()
			if (err != nil) != tc.bad || len(r.sorted) != 0 {
				t.Fatalf("%s heap=%t: %v %#v", tc.where, heap, err, r.sorted)
			}
		}
	}
	// Direct ANN-style source rechecks must not depend on an earlier fetch call.
	for _, where := range []string{"0", boom + " AND 0"} {
		r := sparseReviewMemoryRows(t, "invalid", "invalid", true)
		r.sel.Where, _ = sqlast.ParseExpr(where)
		for i := 0; i < 2; i++ {
			_, _, keep, err := r.evaluateSourceRow(int64(i), []any{nil, "invalid", int64(0)})
			if keep || (err != nil) != strings.HasPrefix(where, "abs") || r.queryStats.BaseRowFetches != 0 {
				t.Fatal(where, i, keep, err, r.queryStats)
			}
		}
	}
	// Once false, startup stays false. Cancellation remains observable afterward.
	r := sparseReviewMemoryRows(t, "invalid", "invalid", true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.ctx = ctx
	r.sel.Where = &sqlast.Literal{Value: int64(0)}
	for i := 0; i < 2; i++ {
		if err := r.startPredicates(); err != io.EOF {
			t.Fatal(i, err)
		}
	}
	cancel()
	if err := r.startPredicates(); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// False startup avoids half scratch just as it avoids sparse/dense decoding.
	h := halfReviewMemoryRows(t, "invalid", "invalid", true)
	h.sel.Where = &sqlast.Literal{Value: int64(0)}
	h.db.opts.MaxSortBytes = 1
	if err := h.materialize(); err != nil || len(h.sorted) != 0 {
		t.Fatal(err, h.sorted)
	}
}

func TestBooleanRankSQLStartupAcrossRepresentations(t *testing.T) {
	ctx := context.Background()
	const boom = "abs(-9223372036854775808)"
	for _, tc := range []struct{ kind, decl, value, op string }{
		{"dense", "VECTOR(2)", "[1,1]", "<->"},
		{"half", "HALFVEC(2)", "[1,1]", "<->"},
		{"sparse", "SPARSEVEC(20)", "{1:1,2:1}/20", "<->"},
		{"bit", "BIT(2)", "01", "<~>"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			db, _ := reviewVectorDB(t, Options{})
			reviewVectorExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v "+tc.decl+" NOT NULL,active INTEGER)")
			reviewVectorExec(t, db, "INSERT INTO items VALUES(1,?,0),(2,?,1)", tc.value, tc.value)
			if err := db.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
				t.Fatal(err)
			}
			for _, path := range []struct{ modifier, tail string }{{"", "LIMIT 2"}, {" NOT INDEXED", "LIMIT 2"}, {"", "DESC LIMIT 2"}} {
				for _, query := range []any{tc.value, "invalid query", nil} {
					for _, where := range []string{"0", "NULL", "0 AND " + boom, "active AND 0"} {
						sql := fmt.Sprintf("SELECT id FROM items%s WHERE %s ORDER BY v %s ? %s", path.modifier, where, tc.op, path.tail)
						got, _, stats := reviewVectorQuery(t, db, sql, query)
						if len(got) != 0 || stats.BaseRowFetches != 0 || stats.ANNNodeReads != 0 || stats.ANNDistanceEvaluations != 0 || stats.VectorRankComponents != 0 || stats.VectorRankNonzeros != 0 || stats.VectorRankPackedBytes != 0 {
							t.Fatal(sql, query, got, stats)
						}
					}
				}
				for _, where := range []string{boom + " AND 0", "active AND " + boom} {
					sql := fmt.Sprintf("SELECT id FROM items%s WHERE %s ORDER BY v %s ? %s", path.modifier, where, tc.op, path.tail)
					rows, err := db.Query(ctx, sql, tc.value)
					if rows != nil {
						rows.Close()
					}
					if err == nil || !strings.Contains(err.Error(), "integer overflow") {
						t.Fatal(sql, err)
					}
				}
			}
			// Alias expansion and residual truth must survive ANN candidate rechecks.
			got, plan, stats := reviewVectorQuery(t, db, "SELECT id,active AS keep FROM items WHERE keep AND id>0 ORDER BY v "+tc.op+" ? LIMIT 2", tc.value)
			if !reflect.DeepEqual(got, [][]any{{int64(2), int64(1)}}) || plan.Method != "hnsw" || plan.VectorRanking != "ann_exact_rescore" || stats.ANNExactRescores == 0 || stats.VectorFilteredRows == 0 {
				t.Fatal(got, plan, stats)
			}
			// Empty ANN sources still evaluate constant errors before traversal.
			reviewVectorExec(t, db, "DELETE FROM items")
			rows, err := db.Query(ctx, "SELECT id FROM items WHERE "+boom+" ORDER BY v "+tc.op+" ? LIMIT 1", tc.value)
			if rows != nil {
				rows.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "integer overflow") {
				t.Fatal(err)
			}
		})
	}
}

func TestBooleanPublicFilterTypedLazinessAndTerminalStartup(t *testing.T) {
	tab := &table{name: "items", ipk: -1, index: map[string]int{"active": 0, "v": 1}, columns: []sqlast.ColumnDef{{Name: "active", Type: "INTEGER"}, {Name: "v", Type: "VECTOR(2)"}}}
	const boom = "abs(-9223372036854775808)"
	for _, tc := range []struct {
		where     string
		keep, bad bool
	}{
		{"0", false, false}, {"0 AND " + boom, false, false}, {boom + " AND 0", false, true},
		{"active AND v", false, false}, {"v AND active", false, true},
		{"1 OR v", true, false}, {"v OR 1", true, false}, {"active OR v", false, true},
	} {
		f, err := (&DB{}).similarityFilter(context.Background(), nil, tab, SimilarityOptions{Where: tc.where})
		if err != nil {
			t.Fatal(tc.where, err)
		}
		for i := 0; i < 2; i++ {
			keep, err := f(int64(i), []any{int64(0), "invalid vector value"})
			if keep != tc.keep || (err != nil) != tc.bad {
				t.Fatal(tc.where, i, keep, err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	f, err := (&DB{}).similarityFilter(ctx, nil, tab, SimilarityOptions{Where: "0"})
	if err != nil {
		t.Fatal(err)
	}
	if keep, err := f(1, []any{int64(0), nil}); keep || err != nil {
		t.Fatal(keep, err)
	}
	cancel()
	if _, err := f(2, []any{int64(0), nil}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestBooleanPublicVectorFiltersAndBudgets(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL,h HALFVEC(2) NOT NULL,s SPARSEVEC(20) NOT NULL,b BIT(2) NOT NULL,active INTEGER)")
	for _, id := range []int{1, 2} {
		reviewVectorExec(t, db, "INSERT INTO items VALUES(?,'[1,1]','[1,1]','{1:1,2:1}/20','01',0)", id)
	}
	for _, field := range []string{"v", "h", "s", "b"} {
		if err := db.CreateHNSWIndex(ctx, "knn_"+field, "items", field); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []VectorSearchMode{VectorModeExact, VectorModeApproximate} {
		for _, tc := range []struct {
			where string
			want  int
			bad   bool
		}{{"active AND abs(-9223372036854775808+id-1)", 0, false}, {"0 AND abs(-9223372036854775808)", 0, false}, {"abs(-9223372036854775808) OR 1", 2, false}, {"abs(-9223372036854775808) AND 0", 0, true}} {
			v, ve := db.VectorSearch(ctx, "items", "v", "[1,1]", VectorSearchOptions{Mode: mode, Limit: 2, Where: tc.where})
			h, he := db.HalfSearch(ctx, "items", "h", "[1,1]", HalfSearchOptions{Mode: mode, Limit: 2, Where: tc.where})
			s, se := db.SparseSearch(ctx, "items", "s", "{1:1,2:1}/20", SparseSearchOptions{Mode: mode, Limit: 2, Where: tc.where})
			b, be := db.BitSearch(ctx, "items", "b", "01", BitSearchOptions{Mode: mode, Limit: 2, Where: tc.where})
			for _, got := range []struct {
				n   int
				err error
			}{{len(v.Hits), ve}, {len(h.Hits), he}, {len(s.Hits), se}, {len(b.Hits), be}} {
				if got.n != tc.want || (got.err != nil) != tc.bad {
					t.Fatal(mode, tc.where, got)
				}
			}
		}
	}
	limits := vector.DefaultSearchLimits()
	limits.MaxCandidates = 1
	if _, err := db.VectorSearch(ctx, "items", "v", "[1,1]", VectorSearchOptions{Limit: 1, Where: "0", Limits: limits}); !errors.Is(err, vector.ErrLimit) {
		t.Fatal("filtered candidates escaped budget", err)
	}
}

func TestBooleanPublicSimilarityFilter(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,active INTEGER,n INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO docs VALUES(1,'abc',0,-9223372036854775808),(2,'abc',0,7)")
	if err := db.CreateGramsIndex(ctx, "doc_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	for _, scan := range []bool{false, true} {
		for _, tc := range []struct {
			where string
			want  int
			bad   bool
		}{{"0", 0, false}, {"active AND abs(n)", 0, false}, {"abs(-9223372036854775808) OR 1", 2, false}, {"abs(-9223372036854775808) AND 0", 0, true}} {
			result, err := db.SimilaritySearch(ctx, "doc_grams", "body", "abc", SimilarityOptions{Where: tc.where, Threshold: 0.5, Limit: 2, ForceScan: scan})
			if len(result.Hits) != tc.want || (err != nil) != tc.bad {
				t.Fatal(scan, tc.where, result, err)
			}
		}
	}
}

func TestBooleanRankIVFStartupAndFallback(t *testing.T) {
	ctx := context.Background()
	for _, lists := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(lists), func(t *testing.T) {
			db, _ := reviewVectorDB(t, Options{})
			reviewVectorExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
			reviewVectorExec(t, db, "INSERT INTO items VALUES(1,'[1,1]'),(2,'[2,1]')")
			if err := db.CreateIVFFlatIndex(ctx, "ivf", "items", "v", IVFFlatIndexOptions{Lists: lists}); err != nil {
				t.Fatal(err)
			}
			for _, query := range []any{"[1,1]", "invalid", nil} {
				got, _, stats := reviewVectorQuery(t, db, "SELECT id FROM items WHERE 0 ORDER BY v <-> ? LIMIT 2", query)
				if len(got) != 0 || stats.BaseRowFetches != 0 || stats.ANNCentroidReads != 0 || stats.ANNMemberKeysRead != 0 || stats.VectorRankComponents != 0 {
					t.Fatal(lists, query, got, stats)
				}
			}
			rows, err := db.Query(ctx, "SELECT id FROM items WHERE abs(-9223372036854775808) AND 0 ORDER BY v <-> '[1,1]' LIMIT 2")
			if rows != nil {
				rows.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "integer overflow") {
				t.Fatal(lists, err)
			}
			// Exercise the successful fallback as well as startup short-circuiting.
			got, _, _ := reviewVectorQuery(t, db, "SELECT id FROM items WHERE id>0 ORDER BY v <-> '[1,1]' LIMIT 2")
			if !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(2)}}) {
				t.Fatal(lists, got)
			}
		})
	}
}
