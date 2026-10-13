package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func pgVectorDDLOpen(t *testing.T, path string) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(path, sqlite.Options{})
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

func pgVectorDDLExec(t *testing.T, db interface {
	Exec(context.Context, string, ...any) (sqlite.Result, error)
}, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(query, err)
	}
}

type pgVectorDDLObservation struct {
	check    sqlite.CheckResult
	schema   []sqlite.SchemaEntry
	metadata []sqlite.ExtensionIndexMetadata
	ids      []int64
	plan     sqlite.QueryPlan
	stats    sqlite.QueryStats
	native   sqlite.VectorResult
}

func pgVectorDDLObserve(t *testing.T, db *sqlite.DB, method, name, opclass, operator string, metric vector.MetricKind) pgVectorDDLObservation {
	t.Helper()
	ctx := context.Background()
	var got pgVectorDDLObservation
	var err error
	got.check, err = db.Check(ctx)
	if err != nil || got.check.Scope != sqlite.CheckScope || got.check.ExtensionIndexes != 1 || got.check.PageReads == 0 {
		t.Fatal(got.check, err)
	}
	if err = db.CheckExtensionIndex(ctx, name); err != nil {
		t.Fatal(err)
	}
	inspected, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: true})
	if err != nil {
		t.Fatal(err)
	}
	got.schema, got.metadata = inspected.Schema, inspected.ExtensionIndexes
	if len(got.metadata) != 1 {
		t.Fatal(got.metadata)
	}
	meta := got.metadata[0]
	if meta.Name != name || meta.SourceTable != "items" || meta.Method != method || meta.MethodVersion != 1 || meta.StorageVersion != 1 || meta.Generation != 1 {
		t.Fatal(meta)
	}
	cfg, err := ann.ParseConfig([]byte(meta.Config))
	if err != nil || cfg.Method != method || cfg.Opclass != opclass || cfg.Metric != metric || cfg.Dimensions != 3 {
		t.Fatal(cfg, err)
	}
	if (method == "hnsw" && (cfg.M != 16 || cfg.EFConstruction != 64)) || (method == "ivfflat" && cfg.Lists != 100) {
		t.Fatal(cfg)
	}
	for _, entry := range got.schema {
		if entry.Name == name || strings.Contains(strings.ToLower(entry.SQL), "using "+method) || strings.Contains(entry.SQL, opclass) {
			t.Fatalf("extension declaration leaked into sqlite_schema: %#v", entry)
		}
	}
	rows, err := db.Query(ctx, "SELECT id FROM items ORDER BY embedding "+operator+" '[1,12,1]' LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		got.ids = append(got.ids, rows.Values()[0].(int64))
	}
	readErr := rows.Err()
	got.plan, got.stats = rows.Plan(), rows.Stats()
	closeErr := rows.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal(readErr, closeErr)
	}
	if !reflect.DeepEqual(got.ids, []int64{1}) || got.plan.Method != method || got.plan.Access != "SEARCH" || got.plan.VectorExact || got.plan.ANNFallback != "" || got.plan.ANNOpclass != opclass || got.plan.VectorMetric != metric || got.stats.ANNExactRescores == 0 || got.stats.ANNIdentityLookups == 0 {
		t.Fatal(got.ids, got.plan, got.stats)
	}
	if method == "hnsw" && (got.stats.ANNNodeReads == 0 || got.stats.ANNEdgeReads == 0 || got.stats.ANNDistanceEvaluations == 0 || got.stats.ANNListsOpened != 0) {
		t.Fatalf("no real graph traversal: %+v", got.stats)
	}
	if method == "ivfflat" && (got.plan.ANNTrainingState != "ready" || got.stats.ANNCentroidReads != 100 || got.stats.ANNListsOpened < 1 || got.stats.ANNListsOpened >= 100 || got.stats.ANNMemberKeysRead == 0 || got.stats.ANNNodeReads != 0) {
		t.Fatalf("no real selected-list search: %+v %+v", got.plan, got.stats)
	}
	got.native, err = db.VectorSearch(ctx, "items", "embedding", "[1,12,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: name, Metric: metric, Limit: 1})
	if err != nil || got.native.Plan.Method != method || got.native.Plan.Exact || len(got.native.Hits) != 1 || got.native.Hits[0].RowID != 1 {
		t.Fatal(got.native, err)
	}
	// A different query metric must retain the existing explained exact fallback.
	otherMetric := vector.L2
	if metric == vector.L2 {
		otherMetric = vector.Cosine
	}
	mismatch, err := db.VectorSearch(ctx, "items", "embedding", "[1,12,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: name, Metric: otherMetric, Limit: 1})
	if err != nil || !mismatch.Plan.Exact || mismatch.Plan.Method != "exact_vector" || mismatch.Plan.ANNFallback == "" || mismatch.Stats.ANNNodeReads != 0 || mismatch.Stats.ANNCentroidReads != 0 {
		t.Fatal(mismatch, err)
	}
	return got
}

func TestPGVectorIndexHealthyEquivalenceAndReopen(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		method, name, opclass, operator, pg, local string
		metric                                     vector.MetricKind
	}{
		{
			"hnsw", "items_embedding_hnsw", "vector_cosine_ops", "<=>",
			`CREATE INDEX items_embedding_hnsw ON items USING hnsw
  (embedding vector_cosine_ops) WITH (m=16, ef_construction=64);`,
			`CREATE INDEX items_embedding_hnsw ON items(embedding) USING hnsw WITH(opclass='vector_cosine_ops',m=16,ef_construction=64);`,
			vector.Cosine,
		},
		{
			"ivfflat", "items_embedding_ivf", "vector_l2_ops", "<->",
			`CREATE INDEX items_embedding_ivf ON items USING ivfflat
  (embedding vector_l2_ops) WITH (lists=100);`,
			`CREATE INDEX items_embedding_ivf ON items(embedding) USING ivfflat WITH(opclass='vector_l2_ops',lists=100);`,
			vector.L2,
		},
	} {
		t.Run(tc.method, func(t *testing.T) {
			var baseline pgVectorDDLObservation
			for i, ddl := range []string{tc.local, tc.pg} {
				path := filepath.Join(t.TempDir(), "ordinary.db")
				db := pgVectorDDLOpen(t, path)
				pgVectorDDLExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY, embedding VECTOR(3) NOT NULL)")
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for id := 1; id <= 128; id++ {
					pgVectorDDLExec(t, tx, "INSERT INTO items VALUES(?,?)", id, fmt.Sprintf("[%d,%d,1]", id, (id%7)*11+1))
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				pgVectorDDLExec(t, db, ddl)
				before := pgVectorDDLObserve(t, db, tc.method, tc.name, tc.opclass, tc.operator, tc.metric)
				if i == 0 {
					baseline = before
				} else if !reflect.DeepEqual(before, baseline) {
					t.Fatalf("PG and original syntax differ:\nPG %#v\noriginal %#v", before, baseline)
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				reopened := pgVectorDDLOpen(t, path)
				after := pgVectorDDLObserve(t, reopened, tc.method, tc.name, tc.opclass, tc.operator, tc.metric)
				if !reflect.DeepEqual(after, before) {
					t.Fatalf("normal reopen changed observation:\nbefore %#v\nafter %#v", before, after)
				}
			}
		})
	}
}

func TestPGVectorIndexExistingAdmissionAndTransaction(t *testing.T) {
	ctx := context.Background()
	db := pgVectorDDLOpen(t, filepath.Join(t.TempDir(), "admission.db"))
	pgVectorDDLExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY, embedding VECTOR(3) NOT NULL, h HALFVEC(3), s SPARSEVEC(3), b BIT(3), plain TEXT)")
	pgVectorDDLExec(t, db, "INSERT INTO items(id,embedding) VALUES(1,'[1,2,3]'),(2,'[2,3,4]')")
	before, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{
		"hnsw (embedding vector_l2_ops) WITH(m=0)",
		"hnsw (embedding vector_l2_ops) WITH(m=-1)",
		"hnsw (embedding vector_l2_ops) WITH(m=1)",
		"hnsw (embedding vector_l2_ops) WITH(m=101)",
		"hnsw (embedding vector_l2_ops) WITH(m=100)",
		"hnsw (embedding vector_l2_ops) WITH(ef_construction=0)",
		"hnsw (embedding vector_l2_ops) WITH(ef_construction=4)",
		"hnsw (embedding vector_l2_ops) WITH(ef_construction=1001)",
		"hnsw (embedding vector_l2_ops) WITH(m=16,ef_construction=16)",
		"hnsw (embedding vector_l2_ops) WITH(m=9223372036854775807)",
		"ivfflat (embedding vector_l2_ops) WITH(lists=0)",
		"ivfflat (embedding vector_l2_ops) WITH(lists=-1)",
		"ivfflat (embedding vector_l2_ops) WITH(lists=32769)",
		"ivfflat (embedding vector_l2_ops) WITH(lists=9223372036854775807)",
		"hnsw (h vector_l2_ops)",
		"hnsw (s vector_l2_ops)",
		"hnsw (b vector_l2_ops)",
		"ivfflat (h vector_l2_ops)",
		"ivfflat (s vector_l2_ops)",
		"ivfflat (b vector_l2_ops)",
		"hnsw (plain vector_l2_ops)",
		"hnsw (missing vector_l2_ops)",
	} {
		if _, err = db.Exec(ctx, "CREATE INDEX invalid ON items USING "+tail); err == nil {
			t.Fatalf("existing admission accepted %s", tail)
		}
		after, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: true})
		if err != nil || !reflect.DeepEqual(after.Schema, before.Schema) || !reflect.DeepEqual(after.ExtensionIndexes, before.ExtensionIndexes) {
			t.Fatalf("rejected DDL changed schema/catalog: %s: %#v %v", tail, after, err)
		}
	}
	// The public Tx entrance shares the parser and existing catalog ownership.
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pgVectorDDLExec(t, tx, `CREATE INDEX "quoted index" ON "items" USING HNSW ("embedding" VECTOR_COSINE_OPS) WITH(M=16,EF_CONSTRUCTION=64)`)
	if err = tx.CheckExtensionIndex(ctx, "quoted index"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	inspected, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: true})
	if err != nil || len(inspected.ExtensionIndexes) != 1 || inspected.ExtensionIndexes[0].Name != "quoted index" {
		t.Fatal(inspected, err)
	}
	pgVectorDDLExec(t, db, `CREATE INDEX IF NOT EXISTS "quoted index" ON "items" USING hnsw ("embedding" vector_cosine_ops) WITH(m=16,ef_construction=64)`)
	again, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: true})
	if err != nil || !reflect.DeepEqual(again.Schema, inspected.Schema) || !reflect.DeepEqual(again.ExtensionIndexes, inspected.ExtensionIndexes) {
		t.Fatal(again, err)
	}
	checked, err := db.Check(ctx)
	if err != nil || checked.ExtensionIndexes != 1 {
		t.Fatal(checked, err)
	}
}
