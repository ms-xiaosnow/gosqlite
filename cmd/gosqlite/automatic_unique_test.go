package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestCLIAutomaticUniqueMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "automatic-unique.db")
	var out bytes.Buffer
	for _, query := range []string{
		"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT UNIQUE)",
		"CREATE INDEX ordinary ON t(id)",
		"INSERT INTO t VALUES(1,'one'),(2,NULL),(3,NULL)",
	} {
		out.Reset()
		if err := run([]string{"query", path, query}, &out); err != nil {
			t.Fatal(query, err)
		}
	}
	out.Reset()
	if err := run([]string{"query", path, "INSERT INTO t VALUES(4,'one')"}, &out); !errors.Is(err, sqlite.ErrConstraint) {
		t.Fatal("CLI reopened writer lost UNIQUE", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err = run([]string{"inspect", path}, &out); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Format string                       `json:"format"`
		Schema []map[string]json.RawMessage `json:"schema"`
	}
	if err = json.Unmarshal(out.Bytes(), &document); err != nil || len(document.Schema) != 3 || document.Format != "SQLite format 3 (metadata inspection only)" {
		t.Fatal(out.String(), err)
	}
	for _, row := range document.Schema {
		var name string
		if err = json.Unmarshal(row["Name"], &name); err != nil {
			t.Fatal(err)
		}
		if name == "sqlite_autoindex_t_1" {
			if len(row) != 6 || string(row["SQLNull"]) != "true" || string(row["SQL"]) != `""` || string(row["Type"]) != `"index"` || string(row["TableName"]) != `"t"` {
				t.Fatal("missing additive NULL discriminator", row)
			}
		} else if len(row) != 5 || row["SQLNull"] != nil {
			t.Fatal("ordinary plaintext inspect JSON changed", row)
		}
	}
	out.Reset()
	if err = run([]string{"query", path, "SELECT sql IS NULL,typeof(sql) FROM sqlite_schema WHERE type='index' AND name='sqlite_autoindex_t_1'"}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`{"row":[1,"null"]}`)) {
		t.Fatal("schema SQL is not a real NULL", out.String())
	}
	out.Reset()
	if err = run([]string{"check", path}, &out); err != nil || !bytes.Contains(out.Bytes(), []byte(`"indexes":2`)) {
		t.Fatal("CLI Check omitted implicit root", out.String(), err)
	}
	after, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("read-only metadata/check changed database", err)
	}
}
