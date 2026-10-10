package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestANNStorageHNSWLifecycleAndReopen(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{MaxTransactionPages: 20000})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY, v VECTOR(3), label TEXT)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := -5; i < 75; i++ {
		txExecute(t, tx, "INSERT INTO items VALUES(?,?,?)", i, fmt.Sprintf("[%d,%d,%d]", i, i%7, i%11), "same")
	}
	txExecute(t, tx, "INSERT INTO items VALUES(100,NULL,'null')")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	state := extensionRead(t, db, "SELECT live_count,tombstone_count,next_node_id,max_level FROM __gosqlite_ext_ann_state")
	if state[0][0] != int64(80) || state[0][1] != int64(0) || state[0][2] != int64(81) || state[0][3].(int64) < 1 {
		t.Fatalf("not a multilayer graph: %v", state)
	}
	before := extensionRead(t, db, "SELECT node_id FROM __gosqlite_ext_ann_live WHERE content_rowid=1")
	execute(t, db, "UPDATE items SET label='changed' WHERE id=1")
	if after := extensionRead(t, db, "SELECT node_id FROM __gosqlite_ext_ann_live WHERE content_rowid=1"); !reflect.DeepEqual(before, after) {
		t.Fatalf("nonvector update changed node %v -> %v", before, after)
	}
	execute(t, db, "UPDATE items SET v='[999,1,2]' WHERE id=1")
	execute(t, db, "UPDATE items SET id=-100 WHERE id=2")
	execute(t, db, "DELETE FROM items WHERE id=3")
	execute(t, db, "INSERT INTO items VALUES(3,'[3,2,1]','reused')")
	execute(t, db, "UPDATE items SET v='[5,5,5]' WHERE id=100")
	execute(t, db, "UPDATE items SET v=NULL WHERE id=4")
	if err = db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	state = extensionRead(t, db, "SELECT live_count,tombstone_count,next_node_id FROM __gosqlite_ext_ann_state")
	if state[0][0] != int64(80) || state[0][1] != int64(4) || state[0][2] != int64(85) {
		t.Fatalf("state after mutation %v", state)
	}
	if got := extensionRead(t, db, "SELECT live FROM __gosqlite_ext_ann_nodes WHERE node_id=?", before[0][0]); got[0][0] != int64(0) {
		t.Fatalf("old identity reused %v", got)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "DELETE FROM items WHERE id=-100")
	if err = tx.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT live_count,tombstone_count,next_node_id FROM __gosqlite_ext_ann_state"); !reflect.DeepEqual(state, got) {
		t.Fatalf("rollback %v != %v", state, got)
	}
	if err = db.Reindex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	state = extensionRead(t, db, "SELECT live_count,tombstone_count,next_node_id FROM __gosqlite_ext_ann_state")
	if !reflect.DeepEqual(state, [][]any{{int64(80), int64(0), int64(81)}}) {
		t.Fatalf("reindex retained tombstones %v", state)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
}

func TestANNStorageLimitsAndStatementRollback(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),note TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2]','a'),(2,'[2,3]','b')")
	if err := db.CreateHNSWIndex(ctx, "knn", "items", "v", sqlite.HNSWIndexOptions{MaxNodes: 2}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE items SET note='earlier' WHERE id=1")
	if _, err = tx.Exec(ctx, "UPDATE items SET v='[9,9]' WHERE id=2"); !errors.Is(err, ann.ErrLimit) {
		t.Fatalf("update should exceed node budget: %v", err)
	}
	if err = tx.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := extensionRead(t, db, "SELECT id,vector_to_text(v),note FROM items ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(1), "[1,2]", "earlier"}, {int64(2), "[2,3]", "b"}}) {
		t.Fatalf("statement undo lost prior work: %v", got)
	}
	if _, err = db.Exec(ctx, "INSERT INTO items VALUES(3,'[3,4]','c')"); !errors.Is(err, ann.ErrLimit) {
		t.Fatalf("insert quota: %v", err)
	}
	if err = db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "DELETE FROM items WHERE id=2")
	if err = db.Reindex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "INSERT INTO items VALUES(3,'[3,4]','c')")
	if err = db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
}

func TestANNStorageFailedBuildAndCosineConstraint(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2))")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,0]'),(2,'[0,0]')")
	if err := db.CreateHNSWIndex(ctx, "bad", "items", "v", sqlite.HNSWIndexOptions{Metric: vector.Cosine}); !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatalf("cosine build zero: %v", err)
	}
	if schema, err := db.Schema(ctx); err != nil || len(schema) != 1 {
		t.Fatalf("failed build published schema: %v %v", schema, err)
	}
	execute(t, db, "UPDATE items SET v='[0,1]' WHERE id=2")
	if err := db.CreateHNSWIndex(ctx, "knn", "items", "v", sqlite.HNSWIndexOptions{Metric: vector.Cosine}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "UPDATE items SET v='[0,0]' WHERE id=1"); !errors.Is(err, sqlite.ErrConstraint) || !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatalf("zero update constraint: %v", err)
	}
	if err := db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	before := extensionRead(t, db, "SELECT generation FROM __gosqlite_ext_catalog")
	if err := db.Reindex(cancelCtx, "knn"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled reindex: %v", err)
	}
	if after := extensionRead(t, db, "SELECT generation FROM __gosqlite_ext_catalog"); !reflect.DeepEqual(before, after) {
		t.Fatalf("canceled generation changed %v", after)
	}
	if err := db.Reindex(ctx, "knn", sqlite.GramsIndexOptions{}); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("grams options on ANN %v", err)
	}
}

func TestANNStorageConfigSQLAndOptionalFamily(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),body TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2]','abc'),(2,'[2,1]','bcd')")
	if err := db.CreateGramsIndex(ctx, "grams", "items", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	oldSchema := extensionRead(t, db, "SELECT name,sql FROM sqlite_schema WHERE name LIKE '__gosqlite_ext_%' ORDER BY name")
	execute(t, db, "CREATE INDEX knn ON items(v) USING hnsw WITH(opclass='vector_l1_ops',m=4,ef_construction=16)")
	for _, name := range []string{"knn", "grams"} {
		if err := db.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	for _, old := range oldSchema {
		new := extensionRead(t, db, "SELECT name,sql FROM sqlite_schema WHERE name=?", old[0])
		if !reflect.DeepEqual(new, [][]any{old}) {
			t.Fatalf("legacy schema changed: %v => %v", old, new)
		}
	}
	for _, o := range []sqlite.HNSWIndexOptions{{M: 1}, {M: 101}, {M: 10, EFConstruction: 10}, {EFConstruction: 1001}, {Metric: vector.L1, Opclass: "vector_l2_ops"}, {Metric: "other"}, {MaxNodes: -1}, {MaxMemoryBytes: 1}} {
		if err := db.CreateHNSWIndex(ctx, "invalid", "items", "v", o); err == nil {
			t.Fatalf("invalid options accepted %+v", o)
		}
	}
	if _, err := db.Exec(ctx, "CREATE INDEX bad ON items(v) USING hnsw WITH(unknown=2)"); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("unknown SQL option %v", err)
	}
	if _, err := db.Exec(ctx, "CREATE INDEX bad ON items(v) USING ivfflat WITH(opclass='vector_l1_ops',lists=2)"); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("unsupported IVF L1 %v", err)
	}
}

func TestANNStorageConstructionQuotaAtomic(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),note TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2]','a'),(2,'[2,3]','b'),(3,'[3,4]','c')")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE items SET note='earlier' WHERE id=1")
	err = tx.CreateHNSWIndex(ctx, "over", "items", "v", sqlite.HNSWIndexOptions{MaxVisited: 1})
	if !errors.Is(err, ann.ErrLimit) {
		t.Fatalf("build quota %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT note FROM items WHERE id=1"); got[0][0] != "earlier" {
		t.Fatalf("earlier statement lost: %v", got)
	}
	if schema, err := db.Schema(ctx); err != nil || len(schema) != 1 {
		t.Fatalf("failed build published objects %v %v", schema, err)
	}
}

func TestANNStorageNormalSharedWALSnapshot(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ann.db")
	opts := sqlite.Options{WALShared: true, MaxTransactionPages: 20000}
	writer, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	execute(t, writer, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
	execute(t, writer, "INSERT INTO items VALUES(1,'[1,2]'),(2,'[2,3]')")
	if err = writer.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	reader, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	execute(t, writer, "UPDATE items SET v='[3,4]' WHERE id=1")
	if err = tx.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if err = writer.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = reader.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if err = writer.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestANNStorageEmptyTombstoneQuotaAndVectorBits(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2))")
	if err := db.CreateHNSWIndex(ctx, "knn", "items", "v", sqlite.HNSWIndexOptions{MaxTombstones: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "INSERT INTO items VALUES(1,NULL),(2,'[0,1]'),(3,'[2,3]')")
	before := extensionRead(t, db, "SELECT node_id FROM __gosqlite_ext_ann_live WHERE content_rowid=2")
	execute(t, db, "UPDATE items SET v='[-0,1]' WHERE id=2")
	after := extensionRead(t, db, "SELECT node_id FROM __gosqlite_ext_ann_live WHERE content_rowid=2")
	if reflect.DeepEqual(before, after) {
		t.Fatal("signed-zero bit update reused immutable node")
	}
	if _, err := db.Exec(ctx, "DELETE FROM items WHERE id=3"); !errors.Is(err, ann.ErrLimit) {
		t.Fatalf("tombstone quota: %v", err)
	}
	if err := db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "DELETE FROM items WHERE id=1") // NULL never consumes graph quota.
	if err := db.Reindex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "DELETE FROM items WHERE id=3")
	if err := db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "UPDATE items SET v=NULL WHERE id=2"); !errors.Is(err, ann.ErrLimit) {
		t.Fatalf("second tombstone accepted: %v", err)
	}
	if err := db.Reindex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "UPDATE items SET v=NULL WHERE id=2")
	if err := db.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
}

func TestANNStorageFailedReindexPreservesPriorStatement(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2))")
	execute(t, db, "CREATE TABLE marker(id INTEGER PRIMARY KEY,note TEXT)")
	execute(t, db, "INSERT INTO marker VALUES(1,'before')")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 48; i++ {
		txExecute(t, tx, "INSERT INTO items VALUES(?,?)", i, fmt.Sprintf("[%d,%d]", i, i%7))
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	before := extensionRead(t, db, "SELECT generation FROM __gosqlite_ext_catalog")
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{MaxTransactionPages: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE marker SET note='earlier' WHERE id=1")
	if err = tx.Reindex(ctx, "knn"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("expected bounded dirty-page failure: %v", err)
	}
	if err = tx.CheckExtensionIndex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT note FROM marker"); got[0][0] != "earlier" {
		t.Fatalf("lost prior statement %v", got)
	}
	if got := extensionRead(t, db, "SELECT generation FROM __gosqlite_ext_catalog"); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed reindex published generation %v => %v", before, got)
	}
}

func TestANNStorageAllMetricsMultipleIndexes(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
	execute(t, db, "INSERT INTO items VALUES(-9223372036854775808,'[1,2]'),(0,'[2,3]'),(9223372036854775807,'[-3,2]')")
	for i, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
		name := fmt.Sprintf("knn%d", i)
		if err := db.CreateHNSWIndex(ctx, name, "items", "v", sqlite.HNSWIndexOptions{Metric: metric}); err != nil {
			t.Fatal(err)
		}
	}
	execute(t, db, "UPDATE items SET v='[7,8]' WHERE id=0")
	for i := 0; i < 4; i++ {
		if err := db.CheckExtensionIndex(ctx, fmt.Sprintf("knn%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	execute(t, db, "CREATE TABLE wide(id INTEGER PRIMARY KEY,v VECTOR(2001))")
	if err := db.CreateHNSWIndex(ctx, "too_wide", "wide", "v"); !errors.Is(err, ann.ErrConfig) {
		t.Fatalf("ANN2000 dimension cap: %v", err)
	}
	for _, option := range []string{"m=0", "m=101", "ef_construction=0", "ef_construction=1001", "m=16,ef_construction=31", "m=4294967312", "ef_construction=4294967360", "max_visited=4294967297", "max_distance_evaluations=4294967297"} {
		if _, err := db.Exec(ctx, "CREATE INDEX bad ON items(v) USING hnsw WITH("+option+")"); err == nil {
			t.Fatalf("invalid SQL parameters accepted: %s", option)
		}
	}
}
