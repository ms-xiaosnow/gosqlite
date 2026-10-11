package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Create a small complete rollback-mode database using only successful APIs,
// close it, then preserve its complete directory inventory and hashes.
func inspectMetadataCLIFixture(t *testing.T, extensions bool) (string, sqlite.FileInfo, []sqlite.SchemaEntry) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metadata.db")
	db, err := sqlite.OpenContext(context.Background(), path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, query := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)", "INSERT INTO docs VALUES(1,'数据库')"} {
		if _, err := db.Exec(context.Background(), query); err != nil {
			t.Fatal(err)
		}
	}
	if extensions {
		if err := db.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"body"}); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := db.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	header := db.Info()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	inspectMetadataCLIUnchanged(t, path)
	return path, header, schema
}

func inspectMetadataCLIUnchanged(t *testing.T, path string) {
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
	t.Cleanup(func() {
		if after := snapshot(); !reflect.DeepEqual(after, before) {
			t.Fatalf("read-only directory changed: before %v; after %v", before, after)
		}
	})
}

func TestInspectMetadataDefaultJSON(t *testing.T) {
	for _, extensions := range []bool{false, true} {
		path, header, schema := inspectMetadataCLIFixture(t, extensions)
		want, err := json.Marshal(struct {
			Format string               `json:"format"`
			Header sqlite.FileInfo      `json:"header"`
			Schema []sqlite.SchemaEntry `json:"schema"`
		}{"SQLite format 3 (metadata inspection only)", header, schema})
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := runContext(context.Background(), []string{"inspect", path}, &out); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), append(want, '\n')) {
			t.Fatalf("default inspect bytes changed: got %s; want %s", out.Bytes(), want)
		}
	}
}

func TestInspectMetadataExtensionsJSON(t *testing.T) {
	path, header, schema := inspectMetadataCLIFixture(t, true)
	canonical, err := grams13.ConfigBytes(grams13.DefaultMatchConfig())
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(struct {
		Format           string                          `json:"format"`
		Header           sqlite.FileInfo                 `json:"header"`
		Schema           []sqlite.SchemaEntry            `json:"schema"`
		ExtensionIndexes []sqlite.ExtensionIndexMetadata `json:"extension_indexes"`
	}{"SQLite format 3 (metadata inspection only)", header, schema, []sqlite.ExtensionIndexMetadata{{ID: 1, Name: "docs_grams", SourceTable: "docs", Method: "grams13", MethodVersion: 1, StorageVersion: 1, Config: string(canonical), Generation: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runContext(context.Background(), []string{"inspect", "--extensions", path}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), append(want, '\n')) {
		t.Fatalf("inspect bytes: got %s; want %s", out.Bytes(), want)
	}
}

func TestInspectMetadataEmptyExtensionsJSON(t *testing.T) {
	path, _, _ := inspectMetadataCLIFixture(t, false)
	var out bytes.Buffer
	if err := runContext(context.Background(), []string{"inspect", "--extensions", path}, &out); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if string(got["extension_indexes"]) != "[]" {
		t.Fatalf("empty metadata = %s", out.Bytes())
	}
}

func TestInspectMetadataCanceledOutput(t *testing.T) {
	path, _, _ := inspectMetadataCLIFixture(t, true)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stop()
	for _, tc := range []struct {
		ctx  context.Context
		want error
	}{{canceled, context.Canceled}, {expired, context.DeadlineExceeded}} {
		var out bytes.Buffer
		err := runContext(tc.ctx, []string{"inspect", "--extensions", path}, &out)
		if !errors.Is(err, tc.want) || out.Len() != 0 {
			t.Fatalf("canceled output=%q error=%v; want %v", out.String(), err, tc.want)
		}
	}
}

// A legitimate application-defined method whose persisted config is not JSON.
// The CLI cannot register it and must still inspect it without resolving code.
type inspectCLIWholeMethod struct{}
type inspectCLIWholeIndex struct{}

func (inspectCLIWholeMethod) Name() string { return "cli_whole" }
func (inspectCLIWholeMethod) Version() int { return 1 }
func (inspectCLIWholeMethod) Configure(config []byte) (registry.Index, error) {
	if string(config) != "whole-v1" {
		return nil, errors.New("unsupported whole config")
	}
	return inspectCLIWholeIndex{}, nil
}
func (inspectCLIWholeIndex) Canonical() []byte { return []byte("whole-v1") }
func (inspectCLIWholeIndex) Keys(ctx context.Context, value string) ([]registry.Key, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []registry.Key{{Namespace: "WHOLE_V1", Length: 1, Text: value}}, nil
}

func TestInspectMetadataOpaqueConfigJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.db")
	methods, err := registry.New(inspectCLIWholeMethod{})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.OpenContext(context.Background(), path, sqlite.Options{IndexMethods: methods})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, query := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)", "INSERT INTO docs VALUES(1,'ordinary')"} {
		if _, err := db.Exec(context.Background(), query); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.CreateMethodIndex(context.Background(), "whole", "docs", []string{"body"}, "cli_whole", 1, []byte("whole-v1")); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	inspectMetadataCLIUnchanged(t, path)
	var out bytes.Buffer
	if err := runContext(context.Background(), []string{"inspect", "--extensions", path}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Indexes []sqlite.ExtensionIndexMetadata `json:"extension_indexes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	want := []sqlite.ExtensionIndexMetadata{{ID: 1, Name: "whole", SourceTable: "docs", Method: "cli_whole", MethodVersion: 1, StorageVersion: 1, Config: "whole-v1", Generation: 1}}
	if !reflect.DeepEqual(result.Indexes, want) {
		t.Fatalf("opaque config metadata = %#v; want %#v", result.Indexes, want)
	}
}
