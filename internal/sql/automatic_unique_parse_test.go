package sql

import (
	"reflect"
	"strings"
	"testing"
)

func TestAutomaticUniqueParseDeclarations(t *testing.T) {
	for _, tc := range []struct {
		query       string
		columns     int
		constraints []UniqueConstraint
	}{
		{"CREATE TABLE t(a UNIQUE)", 1, []UniqueConstraint{{Columns: []string{"a"}}}},
		{"CREATE TABLE t(a TEXT UNIQUE)", 1, []UniqueConstraint{{Columns: []string{"a"}}}},
		{"CREATE TABLE t(a,b,UNIQUE(a,b))", 2, []UniqueConstraint{{Columns: []string{"a", "b"}}}},
		{"CREATE TABLE t(a,CONSTRAINT uq UNIQUE(a))", 1, []UniqueConstraint{{Name: "uq", Columns: []string{"a"}}}},
		{`CREATE TABLE "表 名"("A" TEXT COLLATE "BINARY" CONSTRAINT "first label" UNIQUE UNIQUE,b NUMERIC,CONSTRAINT "second label" UNIQUE("a" COLLATE binary ASC,b ASC),UNIQUE(b,b))`, 2, []UniqueConstraint{{Name: "first label", Columns: []string{"A"}}, {Columns: []string{"A"}}, {Name: "second label", Columns: []string{"a", "b"}}, {Columns: []string{"b", "b"}}}},
		{"CREATE TABLE t(id INTEGER UNIQUE PRIMARY KEY,a DEFAULT 3 UNIQUE NOT NULL COLLATE BINARY,UNIQUE(id),UNIQUE(a,id))", 2, []UniqueConstraint{{Columns: []string{"id"}}, {Columns: []string{"a"}}, {Columns: []string{"id"}}, {Columns: []string{"a", "id"}}}},
		{`CREATE TABLE IF NOT EXISTS "unique"("desc" UNIQUE,"constraint",UNIQUE("constraint" COLLATE BINARY))`, 2, []UniqueConstraint{{Columns: []string{"desc"}}, {Columns: []string{"constraint"}}}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			stmt, err := Parse(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			def, ok := stmt.(*CreateTable)
			if !ok || len(def.Columns) != tc.columns || !reflect.DeepEqual(def.UniqueConstraints, tc.constraints) {
				t.Fatalf("lost ordered constraints: %#v", stmt)
			}
			if def.SQL != tc.query || def.ParameterCount() != 0 {
				t.Fatal("changed schema text or parameter contract", def)
			}
		})
	}
}

func TestAutomaticUniqueParseRejectsUnsupported(t *testing.T) {
	for _, query := range []string{
		"CREATE TABLE t(UNIQUE(a))", "CREATE TABLE t(a,UNIQUE())",
		"CREATE TABLE t(a,UNIQUE(a,))", "CREATE TABLE t(a,UNIQUE(a),b)",
		"CREATE TABLE t(a UNIQUE ASC)", "CREATE TABLE t(a UNIQUE DESC)",
		"CREATE TABLE t(a COLLATE NOCASE UNIQUE)", "CREATE TABLE t(a COLLATE RTRIM)",
		"CREATE TABLE t(a,UNIQUE(a DESC))", "CREATE TABLE t(a,UNIQUE(a COLLATE NOCASE))",
		"CREATE TABLE t(a,UNIQUE(a),UNIQUE(a DESC))",
		"CREATE TABLE t(a,UNIQUE(a),UNIQUE(a COLLATE RTRIM))",
		"CREATE TABLE t(a,UNIQUE(a NULLS FIRST))", "CREATE TABLE t(a,UNIQUE(a ASC NULLS LAST))",
		"CREATE TABLE t(a,UNIQUE(lower(a)))", "CREATE TABLE t(a,UNIQUE(a+1))",
		"CREATE TABLE t(a,UNIQUE((a)))", "CREATE TABLE t(a,UNIQUE(t.a))",
		"CREATE TABLE t(a,UNIQUE(a) WHERE a IS NOT NULL)",
		"CREATE TABLE t(a CONSTRAINT named NOT NULL)", "CREATE TABLE t(a CONSTRAINT named)",
		"CREATE TABLE t(a,CONSTRAINT named PRIMARY KEY(a))", "CREATE TABLE t(a,PRIMARY KEY(a))",
		"CREATE TABLE t(a INTEGER PRIMARY KEY DESC UNIQUE)",
		"CREATE TABLE t(a INTEGER PRIMARY KEY AUTOINCREMENT UNIQUE)",
		"CREATE TABLE t(a UNIQUE) WITHOUT ROWID", "CREATE TABLE t(a UNIQUE) STRICT",
		"CREATE TABLE t(a UNIQUE CHECK(abs(a)>0))", "CREATE TABLE t(a UNIQUE REFERENCES other(a))",
		"CREATE TABLE t(a UNIQUE,b GENERATED ALWAYS AS (a+1))", "CREATE TABLE t AS SELECT 1",
		"CREATE TABLE t(a UNIQUE USING grams13)", "CREATE TABLE t(a,UNIQUE(a) USING hnsw)",
		"INSERT OR IGNORE INTO t VALUES(1)", "UPDATE OR REPLACE t SET a=1",
		"INSERT INTO t VALUES(1) ON CONFLICT DO NOTHING", "DROP INDEX sqlite_autoindex_t_1",
		"DROP INDEX IF EXISTS sqlite_autoindex_t_1", "DROP TABLE t", "ALTER TABLE t RENAME TO u",
		"CREATE TABLE t(a,UNIQUE(a); SELECT 1)",
	} {
		if stmt, err := Parse(query); err == nil {
			t.Errorf("accepted unsupported %q: %#v", query, stmt)
		}
	}
	for _, policy := range []string{"ABORT", "FAIL", "IGNORE", "REPLACE", "ROLLBACK"} {
		for _, query := range []string{
			"CREATE TABLE t(a UNIQUE ON CONFLICT " + policy + ")",
			"CREATE TABLE t(a,UNIQUE(a) ON CONFLICT " + policy + ")",
			"CREATE TABLE t(a UNIQUE,UNIQUE(a) ON CONFLICT " + policy + ")",
		} {
			if stmt, err := Parse(query); err == nil {
				t.Errorf("accepted explicit conflict policy %q: %#v", query, stmt)
			}
		}
	}
}

func TestAutomaticUniqueParseBudgets(t *testing.T) {
	for _, count := range []int{MaxUniqueConstraints, MaxUniqueConstraints + 1} {
		query := "CREATE TABLE t(a" + strings.Repeat(" UNIQUE", count) + ")"
		stmt, err := Parse(query)
		if count > MaxUniqueConstraints {
			if err == nil {
				t.Fatal("constraint declarations were truncated or coalesced before admission")
			}
		} else if err != nil || len(stmt.(*CreateTable).UniqueConstraints) != count {
			t.Fatal("exact declaration bound", err)
		}
	}
	for _, count := range []int{MaxColumns, MaxColumns + 1} {
		query := "CREATE TABLE t(a,UNIQUE(" + strings.Repeat("a,", count-1) + "a))"
		stmt, err := Parse(query)
		if count > MaxColumns {
			if err == nil {
				t.Fatal("accepted oversized UNIQUE key")
			}
		} else if err != nil || len(stmt.(*CreateTable).UniqueConstraints[0].Columns) != count {
			t.Fatal("exact key bound", err)
		}
	}
	for _, count := range []int{MaxUniqueTerms, MaxUniqueTerms + 1} {
		var query strings.Builder
		query.WriteString("CREATE TABLE t(a")
		for remaining := count; remaining > 0; {
			n := min(remaining, MaxColumns)
			query.WriteString(",UNIQUE(" + strings.Repeat("a,", n-1) + "a)")
			remaining -= n
		}
		query.WriteByte(')')
		stmt, err := Parse(query.String())
		if count > MaxUniqueTerms {
			if err == nil {
				t.Fatal("accepted excessive total key terms")
			}
		} else {
			if err != nil {
				t.Fatal("exact total-term bound", err)
			}
			terms := 0
			for _, u := range stmt.(*CreateTable).UniqueConstraints {
				terms += len(u.Columns)
			}
			if terms != count {
				t.Fatal("key terms lost before semantic coalescing", terms)
			}
		}
	}
}
