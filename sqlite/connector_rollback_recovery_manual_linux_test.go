//go:build manual_recovery && linux

package sqlite_test

// SOURCE_ONLY / NOT_RUN. This independent Connector entry is never an ordinary
// test or a CI recovery permission. The explicit gate is checked before any
// filesystem inspection, fixture creation, constructor or connection. All files
// are retained. No process, fault injection, cleanup retry or deletion is used.

import (
	"context"
	sqldriver "database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	gosqlitedriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestManualConnectorRollbackRecovery(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_CONNECTOR_RECOVERY") != "independently_authorized" {
		t.Fatal("NOT_RUN: Connector physical recovery requires independent authorization")
	}
	root := os.Getenv("GOSQLITE_MANUAL_CONNECTOR_RECOVERY_DIR")
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		t.Fatal("GOSQLITE_MANUAL_CONNECTOR_RECOVERY_DIR must name a fresh absolute clean non-root directory")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("disposable root must be an existing non-symlink directory: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root {
		t.Fatalf("disposable root must have no symlink components: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("disposable root must be empty: entries=%d error=%v", len(entries), err)
	}
	manualPublicRecoveryNoPending(t)
	t.Cleanup(func() {
		for _, pending := range sqlite.PendingPlaintextCleanups() {
			t.Errorf("pending owner retained; no cleanup retry or fixture deletion: %v", pending)
		}
	})
	t.Logf("retaining every Connector input and working file under %s", root)

	if !t.Run("nil-default-preserves-input", func(t *testing.T) {
		path, _, current, data := manualPublicRecoveryFiles(t, root)
		opts := sqlite.Options{NoCreate: true}
		connector, err := gosqlitedriver.NewConnector(path, opts)
		manualConnectorRecoveryConstructed(t, connector, err)
		// Replacing the caller's nil policy cannot enable captured options.
		opts.RollbackRecovery = &sqlite.RollbackRecoveryOptions{}
		manualPublicRecoveryEqual(t, path, current)
		manualPublicRecoveryEqual(t, path+"-journal", data)
		manualConnectorRecoveryNoWAL(t, path)
		manualPublicRecoveryNoPending(t)
		scope := manualConnectorRecoveryScopeFor(t)
		conn, err := connector.Connect(context.Background())
		scope.own("connection", conn)
		manualConnectorRecoveryRefusal(t, scope, conn, err, sqlite.ErrRecoveryRequired)
		manualPublicRecoveryEqual(t, path, current)
		manualPublicRecoveryEqual(t, path+"-journal", data)
		manualConnectorRecoveryNoWAL(t, path)
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
	if !t.Run("opt-in-restores-and-reconnects", func(t *testing.T) {
		path, original, current, data := manualPublicRecoveryFiles(t, root)
		cfg := &sqlite.RollbackRecoveryOptions{}
		connector, err := gosqlitedriver.NewConnector(path, sqlite.Options{NoCreate: true, RollbackRecovery: cfg})
		manualConnectorRecoveryConstructed(t, connector, err)
		// All three scalar limits must have been copied by NewConnector. Any
		// retained caller pointer now rejects this fixture instead of restoring it.
		cfg.MaxJournalBytes, cfg.MaxRecords, cfg.MaxDatabaseBytes = 1, 1, 1
		manualPublicRecoveryEqual(t, path, current)
		manualPublicRecoveryEqual(t, path+"-journal", data)
		manualConnectorRecoveryNoWAL(t, path)
		manualPublicRecoveryNoPending(t)
		scope := manualConnectorRecoveryScopeFor(t)
		conn, err := connector.Connect(context.Background())
		owner := scope.own("connection", conn)
		if err != nil || conn == nil {
			scope.retain(err)
			t.Fatalf("expected Connector recovery success: conn=%T error=%v", conn, err)
		}
		manualConnectorRecoveryReadSchema(t, scope, conn)
		if !scope.close(owner) {
			return
		}
		manualPublicRecoveryNoPending(t)
		manualPublicRecoveryEqual(t, path, original)
		manualPublicRecoveryAbsent(t, path+"-journal")
		manualConnectorRecoveryNoWAL(t, path)

		readonly, err := gosqlitedriver.NewConnector(path, sqlite.Options{ReadOnly: true, NoCreate: true})
		manualConnectorRecoveryConstructed(t, readonly, err)
		manualPublicRecoveryEqual(t, path, original)
		manualPublicRecoveryAbsent(t, path+"-journal")
		manualConnectorRecoveryNoWAL(t, path)
		manualPublicRecoveryNoPending(t)
		conn, err = readonly.Connect(context.Background())
		owner = scope.own("read-only connection", conn)
		if err != nil || conn == nil {
			scope.retain(err)
			t.Fatalf("expected read-only Connector reconnect: conn=%T error=%v", conn, err)
		}
		manualConnectorRecoveryReadSchema(t, scope, conn)
		if !scope.close(owner) {
			return
		}
		manualPublicRecoveryEqual(t, path, original)
		manualPublicRecoveryAbsent(t, path+"-journal")
		manualConnectorRecoveryNoWAL(t, path)
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
	if !t.Run("journal-limit-preserves-input", func(t *testing.T) {
		path, _, current, data := manualPublicRecoveryFiles(t, root)
		cfg := &sqlite.RollbackRecoveryOptions{MaxJournalBytes: 1}
		connector, err := gosqlitedriver.NewConnector(path, sqlite.Options{NoCreate: true, RollbackRecovery: cfg})
		manualConnectorRecoveryConstructed(t, connector, err)
		// Relaxing the caller's limit must not relax the captured one-byte cap.
		cfg.MaxJournalBytes = 0
		manualPublicRecoveryEqual(t, path, current)
		manualPublicRecoveryEqual(t, path+"-journal", data)
		manualConnectorRecoveryNoWAL(t, path)
		manualPublicRecoveryNoPending(t)
		scope := manualConnectorRecoveryScopeFor(t)
		conn, err := connector.Connect(context.Background())
		scope.own("connection", conn)
		manualConnectorRecoveryRefusal(t, scope, conn, err, sqlite.ErrRecoveryRequired, sqlite.ErrMemoryLimit)
		manualPublicRecoveryEqual(t, path, current)
		manualPublicRecoveryEqual(t, path+"-journal", data)
		manualConnectorRecoveryNoWAL(t, path)
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
	if !t.Run("no-create-preserves-absence", func(t *testing.T) {
		dir, err := os.MkdirTemp(root, "connector-no-create-")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("retained isolated directory: %s", dir)
		path := filepath.Join(dir, "absent.db")
		manualConnectorRecoveryEmpty(t, dir, path)
		opts := sqlite.Options{NoCreate: true, RollbackRecovery: &sqlite.RollbackRecoveryOptions{}}
		connector, err := gosqlitedriver.NewConnector(path, opts)
		manualConnectorRecoveryConstructed(t, connector, err)
		manualConnectorRecoveryEmpty(t, dir, path)
		manualPublicRecoveryNoPending(t)
		opts.NoCreate = false
		scope := manualConnectorRecoveryScopeFor(t)
		conn, err := connector.Connect(context.Background())
		scope.own("connection", conn)
		if conn != nil {
			scope.retain(err)
			t.Fatalf("NoCreate unexpectedly returned a connection: %v", err)
		}
		scope.retain(err)
		manualConnectorRecoveryCleanRefusal(t, err)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("NoCreate must retain absence cause: %v", err)
		}
		manualConnectorRecoveryEmpty(t, dir, path)
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
}

func manualConnectorRecoveryConstructed(t *testing.T, connector *gosqlitedriver.Connector, err error) {
	t.Helper()
	if err != nil || connector == nil {
		t.Fatalf("expected no-I/O NewConnector success: connector=%T error=%v", connector, err)
	}
}

func manualConnectorRecoveryNoWAL(t *testing.T, path string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm"} {
		manualPublicRecoveryAbsent(t, path+suffix)
	}
}

func manualConnectorRecoveryEmpty(t *testing.T, dir, path string) {
	t.Helper()
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		manualPublicRecoveryAbsent(t, path+suffix)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("NoCreate constructor/Connect changed empty directory: %v %v", entries, err)
	}
}

func manualConnectorRecoveryRefusal(t *testing.T, scope *manualConnectorRecoveryScope, conn sqldriver.Conn, err error, causes ...error) {
	t.Helper()
	scope.retain(err)
	if conn != nil {
		t.Fatalf("expected refusal returned a connection: %v", err)
	}
	manualConnectorRecoveryCleanRefusal(t, err)
	var diagnostic *sqlite.Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != sqlite.CodeIOErr {
		t.Fatalf("expected public CodeIOErr from Connect: %v", err)
	}
	for _, cause := range causes {
		if !errors.Is(err, cause) {
			t.Fatalf("Connect lost cause %v: %v", cause, err)
		}
	}
}

// A sentinel match alone must not hide known uncertainty/cleanup diagnostics.
// The complete received error stays reachable in the local scope and is logged;
// no foreign or global owner is retried, even for an expected refusal.
func manualConnectorRecoveryCleanRefusal(t *testing.T, err error) {
	t.Helper()
	t.Logf("retained Connector refusal diagnostic: %v", err)
	if errors.Is(err, sqlite.ErrPoisoned) || errors.Is(err, sqlite.ErrPlaintextCleanupPending) || errors.Is(err, sqlite.ErrCommitUncertain) {
		t.Fatalf("refusal includes ownership/commit uncertainty; preserve all fixtures: %v", err)
	}
}

// An empty sqlite_schema is a real healthy logical read, through the adapter's
// Prepare -> Stmt.Query -> Rows.Next path. It never opens a native DB directly.
func manualConnectorRecoveryReadSchema(t *testing.T, scope *manualConnectorRecoveryScope, conn sqldriver.Conn) {
	t.Helper()
	stmt, err := conn.Prepare("SELECT name FROM sqlite_schema")
	statement := scope.own("statement", stmt)
	if err != nil || stmt == nil {
		scope.retain(err)
		t.Fatalf("prepare restored schema: %v", err)
	}
	rows, err := stmt.Query(nil)
	rowOwner := scope.own("rows", rows)
	if err != nil || rows == nil {
		scope.retain(err)
		t.Fatalf("query restored schema: %v", err)
	}
	if columns := rows.Columns(); len(columns) != 1 || columns[0] != "name" {
		t.Fatalf("restored schema columns: %v", columns)
	}
	if err := scope.next(rows, rowOwner, make([]sqldriver.Value, 1)); err != io.EOF {
		t.Fatalf("expected empty restored sqlite_schema and clean EOF: %v", err)
	}
	if !scope.close(rowOwner) || !scope.close(statement) {
		t.Fatal("schema owners did not retire; retain connection and fixtures")
	}
}

// Own only objects returned in this scenario. LIFO cleanup keeps rows before
// statement before connection. Once any retirement is unknown, retain all
// remaining owners and errors; never use global cleanup/retry APIs.
type manualConnectorRecoveryOwner struct {
	kind      string
	value     interface{ Close() error }
	attempted bool
	retired   bool
	err       error
}

type manualConnectorRecoveryScope struct {
	t      *testing.T
	owners []*manualConnectorRecoveryOwner
	frozen bool
	errors []error
}

func manualConnectorRecoveryScopeFor(t *testing.T) *manualConnectorRecoveryScope {
	scope := &manualConnectorRecoveryScope{t: t}
	t.Cleanup(func() {
		for i := len(scope.owners) - 1; i >= 0; i-- {
			if !scope.close(scope.owners[i]) {
				break
			}
		}
		for _, owner := range scope.owners {
			if !owner.retired {
				t.Errorf("retained %s: attempted=%t error=%v; no retry or deletion", owner.kind, owner.attempted, owner.err)
			}
		}
	})
	return scope
}

func (scope *manualConnectorRecoveryScope) own(kind string, value interface{ Close() error }) *manualConnectorRecoveryOwner {
	if value == nil {
		return nil
	}
	owner := &manualConnectorRecoveryOwner{kind: kind, value: value}
	scope.owners = append(scope.owners, owner)
	return owner
}

func (scope *manualConnectorRecoveryScope) retain(err error) {
	if err != nil {
		scope.frozen = true
		scope.errors = append(scope.errors, err)
	}
}

func (scope *manualConnectorRecoveryScope) close(owner *manualConnectorRecoveryOwner) bool {
	if owner == nil {
		return true
	}
	if owner.retired {
		return true
	}
	if scope.frozen || owner.attempted {
		return false
	}
	owner.attempted = true
	owner.err = owner.value.Close()
	if owner.err != nil {
		scope.retain(owner.err)
		scope.t.Errorf("%s Close failed; retain all remaining owners and fixtures: %v", owner.kind, owner.err)
		return false
	}
	owner.retired = true
	return true
}

// The pinned adapter's Next calls native Rows.Next, whose terminal path closes
// its reader. Mark the possible retirement BEFORE the call. Only an exact EOF
// proves terminal success; an error (or panic) never earns a later Close retry.
func (scope *manualConnectorRecoveryScope) next(rows sqldriver.Rows, owner *manualConnectorRecoveryOwner, dest []sqldriver.Value) error {
	if scope.frozen || owner.attempted || owner.retired {
		scope.t.Fatal("cannot advance rows after retirement or uncertainty")
	}
	owner.attempted = true
	err := rows.Next(dest)
	if err == nil {
		owner.attempted = false
		return nil
	}
	if err == io.EOF {
		owner.retired = true
		return err
	}
	owner.err = err
	scope.retain(err)
	return err
}
