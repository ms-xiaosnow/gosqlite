package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestIVFStorageTrainedLifecycleAndReopen(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),note TEXT)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := -8; i < 72; i++ {
		txExecute(t, tx, "INSERT INTO items VALUES(?,?,?)", i, fmt.Sprintf("[%d,%d]", i/20*100+i%7, i%11), "old")
	}
	txExecute(t, tx, "INSERT INTO items VALUES(100,NULL,'null')")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateIVFFlatIndex(ctx, "ivf", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 4, TrainingSamples: 64}); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	state := extensionRead(t, db, "SELECT live_count,training_state FROM __gosqlite_ext_ann_state")
	if !reflect.DeepEqual(state, [][]any{{int64(80), "ready"}}) {
		t.Fatal(state)
	}
	centers := extensionRead(t, db, "SELECT list_id,centroid FROM __gosqlite_ext_ann_centroids ORDER BY list_id")
	if len(centers) != 4 {
		t.Fatalf("centroid count %d", len(centers))
	}
	if members := extensionRead(t, db, "SELECT content_rowid FROM __gosqlite_ext_ann_members"); len(members) != 80 {
		t.Fatalf("membership count %d", len(members))
	}
	oldMember := extensionRead(t, db, "SELECT id,list_id FROM __gosqlite_ext_ann_members WHERE content_rowid=1")
	execute(t, db, "UPDATE items SET note='business' WHERE id=1")
	if got := extensionRead(t, db, "SELECT id,list_id FROM __gosqlite_ext_ann_members WHERE content_rowid=1"); !reflect.DeepEqual(got, oldMember) {
		t.Fatalf("nonvector update replaced member %v => %v", oldMember, got)
	}
	execute(t, db, "UPDATE items SET v='[1000,1000]' WHERE id=1")
	execute(t, db, "UPDATE items SET id=-100 WHERE id=2")
	execute(t, db, "DELETE FROM items WHERE id=3")
	execute(t, db, "INSERT INTO items VALUES(3,'[-900,-900]','reused')")
	execute(t, db, "UPDATE items SET v='[11,12]' WHERE id=100")
	execute(t, db, "UPDATE items SET v=NULL WHERE id=4")
	if err = db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT list_id,centroid FROM __gosqlite_ext_ann_centroids ORDER BY list_id"); !reflect.DeepEqual(got, centers) {
		t.Fatal("DML retrained fixed centroids")
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "DELETE FROM items WHERE id=-100")
	if err = tx.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = db.Reindex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
}

func TestIVFStorageUntrainedNeedsExplicitReindex(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2))")
	if err := db.CreateIVFFlatIndex(ctx, "ivf", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 3, Metric: vector.NegativeIP}); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "INSERT INTO items VALUES(1,'[0,0]'),(2,'[1,0]'),(3,NULL)")
	if err := db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err := db.Reindex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	} // Only one trainable direction.
	got := extensionRead(t, db, "SELECT live_count,training_state FROM __gosqlite_ext_ann_state")
	if !reflect.DeepEqual(got, [][]any{{int64(2), "untrained"}}) {
		t.Fatalf("insufficient input trained %v", got)
	}
	execute(t, db, "INSERT INTO items VALUES(4,'[0,1]'),(5,'[-1,0]')")
	if got := extensionRead(t, db, "SELECT training_state FROM __gosqlite_ext_ann_state"); got[0][0] != "untrained" {
		t.Fatal("DML implicitly retrained")
	}
	if err := db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err := db.Reindex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT live_count,training_state FROM __gosqlite_ext_ann_state"); !reflect.DeepEqual(got, [][]any{{int64(4), "ready"}}) {
		t.Fatalf("reindex did not train: %v", got)
	}
	if zero := extensionRead(t, db, "SELECT list_id FROM __gosqlite_ext_ann_members WHERE content_rowid=1"); !reflect.DeepEqual(zero, [][]any{{int64(1)}}) {
		t.Fatalf("IP zero assignment %v", zero)
	}
	if err := db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
}

func TestIVFStorageBuildLimitsAndDMLAtomicity(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),note TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2]','a'),(2,'[2,3]','b'),(3,'[3,4]','c')")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE items SET note='earlier' WHERE id=1")
	if err = tx.CreateIVFFlatIndex(ctx, "bad", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 2, MaxTrainingWork: 1}); !errors.Is(err, ann.ErrLimit) {
		t.Fatalf("training work quota %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT note FROM items WHERE id=1"); got[0][0] != "earlier" {
		t.Fatalf("lost earlier statement %v", got)
	}
	if schema, err := db.Schema(ctx); err != nil || len(schema) != 1 {
		t.Fatalf("failed training published schema %v %v", schema, err)
	}
	if err = db.CreateIVFFlatIndex(ctx, "ivf", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 2, MaxNodes: 3}); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE items SET note='second' WHERE id=1")
	if _, err = tx.Exec(ctx, "INSERT INTO items VALUES(4,'[4,5]','d')"); !errors.Is(err, ann.ErrLimit) {
		t.Fatalf("eligible row limit %v", err)
	}
	if err = tx.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err = db.Reindex(cancelCtx, "ivf"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled reindex: %v", err)
	}
	if err = db.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
}

func TestIVFStorageMetricsZeroPolicyAndSQL(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),body TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2]','abc'),(2,'[3,4]','bcd'),(3,'[-2,1]','cde')")
	for i, metric := range []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP} {
		if err := db.CreateIVFFlatIndex(ctx, fmt.Sprintf("ivf%d", i), "items", "v", sqlite.IVFFlatIndexOptions{Lists: 2, Metric: metric}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.CreateGramsIndex(ctx, "grams", "items", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "UPDATE items SET v='[0,0]' WHERE id=1"); !errors.Is(err, sqlite.ErrConstraint) || !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatalf("cosine constraint %v", err)
	}
	for _, name := range []string{"ivf0", "ivf1", "ivf2", "grams"} {
		if err := db.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	execute(t, db, "CREATE INDEX ivf_sql ON items(v) USING ivfflat WITH(opclass='vector_ip_ops',lists=2,training_samples=8,training_iterations=3)")
	if err := db.CheckExtensionIndex(ctx, "ivf_sql"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateIVFFlatIndex(ctx, "bad", "items", "v", sqlite.IVFFlatIndexOptions{Metric: vector.L1}); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("IVF L1 accepted %v", err)
	}
	for _, option := range []string{"lists=0", "lists=32769", "lists=4294967297", "training_iterations=0", "training_samples=2,lists=3", "m=16", "opclass='vector_l1_ops'"} {
		if _, err := db.Exec(ctx, "CREATE INDEX bad ON items(v) USING ivfflat WITH("+option+")"); err == nil {
			t.Fatalf("invalid IVF option accepted %s", option)
		}
	}
}

func TestIVFStorageOldHNSWCanonicalSurvives(t *testing.T) {
	ctx := context.Background()
	original, err := os.ReadFile(filepath.Join("..", "testdata", "ann", "hnsw-v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "copy.db")
	if err = os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.CheckExtensionIndex(ctx, "items_hnsw"); err != nil {
		t.Fatal(err)
	}
	before := extensionRead(t, db, "SELECT config FROM __gosqlite_ext_catalog WHERE name='items_hnsw'")
	if err = db.CreateIVFFlatIndex(ctx, "items_ivf", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 4}); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "UPDATE items SET embedding='[7,8,9]' WHERE id=3")
	execute(t, db, "DELETE FROM items WHERE id=4")
	execute(t, db, "INSERT INTO items VALUES(1000,'[9,8,7]',1)")
	for _, name := range []string{"items_hnsw", "items_ivf"} {
		if err = db.Reindex(ctx, name); err != nil {
			t.Fatal(err)
		}
		if err = db.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	if after := extensionRead(t, db, "SELECT config FROM __gosqlite_ext_catalog WHERE name='items_hnsw'"); !reflect.DeepEqual(before, after) {
		t.Fatalf("old HNSW canonical bytes changed %v => %v", before, after)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"items_hnsw", "items_ivf"} {
		if err = db.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(filepath.Join("..", "testdata", "ann", "hnsw-v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, after) {
		t.Fatal("test changed original compatibility fixture")
	}
}

func TestIVFStorageEmptyWideMemoryAndReindexUndo(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE wide(id INTEGER PRIMARY KEY,v VECTOR(2000))")
	if err := db.CreateIVFFlatIndex(ctx, "empty", "wide", "v", sqlite.IVFFlatIndexOptions{Lists: 32768, TrainingSamples: 32768, MaxMemoryBytes: 1024}); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckExtensionIndex(ctx, "empty"); err != nil {
		t.Fatal(err)
	}
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
	if err = db.CreateIVFFlatIndex(ctx, "ivf", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 4}); err != nil {
		t.Fatal(err)
	}
	centers := extensionRead(t, db, "SELECT list_id,centroid FROM __gosqlite_ext_ann_centroids ORDER BY list_id")
	if err = db.Reindex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if after := extensionRead(t, db, "SELECT list_id,centroid FROM __gosqlite_ext_ann_centroids ORDER BY list_id"); !reflect.DeepEqual(after, centers) {
		t.Fatal("same snapshot REINDEX changed deterministic centroids")
	}
	before := extensionRead(t, db, "SELECT generation FROM __gosqlite_ext_catalog ORDER BY id")
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
	if err = tx.Reindex(ctx, "ivf"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("expected bounded REINDEX failure: %v", err)
	}
	if err = tx.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT note FROM marker"); got[0][0] != "earlier" {
		t.Fatalf("prior statement lost %v", got)
	}
	if after := extensionRead(t, db, "SELECT generation FROM __gosqlite_ext_catalog ORDER BY id"); !reflect.DeepEqual(before, after) {
		t.Fatalf("failed REINDEX advanced generation %v => %v", before, after)
	}
}

func TestIVFStorageSharedWALSnapshot(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ivf.db")
	options := sqlite.Options{WALShared: true}
	writer, err := sqlite.Open(path, options)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	execute(t, writer, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
	execute(t, writer, "INSERT INTO items VALUES(1,'[1,2]'),(2,'[2,3]'),(3,'[10,12]'),(4,'[12,13]')")
	if err = writer.CreateIVFFlatIndex(ctx, "ivf", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 2}); err != nil {
		t.Fatal(err)
	}
	reader, err := sqlite.Open(path, options)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	old, err := tx.VectorSearch(ctx, "items", "v", "[1,2]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "ivf", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	execute(t, writer, "UPDATE items SET v='[100,200]' WHERE id=1")
	if err = writer.Reindex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = tx.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	same, err := tx.VectorSearch(ctx, "items", "v", "[1,2]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "ivf", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old.Hits, same.Hits) || old.Plan.ANNGeneration != same.Plan.ANNGeneration {
		t.Fatalf("reader snapshot changed %+v => %+v", old, same)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = reader.CheckExtensionIndex(ctx, "ivf"); err != nil {
		t.Fatal(err)
	}
	if err = writer.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
}
