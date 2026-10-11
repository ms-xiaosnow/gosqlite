package sqlite

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

// These fixtures are built exclusively through successful normal APIs. No raw
// database bytes, I/O outcomes, namespace objects or storage state are changed.
func inspectMetadataOpen(t *testing.T, path string, options Options) *DB {
	t.Helper()
	db, err := OpenContext(context.Background(), path, options)
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

func inspectMetadataExec(t *testing.T, db *DB, query string) {
	t.Helper()
	if _, err := db.Exec(context.Background(), query); err != nil {
		t.Fatal(err)
	}
}

func inspectMetadataCreate(t *testing.T, options Options) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "complete.db")
	db := inspectMetadataOpen(t, path, options)
	inspectMetadataExec(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)")
	inspectMetadataExec(t, db, "INSERT INTO docs VALUES(1,'数据库 a-b'),(2,'second')")
	return db, path
}

func inspectMetadataClose(t *testing.T, db *DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func inspectMetadataRead(t *testing.T, db *DB) InspectResult {
	t.Helper()
	result, err := db.Inspect(context.Background(), InspectOptions{IncludeExtensions: true})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// Inventory and hash every directory entry before read-only reopen; compare
// after explicit Close as well as on cleanup, detecting any sidecar creation.
func inspectMetadataUnchanged(t *testing.T, path string) func() {
	t.Helper()
	snapshot := func() map[string][32]byte {
		t.Helper()
		result := map[string][32]byte{}
		root := filepath.Dir(path)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if entry.IsDir() {
				result[rel+"/"] = [32]byte{}
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[rel] = sha256.Sum256(data)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := snapshot()
	check := func() {
		t.Helper()
		if after := snapshot(); !reflect.DeepEqual(after, before) {
			t.Fatalf("read-only directory changed: before %v; after %v", before, after)
		}
	}
	t.Cleanup(check)
	return check
}

func TestInspectMetadataEmptyReadonly(t *testing.T) {
	db, path := inspectMetadataCreate(t, Options{})
	inspectMetadataClose(t, db)
	unchanged := inspectMetadataUnchanged(t, path)
	ro := inspectMetadataOpen(t, path, Options{ReadOnly: true})
	result := inspectMetadataRead(t, ro)
	if result.ExtensionIndexes == nil || len(result.ExtensionIndexes) != 0 || len(result.Schema) != 1 {
		t.Fatalf("unexpected empty metadata: %+v", result)
	}
	plain, err := ro.Inspect(context.Background(), InspectOptions{})
	if err != nil || plain.ExtensionIndexes != nil || !reflect.DeepEqual(plain.Schema, result.Schema) || plain.Header != result.Header {
		t.Fatalf("default metadata = %+v, %v", plain, err)
	}
	inspectMetadataClose(t, ro)
	unchanged()
}

func TestInspectMetadataGramsRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, path := inspectMetadataCreate(t, Options{})
	config := grams13.DefaultMatchConfig()
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	inspectMetadataClose(t, db)
	checkPhase := func(t *testing.T, generation int64) {
		t.Helper()
		unchanged := inspectMetadataUnchanged(t, path)
		ro := inspectMetadataOpen(t, path, Options{ReadOnly: true})
		got := inspectMetadataRead(t, ro)
		canonical, err := grams13.ConfigBytes(config)
		if err != nil {
			t.Fatal(err)
		}
		want := []ExtensionIndexMetadata{{ID: 1, Name: "docs_grams", SourceTable: "docs", Method: grams13.MethodName, MethodVersion: grams13.MethodVersion, StorageVersion: 1, Config: string(canonical), Generation: generation}}
		if !reflect.DeepEqual(got.ExtensionIndexes, want) {
			t.Fatalf("metadata = %#v; want %#v", got.ExtensionIndexes, want)
		}
		inspectMetadataClose(t, ro)
		unchanged()
	}
	// Retire each read-only phase's inventory assertion before the next write.
	t.Run("created", func(t *testing.T) { checkPhase(t, 1) })
	config.Punctuation = grams13.PunctuationRemove
	writer := inspectMetadataOpen(t, path, Options{})
	if err := writer.Reindex(ctx, "docs_grams", GramsIndexOptions{MatchConfig: config}); err != nil {
		t.Fatal(err)
	}
	inspectMetadataClose(t, writer)
	t.Run("reindexed", func(t *testing.T) { checkPhase(t, 2) })
}

type inspectMethodCounts struct{ configure, canonical, keys atomic.Int64 }
type inspectWholeMethod struct{ counts *inspectMethodCounts }

func (inspectWholeMethod) Name() string { return "inspect_whole" }
func (inspectWholeMethod) Version() int { return 1 }
func (m inspectWholeMethod) Configure(config []byte) (registry.Index, error) {
	m.counts.configure.Add(1)
	// A harmless descriptive label exercises ordinary opaque config overflow.
	if string(config) != "whole-v1" && !strings.HasPrefix(string(config), "whole-v1;label=") {
		return nil, fmt.Errorf("unsupported whole-text configuration")
	}
	return inspectWholeIndex{m.counts, string(config)}, nil
}

type inspectWholeIndex struct {
	counts *inspectMethodCounts
	config string
}

func (m inspectWholeIndex) Canonical() []byte {
	m.counts.canonical.Add(1)
	return []byte(m.config)
}
func (m inspectWholeIndex) Keys(ctx context.Context, value string) ([]registry.Key, error) {
	m.counts.keys.Add(1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []registry.Key{{Namespace: "WHOLE_V1", Length: 1, Text: value}}, nil
}
func (m *inspectMethodCounts) values() [3]int64 {
	return [3]int64{m.configure.Load(), m.canonical.Load(), m.keys.Load()}
}

func inspectMetadataCustomFixture(t *testing.T, config string) (string, *registry.Registry, *inspectMethodCounts) {
	t.Helper()
	counts := &inspectMethodCounts{}
	methods, err := registry.New(inspectWholeMethod{counts})
	if err != nil {
		t.Fatal(err)
	}
	db, path := inspectMetadataCreate(t, Options{IndexMethods: methods})
	if err := db.CreateMethodIndex(context.Background(), "whole", "docs", []string{"body"}, "inspect_whole", 1, []byte(config)); err != nil {
		t.Fatal(err)
	}
	inspectMetadataClose(t, db)
	return path, methods, counts
}

func TestInspectMetadataCustomMethodNoCallbacks(t *testing.T) {
	path, methods, counts := inspectMetadataCustomFixture(t, "whole-v1")
	unchanged := inspectMetadataUnchanged(t, path)
	before := counts.values()
	for _, value := range before {
		if value == 0 {
			t.Fatalf("fixture did not use each callback: %v", before)
		}
	}
	for _, registered := range []*registry.Registry{methods, nil} {
		ro := inspectMetadataOpen(t, path, Options{ReadOnly: true, IndexMethods: registered})
		got := inspectMetadataRead(t, ro)
		want := []ExtensionIndexMetadata{{1, "whole", "docs", "inspect_whole", 1, 1, "whole-v1", 1}}
		if !reflect.DeepEqual(got.ExtensionIndexes, want) {
			t.Fatalf("metadata = %#v; want %#v", got.ExtensionIndexes, want)
		}
		inspectMetadataClose(t, ro)
	}
	if after := counts.values(); after != before {
		t.Fatalf("inspection invoked callbacks: %v -> %v", before, after)
	}
	unchanged()
}

// This observer forwards each read unchanged, including its returned error. It
// only records page numbers and does not inject cancellation, data or failures.
type inspectMetadataPageRecorder struct {
	btree.PageReader
	pages map[uint32]int
}

func (r *inspectMetadataPageRecorder) ReadPage(ctx context.Context, page uint32) ([]byte, error) {
	r.pages[page]++
	return r.PageReader.ReadPage(ctx, page)
}

func inspectMetadataRecorded(t *testing.T, db *DB) (InspectResult, map[uint32]int, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Begin(ctx, TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			t.Error(err)
		}
	}()
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := tx.ensureSnapshotLocked(ctx); err != nil {
		t.Fatal(err)
	}
	r := &inspectMetadataPageRecorder{PageReader: tx.reader, pages: map[uint32]int{}}
	result, err := db.inspectSnapshot(ctx, r, InspectOptions{IncludeExtensions: true})
	return result, r.pages, err
}

func TestInspectMetadataCatalogOnlyReads(t *testing.T) {
	path, _, _ := inspectMetadataCustomFixture(t, "whole-v1")
	unchanged := inspectMetadataUnchanged(t, path)
	ro := inspectMetadataOpen(t, path, Options{ReadOnly: true})
	got, pages, err := inspectMetadataRecorded(t, ro)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ExtensionIndexes) != 1 || pages[1] == 0 {
		t.Fatalf("missing schema/catalog: %+v, %v", got, pages)
	}
	for _, entry := range got.Schema {
		if entry.Name == extensionCatalogName {
			if pages[entry.RootPage] == 0 {
				t.Fatal("catalog was not read")
			}
		} else if entry.RootPage > 0 && pages[entry.RootPage] != 0 {
			t.Fatalf("read non-catalog data root %q (%d): %v", entry.Name, entry.RootPage, pages)
		}
	}
	inspectMetadataClose(t, ro)
	unchanged()
}

func TestInspectMetadataQuota(t *testing.T) {
	config := "whole-v1;label=" + strings.Repeat("ordinary", 1024)
	path, _, _ := inspectMetadataCustomFixture(t, config)
	unchanged := inspectMetadataUnchanged(t, path)
	ro := inspectMetadataOpen(t, path, Options{ReadOnly: true})
	complete, allPages, err := inspectMetadataRecorded(t, ro)
	if err != nil || len(complete.ExtensionIndexes) != 1 || complete.ExtensionIndexes[0].Config != config {
		t.Fatalf("complete fixture: %+v, %v", complete, err)
	}
	inspectMetadataClose(t, ro)
	for _, limit := range []int64{1, 128 << 10} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			limited := inspectMetadataOpen(t, path, Options{ReadOnly: true, MaxSortBytes: limit})
			result, err := limited.Inspect(context.Background(), InspectOptions{IncludeExtensions: true})
			if !errors.Is(err, ErrMemoryLimit) || !reflect.DeepEqual(result, InspectResult{}) {
				t.Fatalf("limit=%d result=%+v err=%v", limit, result, err)
			}
			if limit > 1 {
				_, pages, err := inspectMetadataRecorded(t, limited)
				if !errors.Is(err, ErrMemoryLimit) {
					t.Fatalf("recorded quota: %v", err)
				}
				if len(pages) >= len(allPages) {
					t.Fatalf("quota did not stop before config overflow pages: quota %v; full %v", pages, allPages)
				}
				for _, entry := range complete.Schema {
					if entry.Name == extensionCatalogName && pages[entry.RootPage] == 0 {
						t.Fatal("quota did not reach the catalog record preflight")
					}
				}
			}
			inspectMetadataClose(t, limited)
		})
	}
	unchanged()
}

func TestInspectMetadataContexts(t *testing.T) {
	db, path := inspectMetadataCreate(t, Options{})
	inspectMetadataClose(t, db)
	unchanged := inspectMetadataUnchanged(t, path)
	ro := inspectMetadataOpen(t, path, Options{ReadOnly: true})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stop()
	for _, tc := range []struct {
		ctx  context.Context
		want error
	}{{canceled, context.Canceled}, {expired, context.DeadlineExceeded}} {
		got, err := ro.Inspect(tc.ctx, InspectOptions{IncludeExtensions: true})
		if !errors.Is(err, tc.want) || !reflect.DeepEqual(got, InspectResult{}) {
			t.Fatalf("result=%+v err=%v; want %v", got, err, tc.want)
		}
	}
	inspectMetadataRead(t, ro)
	inspectMetadataClose(t, ro)
	unchanged()
}

func TestInspectMetadataTransactionAndFreshHeader(t *testing.T) {
	ctx := context.Background()
	db, path := inspectMetadataCreate(t, Options{})
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "CREATE TABLE pending(id INTEGER PRIMARY KEY, body TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO pending VALUES(1,'normal')"); err != nil {
		t.Fatal(err)
	}
	if err := tx.CreateGramsIndex(ctx, "pending_grams", "pending", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	got := inspectMetadataRead(t, db)
	if len(got.ExtensionIndexes) != 1 || got.ExtensionIndexes[0].SourceTable != "pending" {
		t.Fatalf("missing transaction metadata: %+v", got)
	}
	if got.Header != fileInfo(tx.reader.Header()) {
		t.Fatalf("header is outside transaction: %+v", got.Header)
	}
	found := false
	for _, entry := range got.Schema {
		found = found || entry.Name == "pending"
	}
	if !found {
		t.Fatal("uncommitted DDL omitted")
	}
	// A subsequent ordinary query/write/commit proves Inspect did not close the
	// borrowed reader or retire the transaction.
	rows, err := tx.Query(ctx, "SELECT body FROM pending")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Values()[0] != "normal" {
		t.Fatal("transaction reader no longer usable")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO pending VALUES(2,'still usable')"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	inspectMetadataClose(t, db)
	// Open the observer before a separate connection's successful commit.
	ro := inspectMetadataOpen(t, path, Options{ReadOnly: true})
	old := ro.Info()
	writer := inspectMetadataOpen(t, path, Options{})
	inspectMetadataExec(t, writer, "CREATE TABLE later(value TEXT)")
	if _, err := writer.Schema(ctx); err != nil {
		t.Fatal(err)
	}
	wantHeader := writer.Info()
	inspectMetadataClose(t, writer)
	unchanged := inspectMetadataUnchanged(t, path)
	fresh := inspectMetadataRead(t, ro)
	if fresh.Header.SchemaCookie <= old.SchemaCookie || fresh.Header != wantHeader {
		t.Fatalf("stale header: before %+v; after %+v; committed %+v", old, fresh.Header, wantHeader)
	}
	found = false
	for _, entry := range fresh.Schema {
		found = found || entry.Name == "later"
	}
	if !found {
		t.Fatal("later committed DDL omitted")
	}
	if ro.Info() != fresh.Header {
		t.Fatal("Info did not observe the inspected snapshot")
	}
	inspectMetadataClose(t, ro)
	unchanged()
}
