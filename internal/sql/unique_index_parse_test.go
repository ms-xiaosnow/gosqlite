package sql

import (
	"reflect"
	"strings"
	"testing"
)

func TestUniqueIndexParseAccepted(t *testing.T) {
	for _, tc := range []struct {
		name, query, index, table string
		columns                   []string
		desc                      []bool
		ifNotExists               bool
	}{
		{"single", "CREATE UNIQUE INDEX idx ON t(a)", "idx", "t", []string{"a"}, []bool{false}, false},
		{"case insensitive", "create unique index if not exists idx on t(a collate binary asc,b)", "idx", "t", []string{"a", "b"}, []bool{false, false}, true},
		{"quoted composite", `CREATE UNIQUE INDEX IF NOT EXISTS "unique index" ON "table name"("a""b" COLLATE BINARY ASC,"second field" ASC);`, "unique index", "table name", []string{`a"b`, "second field"}, []bool{false, false}, true},
		{"quoted keyword", `CREATE UNIQUE INDEX "unique" ON "index"("desc" COLLATE "BINARY")`, "unique", "index", []string{"desc"}, []bool{false}, false},
		{"descending is represented", "CREATE UNIQUE INDEX idx ON t(a DESC,b ASC)", "idx", "t", []string{"a", "b"}, []bool{true, false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stmt, err := Parse(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			def, ok := stmt.(*CreateIndex)
			if !ok {
				t.Fatalf("statement type = %T", stmt)
			}
			if !def.Unique || def.Name != tc.index || def.Table != tc.table || def.IfNotExists != tc.ifNotExists || !reflect.DeepEqual(def.Columns, tc.columns) || !reflect.DeepEqual(def.Desc, tc.desc) {
				t.Fatalf("unexpected declaration: %#v", def)
			}
			if def.SQL != tc.query || def.Method != "" || len(def.Options) != 0 || def.ParameterCount() != 0 {
				t.Fatalf("ordinary UNIQUE declaration changed metadata: %#v", def)
			}
		})
	}
}

func TestUniqueIndexParsePreservesOrdinary(t *testing.T) {
	for _, query := range []string{
		"CREATE INDEX idx ON t(a)",
		"CREATE INDEX IF NOT EXISTS idx ON t(a COLLATE BINARY ASC,b DESC)",
		"CREATE INDEX idx ON t(a) USING grams13",
		"CREATE INDEX idx ON t(a) USING hnsw WITH (metric='cosine')",
		"CREATE INDEX idx ON t(a) USING ivfflat WITH (lists=4)",
	} {
		stmt, err := Parse(query)
		if err != nil {
			t.Fatal(query, err)
		}
		def, ok := stmt.(*CreateIndex)
		if !ok || def.Unique || def.SQL != query {
			t.Fatalf("ordinary index grammar changed: %q: %#v", query, stmt)
		}
	}
}

func TestUniqueIndexParseRejectsUnsupported(t *testing.T) {
	for _, query := range []string{
		"CREATE TABLE t(a UNIQUE)",
		"CREATE TABLE t(a TEXT UNIQUE)",
		"CREATE TABLE t(a,b,UNIQUE(a,b))",
		"CREATE TABLE t(a,CONSTRAINT uq UNIQUE(a))",
		"CREATE UNIQUE TABLE t(a)",
		"CREATE UNIQUE idx ON t(a)",
		"CREATE UNIQUE UNIQUE INDEX idx ON t(a)",
		"CREATE UNIQUE INDEX idx ON t(a) USING grams13",
		"CREATE UNIQUE INDEX idx ON t(a) USING hnsw WITH (metric='cosine')",
		"CREATE UNIQUE INDEX idx ON t(a) USING ivfflat WITH (lists=4)",
		"CREATE UNIQUE INDEX idx ON t(a) USING custom_method",
		"CREATE UNIQUE INDEX idx ON t(lower(a))",
		"CREATE UNIQUE INDEX idx ON t(a+1)",
		"CREATE UNIQUE INDEX idx ON t((a))",
		"CREATE UNIQUE INDEX idx ON t(a) WHERE a IS NOT NULL",
		"CREATE UNIQUE INDEX idx ON t(a COLLATE NOCASE)",
		"CREATE UNIQUE INDEX idx ON t(a COLLATE RTRIM)",
		"CREATE UNIQUE INDEX idx ON t(a COLLATE)",
		"CREATE UNIQUE INDEX idx ON t(a) ON CONFLICT IGNORE",
		"CREATE UNIQUE INDEX idx ON t(a) WITH (x=1)",
		"CREATE UNIQUE INDEX idx ON t(a) trailing",
		"CREATE UNIQUE INDEX idx ON t(a); SELECT 1",
		"CREATE UNIQUE INDEX idx ON t(a);;",
		"CREATE UNIQUE INDEX IF EXISTS idx ON t(a)",
		"CREATE UNIQUE INDEX idx ON t()",
		"CREATE UNIQUE INDEX idx ON t(a,)",
		"CREATE UNIQUE INDEX idx ON t(?)",
	} {
		if stmt, err := Parse(query); err == nil {
			t.Errorf("accepted unsupported %q: %#v", query, stmt)
		}
	}
}

func TestUniqueIndexParseColumnBudget(t *testing.T) {
	// One fixed boundary on either side of MaxColumns, not an unbounded corpus.
	for _, count := range []int{MaxColumns, MaxColumns + 1} {
		query := "CREATE UNIQUE INDEX idx ON t(" + strings.Repeat("a,", count-1) + "a)"
		stmt, err := Parse(query)
		if count > MaxColumns {
			if err == nil {
				t.Fatal("accepted index beyond column admission limit")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		def := stmt.(*CreateIndex)
		if !def.Unique || len(def.Columns) != count || len(def.Desc) != count {
			t.Fatal("lost bounded UNIQUE column metadata", len(def.Columns), len(def.Desc))
		}
	}
}
