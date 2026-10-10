package sqlite_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These independent review regressions use ordinary valid data and cooperative
// statement/transaction rollback. They contain no physical fault injection.
func TestM4ReviewNativeSchemaCookiesAcrossIndexes(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT,body TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'甲乙丙','rare'),(2,'ordinary','common'),(3,'another','common')")
	if err := db.CreateGramsIndex(ctx, "title_grams", "docs", []string{"title"}); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "CREATE INDEX body_grams ON docs(body) USING grams13")
	if err := db.CreateGramsIndex(ctx, "both_grams", "docs", []string{"title", "body"}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "CREATE TABLE unrelated(id INTEGER PRIMARY KEY,note TEXT)")
	txExecute(t, tx, "CREATE INDEX ordinary_title ON docs(title)")
	for _, name := range []string{"title_grams", "body_grams", "both_grams"} {
		if err := tx.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatalf("%s after native DDL: %v", name, err)
		}
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"title_grams", "body_grams", "both_grams"} {
		if err := db.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatalf("%s after DDL rollback: %v", name, err)
		}
	}
	execute(t, db, "CREATE TABLE committed(id INTEGER PRIMARY KEY,note TEXT)")
	execute(t, db, "UPDATE docs SET id=-10,title='changed甲乙丙',body='changed rare' WHERE id=1")
	for _, name := range []string{"title_grams", "body_grams", "both_grams"} {
		if err := db.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatalf("%s after DDL and rowid change: %v", name, err)
		}
	}
	p, _, _ := gramCompare(t, db, "title LIKE '%甲乙丙%'")
	if p.Access != "SEARCH" {
		t.Fatalf("clean peer indexes unexpectedly invalidated: %+v", p)
	}
}

func TestM4ReviewFailedReconfigurePreservesTransaction(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'Straße é'),(2,'plain'),(3,'third')")
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"title"}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "INSERT INTO docs VALUES(4,'kept')")
	metadata := "SELECT config,config_hash,generation,needs_reindex,source_rows,schema_cookie FROM __gosqlite_ext_catalog ORDER BY id"
	before, _, _ := gramResults(t, tx, metadata)
	tooSmall := grams13.DefaultMatchConfig()
	tooSmall.CaseFold = grams13.CaseFoldUnicode
	tooSmall.Limits.MaxInputBytes = 1
	if err = tx.Reindex(ctx, "docs_grams", sqlite.GramsIndexOptions{MatchConfig: tooSmall}); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("expected rebuild quota error, got %v", err)
	}
	after, _, _ := gramResults(t, tx, metadata)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("failed rebuild published partial metadata: before=%#v after=%#v", before, after)
	}
	if err = tx.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	got, _, _ := gramResults(t, tx, "SELECT id FROM docs WHERE title LIKE '%kept%'")
	if !reflect.DeepEqual(got, [][]any{{int64(4)}}) {
		t.Fatalf("earlier transaction statement lost: %#v", got)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = tx.Reindex(canceled, "docs_grams"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cooperative cancellation, got %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
}

func TestM4ReviewIndexedMultirowStatementQuotaRollback(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'needle'),(2,'other'),(3,'third')")
	config := grams13.DefaultMatchConfig()
	config.Limits.MaxInputBytes = 12
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"title"}, sqlite.GramsIndexOptions{MatchConfig: config}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "INSERT INTO docs VALUES(4,'kept')")
	if _, err = tx.Exec(ctx, "INSERT INTO docs VALUES(5,'partial'),(6,'this valid text exceeds the configured quota')"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("expected atomic indexed-field quota error: %v", err)
	}
	if err = tx.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	got, _, _ := gramResults(t, tx, "SELECT id,title FROM docs ORDER BY id")
	want := [][]any{{int64(1), "needle"}, {int64(2), "other"}, {int64(3), "third"}, {int64(4), "kept"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statement rollback changed prior rows: %#v", got)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	p, _, _ := gramCompare(t, db, "title LIKE '%partial%'")
	if p.Access != "SEARCH" {
		t.Fatalf("empty but valid posting range should remain usable: %+v", p)
	}
	if err = db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
}

func TestM4ReviewIndexPublicationMetadataBudget(t *testing.T) {
	db, _ := writable(t, sqlite.Options{MaxSortBytes: 512})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)")
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"title"}); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("empty-source build must enforce published catalog budget: %v", err)
	}
	entries, err := db.Schema(ctx)
	if err != nil || len(entries) != 1 || entries[0].Name != "docs" {
		t.Fatalf("quota failure published partial extension schema: %#v %v", entries, err)
	}
	execute(t, db, "INSERT INTO docs VALUES(1,'still writable')")
}

func TestM4ReviewIndexNameAcceptanceMatchesLoader(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	ctx := context.Background()
	execute(t, db, `CREATE TABLE docs(id INTEGER PRIMARY KEY,"" TEXT,title TEXT)`)
	for _, tc := range []struct {
		name    string
		columns []string
	}{{"", []string{"title"}}, {"empty_field", []string{""}}} {
		if err := db.CreateGramsIndex(ctx, tc.name, "docs", tc.columns); err == nil {
			t.Fatalf("accepted metadata rejected by its own loader: name=%q fields=%q", tc.name, tc.columns)
		}
	}
	entries, err := db.Schema(ctx)
	if err != nil || len(entries) != 1 || entries[0].Name != "docs" {
		t.Fatalf("rejected names published partial extension schema: %#v %v", entries, err)
	}
	execute(t, db, `INSERT INTO docs VALUES(1,'empty-named column','normal')`)
	if err := db.CreateGramsIndex(ctx, "valid_grams", "docs", []string{"title"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckExtensionIndex(ctx, "valid_grams"); err != nil {
		t.Fatal(err)
	}
}
