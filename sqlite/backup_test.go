//go:build linux && manual_backup_publication

package sqlite_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/backupmanual"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These explicit manual tests retain every intact disposable fixture. Their
// original SQL and assertions remain the ordinary Backup acceptance boundary.
func backupOpen(t *testing.T, scope *backupmanual.Scope, path string, opts sqlite.Options) *sqlite.DB {
	t.Helper()
	scope.Before("open")
	db, err := sqlite.Open(path, opts)
	scope.RetainErrors(err)
	if db != nil {
		scope.OwnDB(db)
	}
	scope.Observe("open", map[string]any{"path": path, "options": opts, "database_returned": db != nil}, err)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
func backupExec(t *testing.T, scope *backupmanual.Scope, db *sqlite.DB, q string, args ...any) {
	t.Helper()
	scope.Before("exec")
	result, err := db.Exec(context.Background(), q, args...)
	scope.Observe("exec", map[string]any{"input": backupmanual.SQL(q, args...), "result": fmt.Sprintf("%#v", result)}, err)
	if err != nil {
		t.Fatal(q, err)
	}
}
func backupBytes(t *testing.T, scope *backupmanual.Scope, path string) []byte {
	t.Helper()
	scope.Before("read bytes")
	p, err := fileview.ReadFile(path)
	scope.Observe("read bytes", map[string]any{"path": path, "bytes": len(p), "sha256": fmt.Sprintf("%x", sha256.Sum256(p))}, err)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func backupQuery(t *testing.T, scope *backupmanual.Scope, db *sqlite.DB, q string) [][]any {
	t.Helper()
	scope.Before("query")
	r, err := db.Query(context.Background(), q)
	scope.RetainErrors(err)
	if r != nil {
		scope.OwnRows(db, r)
	}
	scope.Observe("query", map[string]any{"input": backupmanual.SQL(q), "rows_returned": r != nil}, err)
	if err != nil {
		t.Fatal(q, err)
	}
	var out [][]any
	for {
		scope.Before("rows next")
		next := r.Next()
		scope.Record("rows next", next)
		if !next {
			break
		}
		values := r.Values()
		scope.Record("rows values", values)
		out = append(out, append([]any(nil), values...))
	}
	// Next(false) can already have attempted internal retirement. Preserve the
	// original Err and Close success requirements without retrying on Err.
	err = r.Err()
	scope.Observe("rows exhausted", out, err)
	if err != nil {
		t.Fatal(err)
	}
	if err := scope.CloseRows(r); err != nil {
		t.Fatal(err)
	}
	return out
}

// Plan builders only construct fixed data. In particular they never parse SQL,
// open a database, discover a path, or run a collector or an oracle.
func backupPlanCall(operation, path string, input, expected any) any {
	return map[string]any{"operation": operation, "path": path, "input": input, "expected": expected}
}
func backupQueryPlan(path, q string, expected any) any {
	return backupPlanCall("Query; Next/Values to exhaustion; Err; CloseRows once only if Err is nil", path, backupmanual.SQL(q), expected)
}
func backupRoundTripPlan(size uint32, path, destination, sourceDir string, queries []string) any {
	calls := []any{
		backupPlanCall("Open", path, sqlite.Options{PageSize: size}, "error == nil"),
		backupPlanCall("Exec", path, backupmanual.SQL("CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,payload BLOB,v VECTOR(3),optional TEXT)"), "error == nil"),
		backupPlanCall("Exec", path, backupmanual.SQL("CREATE INDEX docs_body ON docs(body)"), "error == nil"),
	}
	for i := 1; i <= 12; i++ {
		calls = append(calls, backupPlanCall("Exec", path, backupmanual.SQL("INSERT INTO docs VALUES(?,?,?,?,?)", i, fmt.Sprintf("数据库 カタカナ 한글 😀 e\u0301 %d", i), bytes.Repeat([]byte{0, 1, 255, byte(i)}, 500), fmt.Sprintf("[%d,1,2]", i), nil), "error == nil"))
	}
	calls = append(calls, backupPlanCall("Exec", path, backupmanual.SQL("DELETE FROM docs WHERE id>8"), "error == nil"))
	for _, q := range []string{
		"CREATE INDEX grams ON docs(body) USING grams13",
		"CREATE INDEX hn ON docs(v) USING hnsw WITH(opclass='vector_l2_ops',m=4,ef_construction=16)",
		"CREATE INDEX iv ON docs(v) USING ivfflat WITH(opclass='vector_l2_ops',lists=2,training_samples=8,training_iterations=2)",
	} {
		calls = append(calls, backupPlanCall("Exec", path, backupmanual.SQL(q), "error == nil"))
	}
	calls = append(calls,
		backupPlanCall("CloseDB", path, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("ReadFile", path, nil, "error == nil; retain every before byte"),
		backupPlanCall("Open", path, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupPlanCall("Schema", path, nil, "error == nil; retain complete wantSchema"),
		backupPlanCall("Check", path, nil, "error == nil; retain complete wantCheck"),
	)
	for _, q := range queries {
		calls = append(calls, backupQueryPlan(path, q, "error == nil; retain every result value as want"))
	}
	calls = append(calls,
		backupPlanCall("DB.Backup; source.Info", destination, map[string]any{"source": path}, map[string]any{"error": nil, "info": "== source.Info()", "PageSize": size, "ReadVersion": 1}),
		backupPlanCall("CloseDB", path, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("ReadFile", path, nil, "error == nil; bytes.Equal(before, source bytes)"),
		backupPlanCall("ReadFile", destination, nil, "error == nil; bytes.Equal(before, destination bytes)"),
		backupPlanCall("ReadDir", sourceDir, nil, "error == nil; len(entries) == 1"),
		backupPlanCall("Open", destination, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupPlanCall("Schema", destination, nil, "error == nil; reflect.DeepEqual(gotSchema, wantSchema)"),
		backupPlanCall("Check", destination, nil, "error == nil; gotCheck == wantCheck"),
	)
	for _, q := range queries {
		calls = append(calls, backupQueryPlan(destination, q, "error == nil; reflect.DeepEqual(got, matching source want)"))
	}
	calls = append(calls, backupPlanCall("CloseDB", destination, nil, "error == nil; one explicit end attempt"))
	return map[string]any{"case": "ordinary-round-trip", "page_size": size, "calls": calls}
}

func TestPlaintextBackupOrdinaryRoundTrip(t *testing.T) {
	run := backupmanual.ForTest(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		if !t.Run(fmt.Sprint(size), func(t *testing.T) {
			scope := run.Case(t, fmt.Sprint(size))
			sourceDir := scope.Dir("source")
			path := filepath.Join(sourceDir, "source.db")
			destination := filepath.Join(scope.Dir("destination"), "snapshot.db")
			queries := []string{"SELECT id,body,payload,v,optional FROM docs ORDER BY id", "SELECT id FROM docs WHERE body LIKE '%数%' ORDER BY id", "SELECT id FROM docs WHERE body LIKE '%数据%' ORDER BY id", "SELECT id FROM docs WHERE body LIKE '%数据库%' ORDER BY id", "SELECT id FROM docs ORDER BY v <-> '[3,1,2]' LIMIT 4"}
			scope.SetPlan(backupRoundTripPlan(size, path, destination, sourceDir, queries))
			db := backupOpen(t, scope, path, sqlite.Options{PageSize: size})
			backupExec(t, scope, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,payload BLOB,v VECTOR(3),optional TEXT)")
			backupExec(t, scope, db, "CREATE INDEX docs_body ON docs(body)")
			for i := 1; i <= 12; i++ {
				backupExec(t, scope, db, "INSERT INTO docs VALUES(?,?,?,?,?)", i, fmt.Sprintf("数据库 カタカナ 한글 😀 e\u0301 %d", i), bytes.Repeat([]byte{0, 1, 255, byte(i)}, 500), fmt.Sprintf("[%d,1,2]", i), nil)
			}
			backupExec(t, scope, db, "DELETE FROM docs WHERE id>8")
			for _, q := range []string{
				"CREATE INDEX grams ON docs(body) USING grams13",
				"CREATE INDEX hn ON docs(v) USING hnsw WITH(opclass='vector_l2_ops',m=4,ef_construction=16)",
				"CREATE INDEX iv ON docs(v) USING ivfflat WITH(opclass='vector_l2_ops',lists=2,training_samples=8,training_iterations=2)",
			} {
				backupExec(t, scope, db, q)
			}
			if err := scope.CloseDB(db); err != nil {
				t.Fatal(err)
			}
			before := backupBytes(t, scope, path)
			source := backupOpen(t, scope, path, sqlite.Options{ReadOnly: true, NoCreate: true})
			scope.Before("source schema")
			wantSchema, err := source.Schema(ctx)
			scope.Observe("source schema", wantSchema, err)
			if err != nil {
				t.Fatal(err)
			}
			scope.Before("source check")
			wantCheck, err := source.Check(ctx)
			scope.Observe("source check", wantCheck, err)
			if err != nil {
				t.Fatal(err)
			}
			want := make([][][]any, len(queries))
			for i, q := range queries {
				want[i] = backupQuery(t, scope, source, q)
			}
			scope.Before("source backup")
			info, err := source.Backup(ctx, destination)
			scope.Observe("source backup", info, err)
			sourceInfo := source.Info()
			scope.Record("source info", sourceInfo)
			if err != nil || info != sourceInfo || info.PageSize != size || info.ReadVersion != 1 {
				t.Fatal(info, err)
			}
			if err := scope.CloseDB(source); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, backupBytes(t, scope, path)) || !bytes.Equal(before, backupBytes(t, scope, destination)) {
				t.Fatal("backup did not preserve every source byte")
			}
			scope.Before("source directory")
			entries, err := os.ReadDir(sourceDir)
			entryNames := make([]string, len(entries))
			for i, entry := range entries {
				entryNames[i] = entry.Name()
			}
			scope.Observe("source directory", entryNames, err)
			if err != nil || len(entries) != 1 {
				t.Fatal("source family changed", entries, err)
			}
			copy := backupOpen(t, scope, destination, sqlite.Options{ReadOnly: true, NoCreate: true})
			scope.Before("copy schema")
			gotSchema, err := copy.Schema(ctx)
			scope.Observe("copy schema", gotSchema, err)
			if err != nil || !reflect.DeepEqual(gotSchema, wantSchema) {
				t.Fatal("schema mismatch", err)
			}
			scope.Before("copy check")
			gotCheck, err := copy.Check(ctx)
			scope.Observe("copy check", gotCheck, err)
			if err != nil || gotCheck != wantCheck {
				t.Fatal("copied indexes/ownership differ", gotCheck, wantCheck, err)
			}
			for i, q := range queries {
				if got := backupQuery(t, scope, copy, q); !reflect.DeepEqual(got, want[i]) {
					t.Fatal("query mismatch", q)
				}
			}
			if err := scope.CloseDB(copy); err != nil {
				t.Fatal(err)
			}
			scope.FinishSuccess()
		}) {
			return
		}
	}
	run.FinishSuccess()
}
func TestPlaintextBackupExistingDestinationPreflight(t *testing.T) {
	run := backupmanual.ForTest(t)
	ctx := context.Background()
	scope := run.Case(t, "existing-destination")
	path, dest := filepath.Join(scope.Dir("source"), "source.db"), filepath.Join(scope.Dir("destination"), "existing.db")
	scope.SetPlan([]any{
		backupPlanCall("Open", path, sqlite.Options{}, "error == nil"),
		backupPlanCall("Exec", path, backupmanual.SQL("CREATE TABLE t(id INTEGER PRIMARY KEY)"), "error == nil"),
		backupPlanCall("Exec", path, backupmanual.SQL("INSERT INTO t VALUES(1)"), "error == nil"),
		backupPlanCall("Open", dest, sqlite.Options{}, "error == nil"),
		backupPlanCall("Exec", dest, backupmanual.SQL("CREATE TABLE preserve(id INTEGER PRIMARY KEY)"), "error == nil"),
		backupPlanCall("CloseDB", dest, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("ReadFile", dest, nil, "error == nil; retain every before byte"),
		backupPlanCall("DB.Backup", dest, map[string]any{"source": path}, "errors.Is(err, os.ErrExist); !errors.Is(err, sqlite.ErrCommitUncertain); info == sqlite.FileInfo{}; only known clean refusal; no pending/poison/unknown ownership"),
		backupPlanCall("ReadFile", dest, nil, "error == nil; bytes.Equal(before, every target byte)"),
		backupPlanCall("CloseDB", path, nil, "error == nil; one explicit end attempt"),
	})
	source := backupOpen(t, scope, path, sqlite.Options{})
	backupExec(t, scope, source, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	backupExec(t, scope, source, "INSERT INTO t VALUES(1)")
	target := backupOpen(t, scope, dest, sqlite.Options{})
	backupExec(t, scope, target, "CREATE TABLE preserve(id INTEGER PRIMARY KEY)")
	if err := scope.CloseDB(target); err != nil {
		t.Fatal(err)
	}
	before := backupBytes(t, scope, dest)
	scope.Before("existing destination backup")
	info, err := source.Backup(ctx, dest)
	scope.RetainErrors(err)
	scope.Record("existing destination backup info", info)
	scope.Record("existing destination backup error", err)
	if !errors.Is(err, os.ErrExist) || errors.Is(err, sqlite.ErrCommitUncertain) || info != (sqlite.FileInfo{}) {
		t.Fatal(info, err)
	}
	if !scope.AcceptExistingRefusal(err) {
		t.Fatal("existing destination refusal retained unknown ownership")
	}
	if !bytes.Equal(before, backupBytes(t, scope, dest)) {
		t.Fatal("existing target changed")
	}
	if err := scope.CloseDB(source); err != nil {
		t.Fatal(err)
	}
	scope.FinishSuccess()
	run.FinishSuccess()
}
func TestPlaintextBackupPinnedReadOnlyAndConcurrentReaders(t *testing.T) {
	run := backupmanual.ForTest(t)
	ctx := context.Background()
	scope := run.Case(t, "pinned-concurrent-readers")
	path := filepath.Join(scope.Dir("source"), "source.db")
	dest1, dest2 := filepath.Join(scope.Dir("tx-target"), "tx.db"), filepath.Join(scope.Dir("peer-target"), "peer.db")
	scope.SetPlan([]any{
		backupPlanCall("Open", path, sqlite.Options{}, "error == nil"),
		backupPlanCall("Exec", path, backupmanual.SQL("CREATE TABLE t(id INTEGER PRIMARY KEY,body TEXT)"), "error == nil"),
		backupPlanCall("Exec", path, backupmanual.SQL("INSERT INTO t VALUES(1,'pinned 数据库')"), "error == nil"),
		backupPlanCall("Begin", path, sqlite.TxOptions{ReadOnly: true}, "error == nil"),
		backupPlanCall("Tx.Check", path, nil, "error == nil; pin snapshot before backup"),
		backupPlanCall("Open independent peer", path, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupPlanCall("concurrent Tx.Backup and peer DB.Backup; WaitGroup.Wait", path, []string{dest1, dest2}, "ea == nil; eb == nil; a == b; db.Autocommit() == false"),
		backupPlanCall("Tx.Check", path, nil, "error == nil; transaction snapshot retained after backup"),
		backupPlanCall("CommitTx", path, nil, "error == nil; one end attempt; no fallback Rollback"),
		backupPlanCall("CloseDB peer", path, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("Open", dest1, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupQueryPlan(dest1, "SELECT body FROM t", "len(got) == 1; strings.Contains(got[0][0].(string), 数据库)"),
		backupPlanCall("CloseDB", dest1, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("Open", dest2, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupQueryPlan(dest2, "SELECT body FROM t", "len(got) == 1; strings.Contains(got[0][0].(string), 数据库)"),
		backupPlanCall("CloseDB", dest2, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("CloseDB fixture", path, nil, "error == nil; child transaction is ended"),
	})
	db := backupOpen(t, scope, path, sqlite.Options{})
	backupExec(t, scope, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,body TEXT)")
	backupExec(t, scope, db, "INSERT INTO t VALUES(1,'pinned 数据库')")
	scope.Before("begin pinned transaction")
	tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	scope.RetainErrors(err)
	if tx != nil {
		scope.OwnTx(db, tx)
	}
	scope.Observe("begin pinned transaction", tx != nil, err)
	if err != nil {
		t.Fatal(err)
	}
	scope.Before("pin transaction check")
	check, err := tx.Check(ctx)
	scope.Observe("pin transaction check", check, err)
	if err != nil {
		t.Fatal(err)
	}
	other := backupOpen(t, scope, path, sqlite.Options{ReadOnly: true, NoCreate: true})
	var a, b sqlite.FileInfo
	var ea, eb error
	var wait sync.WaitGroup
	wait.Add(2)
	scope.Before("concurrent transaction backup")
	scope.Before("concurrent peer backup")
	go func() { defer wait.Done(); a, ea = tx.Backup(ctx, dest1) }()
	go func() { defer wait.Done(); b, eb = other.Backup(ctx, dest2) }()
	wait.Wait()
	scope.RetainErrors(ea, eb)
	// Retain both results before either error can stop the test. No scope or
	// testing callback runs on the two native-operation goroutines.
	scope.Record("concurrent transaction backup info", a)
	scope.Record("concurrent transaction backup error", ea)
	scope.Record("concurrent peer backup info", b)
	scope.Record("concurrent peer backup error", eb)
	scope.Observe("concurrent transaction backup", a, ea)
	scope.Observe("concurrent peer backup", b, eb)
	var autocommit bool
	if a == b {
		autocommit = db.Autocommit()
		scope.Record("fixture autocommit", autocommit)
	}
	if ea != nil || eb != nil || a != b || autocommit {
		t.Fatal(a, b, ea, eb)
	}
	scope.Before("post-backup transaction check")
	check, err = tx.Check(ctx)
	scope.Observe("post-backup transaction check", check, err)
	if err != nil {
		t.Fatal("backup released transaction snapshot", err)
	}
	if err = scope.CommitTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err = scope.CloseDB(other); err != nil {
		t.Fatal(err)
	}
	for _, dest := range []string{dest1, dest2} {
		copy := backupOpen(t, scope, dest, sqlite.Options{ReadOnly: true, NoCreate: true})
		if got := backupQuery(t, scope, copy, "SELECT body FROM t"); len(got) != 1 || !strings.Contains(got[0][0].(string), "数据库") {
			t.Fatal(got)
		}
		if err = scope.CloseDB(copy); err != nil {
			t.Fatal(err)
		}
	}
	if err = scope.CloseDB(db); err != nil {
		t.Fatal(err)
	}
	scope.FinishSuccess()
	run.FinishSuccess()
}

func TestPlaintextBackupEmptyAndLazyReadOnly(t *testing.T) {
	run := backupmanual.ForTest(t)
	ctx := context.Background()
	scope := run.Case(t, "empty-lazy-readonly")
	empty := filepath.Join(scope.Dir("source"), "empty.db")
	first := filepath.Join(scope.Dir("connection-target"), "connection.db")
	second := filepath.Join(scope.Dir("lazy-target"), "lazy.db")
	scope.SetPlan([]any{
		backupPlanCall("Open", empty, sqlite.Options{PageSize: 512}, "error == nil"),
		backupPlanCall("DB.Backup", first, map[string]any{"source": empty}, "error == nil; info.PageCount == 1"),
		backupPlanCall("Begin", empty, sqlite.TxOptions{ReadOnly: true}, "error == nil"),
		backupPlanCall("Tx.Backup", second, map[string]any{"source": empty, "lazy": true}, "error == nil; info.PageCount == 1; no prior pin/query/check"),
		backupPlanCall("CommitTx", empty, nil, "error == nil; one end attempt; no fallback Rollback"),
		backupPlanCall("Open", first, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupPlanCall("Check", first, nil, "error == nil; result.Tables == 0; result.Pages == 1"),
		backupPlanCall("CloseDB", first, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("Open", second, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupPlanCall("Check", second, nil, "error == nil; result.Tables == 0; result.Pages == 1"),
		backupPlanCall("CloseDB", second, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("CloseDB", empty, nil, "error == nil; child transaction is ended"),
	})
	db := backupOpen(t, scope, empty, sqlite.Options{PageSize: 512})
	scope.Before("empty connection backup")
	info, err := db.Backup(ctx, first)
	scope.Observe("empty connection backup", info, err)
	if err != nil || info.PageCount != 1 {
		t.Fatal(info, err)
	}
	scope.Before("begin lazy transaction")
	tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	scope.RetainErrors(err)
	if tx != nil {
		scope.OwnTx(db, tx)
	}
	scope.Observe("begin lazy transaction", tx != nil, err)
	if err != nil {
		t.Fatal(err)
	}
	scope.Before("lazy transaction backup")
	info, err = tx.Backup(ctx, second)
	scope.Observe("lazy transaction backup", info, err)
	if err != nil || info.PageCount != 1 {
		t.Fatal(info, err)
	}
	if err = scope.CommitTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first, second} {
		copy := backupOpen(t, scope, path, sqlite.Options{ReadOnly: true, NoCreate: true})
		scope.Before("empty copy check")
		result, err := copy.Check(ctx)
		scope.Observe("empty copy check", result, err)
		if err != nil || result.Tables != 0 || result.Pages != 1 {
			t.Fatal(result, err)
		}
		if err = scope.CloseDB(copy); err != nil {
			t.Fatal(err)
		}
	}
	if err = scope.CloseDB(db); err != nil {
		t.Fatal(err)
	}
	scope.FinishSuccess()
	run.FinishSuccess()
}
