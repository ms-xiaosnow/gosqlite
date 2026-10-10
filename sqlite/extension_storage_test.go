package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func extensionRead(t *testing.T, db *sqlite.DB, q string, args ...any) [][]any {
	t.Helper()
	rows, err := db.Query(context.Background(), q, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result [][]any
	for rows.Next() {
		result = append(result, rows.Values())
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestPersistentGramsIncrementalDeltaIdentity(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, title TEXT, body TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'title','abc'),(2,'other',NULL),(3,'emoji','👩‍💻 数据库')")
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"title", "body"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	q := "SELECT id FROM __gosqlite_ext_postings WHERE field_id=2 AND gram='ab' AND content_rowid=1 ORDER BY id"
	before := extensionRead(t, db, q)
	if len(before) != 2 {
		t.Fatalf("expected two namespaces: %v", before)
	}
	execute(t, db, "UPDATE docs SET body='abd' WHERE id=1")
	after := extensionRead(t, db, q)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unchanged keys were replaced: %v -> %v", before, after)
	}
	if err := db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "UPDATE docs SET id=-100 WHERE id=1")
	execute(t, db, "DELETE FROM docs WHERE id=2")
	execute(t, db, "INSERT INTO docs VALUES(42,'new','abde')")
	if err := db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE docs SET body='changed' WHERE id=-100")
	if err := tx.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
}
func TestPersistentGramsShadowProtectionAndSQLConfig(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'Straße')")
	for _, name := range []string{"", "bad\x00name", string([]byte{0xff})} {
		if err := db.CreateGramsIndex(ctx, name, "docs", []string{"body"}); err == nil {
			t.Fatalf("accepted invalid index name %q", name)
		}
	}
	schemas, err := db.Schema(ctx)
	if err != nil || len(schemas) != 1 {
		t.Fatalf("invalid-name request changed schema: %v %v", schemas, err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE ""(x)`); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("empty schema name accepted: %v", err)
	}
	execute(t, db, `CREATE TABLE empty_field("" TEXT)`)
	if err := db.CreateGramsIndex(ctx, "empty_column_index", "empty_field", []string{""}); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("empty indexed field accepted: %v", err)
	}
	schemas, err = db.Schema(ctx)
	if err != nil || len(schemas) != 2 {
		t.Fatalf("empty-field request changed schema: %v %v", schemas, err)
	}
	execute(t, db, "CREATE INDEX grams ON docs(body) USING grams13 WITH (min_gram=1,max_gram=3,case_fold='unicode-full-default')")
	for _, q := range []string{
		"INSERT INTO __gosqlite_ext_catalog VALUES(NULL,'x','docs','x','grams13',1,1,'x','x',1,0,0,0)",
		"DELETE FROM __gosqlite_ext_postings", "UPDATE __gosqlite_ext_frequency SET documents=0",
		"CREATE INDEX bad ON __gosqlite_ext_fields(column_name)",
		"CREATE TABLE __gosqlite_ext_impersonator(x)", "CREATE TABLE grams(x)",
		"CREATE INDEX invalid ON docs(body) USING grams13 WITH (max_gram=2)",
		"CREATE INDEX unknown ON docs(body) USING magic",
	} {
		if _, err := db.Exec(ctx, q); err == nil {
			t.Fatalf("accepted protected/unsupported SQL: %s", q)
		}
	}
	if err := db.CheckExtensionIndex(ctx, "grams"); err != nil {
		t.Fatal(err)
	}
	config := grams13.DefaultMatchConfig()
	config.Punctuation = grams13.PunctuationRemove
	if err := db.Reindex(ctx, "grams", sqlite.GramsIndexOptions{MatchConfig: config}); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckExtensionIndex(ctx, "grams"); err != nil {
		t.Fatal(err)
	}
}

// This fixture is an ordinary application-supplied immutable implementation;
// instrumentation is atomic and does not affect derived keys.
type wholeMethod struct{ calls *atomic.Int64 }

func (wholeMethod) Name() string { return "whole_text" }
func (wholeMethod) Version() int { return 1 }
func (m wholeMethod) Configure(c []byte) (registry.Index, error) {
	if string(c) != "whole-v1" {
		return nil, errors.New("bad config")
	}
	return wholeIndex{m.calls}, nil
}

type wholeIndex struct{ calls *atomic.Int64 }

func (wholeIndex) Canonical() []byte { return []byte("whole-v1") }
func (i wholeIndex) Keys(ctx context.Context, value string) ([]registry.Key, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	i.calls.Add(1)
	return []registry.Key{{Namespace: "WHOLE_V1", Length: 1, Text: value}}, nil
}
func TestPersistentRegisteredMethodIsRealAndConnectionLocal(t *testing.T) {
	ctx := context.Background()
	var calls atomic.Int64
	r, err := registry.New(grams13.Method(), wholeMethod{&calls})
	if err != nil {
		t.Fatal(err)
	}
	db, path := writable(t, sqlite.Options{IndexMethods: r})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'abc'),(2,'xyz')")
	if err := db.CreateMethodIndex(ctx, "whole", "docs", []string{"body"}, "whole_text", 1, []byte("whole-v1")); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("custom handler calls=%d", calls.Load())
	}
	execute(t, db, "UPDATE docs SET body='new' WHERE id=1")
	execute(t, db, "DELETE FROM docs WHERE id=2")
	execute(t, db, "INSERT INTO docs VALUES(3,'third')")
	if err := db.CheckExtensionIndex(ctx, "whole"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	unknown, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unknown.Exec(ctx, "INSERT INTO docs VALUES(9,'no')"); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("unknown method not refused: %v", err)
	}
	if _, err := unknown.Query(ctx, "SELECT id FROM docs WHERE body LIKE '%n%'"); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("unknown method query not failclosed: %v", err)
	}
	unknown.Close()
	reopened, err := sqlite.Open(path, sqlite.Options{IndexMethods: r})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.CheckExtensionIndex(ctx, "whole"); err != nil {
		t.Fatal(err)
	}
}
func TestPersistentGramsBuildBudgetAndCancelPreserveSource(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "limit.db")
	db, err := sqlite.Open(path, sqlite.Options{MaxSortBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)")
	text := ""
	for n := 0; n < 100; n++ {
		text += fmt.Sprintf("%c", rune(0x4e00+n))
	}
	execute(t, db, "INSERT INTO docs VALUES(1,?)", text)
	if err := db.CreateGramsIndex(ctx, "too_large", "docs", []string{"body"}); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("budget: %v", err)
	}
	schemas, err := db.Schema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) != 1 {
		t.Fatalf("failed build left schema: %+v", schemas)
	}
	got := extensionRead(t, db, "SELECT body FROM docs")
	if len(got) != 1 || got[0][0] != text {
		t.Fatalf("source changed: %v", got)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := db.CreateGramsIndex(cancelled, "cancelled", "docs", []string{"body"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	execute(t, db, "UPDATE docs SET body='small' WHERE id=1")
	if err := db.CreateGramsIndex(ctx, "small", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckExtensionIndex(ctx, "small"); err != nil {
		t.Fatal(err)
	}
}

type cancelOnTextKey struct{}
type cancellableGramsMethod struct{}

func (cancellableGramsMethod) Name() string { return "cancellable_grams" }
func (cancellableGramsMethod) Version() int { return 1 }
func (cancellableGramsMethod) Configure(c []byte) (registry.Index, error) {
	inner, err := grams13.Method().Configure(c)
	if err != nil {
		return nil, err
	}
	return cancellableGramsIndex{inner}, nil
}

type cancellableGramsIndex struct{ inner registry.Index }

func (i cancellableGramsIndex) Canonical() []byte { return i.inner.Canonical() }
func (i cancellableGramsIndex) Keys(ctx context.Context, s string) ([]registry.Key, error) {
	if s == "stop" {
		if cancel, ok := ctx.Value(cancelOnTextKey{}).(context.CancelFunc); ok {
			cancel()
		}
	}
	return i.inner.Keys(ctx, s)
}
func TestPersistentMethodCancelledRebuildRestoresPriorStatement(t *testing.T) {
	ctx := context.Background()
	methods, err := registry.New(grams13.Method(), cancellableGramsMethod{})
	if err != nil {
		t.Fatal(err)
	}
	db, _ := writable(t, sqlite.Options{IndexMethods: methods})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'first'),(2,'stop'),(3,'last')")
	config, err := grams13.ConfigBytes(grams13.DefaultMatchConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMethodIndex(ctx, "custom", "docs", []string{"body"}, "cancellable_grams", 1, config); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE docs SET body='prior retained' WHERE id=1")
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cancelCtx = context.WithValue(cancelCtx, cancelOnTextKey{}, context.CancelFunc(cancel))
	if err := tx.Reindex(cancelCtx, "custom"); !errors.Is(err, context.Canceled) {
		t.Fatalf("mid-rebuild cancellation=%v", err)
	}
	if err := tx.CheckExtensionIndex(ctx, "custom"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT body FROM docs WHERE id=1"); len(got) != 1 || got[0][0] != "prior retained" {
		t.Fatalf("prior statement lost: %v", got)
	}
	if err := db.CheckExtensionIndex(ctx, "custom"); err != nil {
		t.Fatal(err)
	}
}
func TestPersistentGramsDirtyPageBudgetBuildRollback(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{PageSize: 512, MaxTransactionPages: 12})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)")
	for i := 1; i <= 50; i++ {
		execute(t, db, "INSERT INTO docs VALUES(?,?)", i, fmt.Sprintf("database-%03d-中文搜索内容abcdef", i))
	}
	if err := db.CreateGramsIndex(ctx, "too_many_pages", "docs", []string{"body"}); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("dirty-page build quota=%v", err)
	}
	schema, err := db.Schema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema) != 1 {
		t.Fatalf("failed build left %d schema entries", len(schema))
	}
	if got := extensionRead(t, db, "SELECT id FROM docs"); len(got) != 50 {
		t.Fatalf("source rows lost: %d", len(got))
	}
	execute(t, db, "INSERT INTO docs VALUES(51,'still writable')")
}

func TestPersistentEmptySourceMetadataBudgetIsAtomic(t *testing.T) {
	ctx := context.Background()
	tiny, _ := writable(t, sqlite.Options{MaxSortBytes: 32})
	execute(t, tiny, "CREATE TABLE docs(body TEXT)")
	if err := tiny.CreateGramsIndex(ctx, "too_large_metadata", "docs", []string{"body"}); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("empty-source metadata quota=%v", err)
	}
	schema, err := tiny.Schema(ctx)
	if err != nil || len(schema) != 1 {
		t.Fatalf("failed metadata creation published schema: %v %v", schema, err)
	}
	db, _ := writable(t, sqlite.Options{MaxSortBytes: 4096})
	execute(t, db, "CREATE TABLE docs(body TEXT)")
	added := 0
	for n := 0; n < 10; n++ {
		name := fmt.Sprintf("index%d", n)
		err := db.CreateGramsIndex(ctx, name, "docs", []string{"body"})
		if errors.Is(err, sqlite.ErrMemoryLimit) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		added++
	}
	if added == 0 || added == 10 {
		t.Fatalf("unexpected metadata capacity %d", added)
	}
	for n := 0; n < added; n++ {
		if err := db.CheckExtensionIndex(ctx, fmt.Sprintf("index%d", n)); err != nil {
			t.Fatal(err)
		}
	}
	schema, err = db.Schema(ctx)
	if err != nil || len(schema) != 8+3*added {
		t.Fatalf("failed cumulative metadata creation published schema: count=%d added=%d err=%v", len(schema), added, err)
	}
}
