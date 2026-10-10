//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// All fixtures are intact databases made through ordinary SQL. The pinned CLI
// completes normally; these tests do not inject storage or process failures.
func relationalOracleFixture(t *testing.T, shared bool) (*sqlite.DB, string) {
	t.Helper()
	confOracle(t)
	path := filepath.Join(t.TempDir(), "relations.db")
	db, err := sqlite.Open(path, sqlite.Options{WALShared: shared, PageSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, q := range []string{
		"CREATE TABLE lhs(lid INTEGER PRIMARY KEY,k NUMERIC,txt TEXT,r REAL,payload BLOB,flag INTEGER)",
		"CREATE TABLE rhs(rid INTEGER PRIMARY KEY,k TEXT,tag TEXT,score REAL,payload BLOB)",
		"CREATE TABLE third(cid INTEGER PRIMARY KEY,rid INTEGER,caption TEXT)",
		"CREATE TABLE empty_source(eid INTEGER PRIMARY KEY,k TEXT)",
		"CREATE TABLE shadows(id INTEGER PRIMARY KEY,rowid TEXT,_rowid_ TEXT,oid TEXT)",
		"CREATE TABLE plain(value TEXT)",
		"INSERT INTO lhs VALUES(1,1,'01',1,x'00ff',1),(2,1,'1',2,x'',0),(3,NULL,NULL,NULL,NULL,NULL),(4,2,'2',2.5,x'32',1),(5,3,'3',3,x'33',0),(6,'word','word',6,x'31',1)",
		"INSERT INTO rhs VALUES(10,'1','first',1,x'00ff'),(11,'01','duplicate',2,x''),(12,NULL,'null',NULL,NULL),(13,'2','two',2.5,x'32'),(14,'other','unmatched',4,x'34'),(15,'word','word',6,x'31'),(16,x'31','blob',1,x'31')",
		"INSERT INTO third VALUES(100,10,'one'),(101,10,'again'),(102,13,'two'),(103,99,'unused')",
		"INSERT INTO shadows VALUES(7,'public rowid','public _rowid_','public oid')",
		"INSERT INTO plain VALUES('plain one'),('plain two')",
	} {
		writeExec(t, db, q)
	}
	return db, path
}

func TestRelationalOracleJoins(t *testing.T) {
	cases := []confCase{
		{name: "inner_duplicates", query: "SELECT a.lid,b.rid,a.k,b.k FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "explicit_inner", query: "SELECT a.lid,b.rid FROM lhs AS a INNER JOIN rhs AS b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "reversed_affinity", query: "SELECT a.lid,b.rid,a.k=b.k,b.k=a.k FROM lhs a JOIN rhs b ON b.k=a.k ORDER BY a.lid,b.rid"},
		{name: "unary_removes_affinity", query: "SELECT a.lid,b.rid,+a.k=b.k,a.k=+b.k,+a.k=+b.k FROM lhs a CROSS JOIN rhs b ORDER BY a.lid,b.rid"},
		{name: "comma_where", query: "SELECT a.lid,b.rid FROM lhs a,rhs b WHERE a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "cross_where", query: "SELECT a.lid,b.rid FROM lhs a CROSS JOIN rhs b WHERE a.lid<3 AND b.rid<13 ORDER BY a.lid,b.rid"},
		{name: "cross_on", query: "SELECT a.lid,b.rid FROM lhs a CROSS JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "comma_on", query: "SELECT a.lid,b.rid FROM lhs a,rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "join_without_on", query: "SELECT a.lid,b.rid FROM lhs a JOIN rhs b ORDER BY a.lid,b.rid LIMIT 5 OFFSET 3"},
		{name: "left_duplicates_and_null", query: "SELECT a.lid,b.rid,a.k,b.k,typeof(b.score) FROM lhs a LEFT JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "left_outer", query: "SELECT a.lid,b.rid FROM lhs a LEFT OUTER JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "null_is_match", query: "SELECT a.lid,b.rid FROM lhs a LEFT JOIN rhs b ON a.k IS b.k ORDER BY a.lid,b.rid"},
		{name: "null_on", query: "SELECT a.lid,b.rid,b.rowid FROM lhs a LEFT JOIN rhs b ON NULL ORDER BY a.lid"},
		{name: "on_filter_before_extension", query: "SELECT a.lid,b.rid FROM lhs a LEFT JOIN rhs b ON a.k=b.k AND b.rid=10 ORDER BY a.lid,b.rid"},
		{name: "where_filter_after_extension", query: "SELECT a.lid,b.rid FROM lhs a LEFT JOIN rhs b ON a.k=b.k WHERE b.rid=10 ORDER BY a.lid,b.rid"},
		{name: "where_null_finds_unmatched", query: "SELECT a.lid,b.rid FROM lhs a LEFT JOIN rhs b ON a.k=b.k WHERE b.rid IS NULL ORDER BY a.lid"},
		{name: "three_way_inner", query: "SELECT a.lid,b.rid,c.cid FROM lhs a JOIN rhs b ON a.k=b.k JOIN third c ON b.rid=c.rid ORDER BY a.lid,b.rid,c.cid"},
		{name: "three_way_left", query: "SELECT a.lid,b.rid,c.cid,c.caption FROM lhs a LEFT JOIN rhs b ON a.k=b.k LEFT JOIN third c ON b.rid=c.rid ORDER BY a.lid,b.rid,c.cid"},
		{name: "mixed_comma_left_associative", query: "SELECT a.lid,b.rid,c.cid FROM lhs a,rhs b LEFT JOIN third c ON c.rid=b.rid WHERE a.lid=1 AND b.rid<14 ORDER BY a.lid,b.rid,c.cid"},
		{name: "third_references_first", query: "SELECT a.lid,b.rid,c.cid FROM lhs a JOIN rhs b ON a.k=b.k JOIN third c ON c.rid=b.rid AND a.flag=1 ORDER BY a.lid,b.rid,c.cid"},
		{name: "star_duplicate_labels", query: "SELECT * FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "qualified_stars", query: "SELECT b.*,a.*,b.rid AS rid FROM lhs a LEFT JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "duplicate_alias_disjoint_qualified_star", query: "SELECT x.* FROM lhs x,third x ORDER BY lid,cid"},
		{name: "duplicate_alias_disjoint_unqualified_star", query: "SELECT * FROM lhs x,third x ORDER BY lid,cid"},
		{name: "label_spellings", query: "SELECT a.LID,b.RID,a.lid+1,a.txt AS \"Mixed Case\",a.lid AS same,b.rid AS same FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "rowid_aliases", query: "SELECT a.rowid,a._rowid_,a.oid,b.rowid,b._rowid_,b.oid FROM lhs a LEFT JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "rowid_shadows", query: "SELECT s.id,s.rowid,s._rowid_,s.oid,p.rowid,p._rowid_,p.oid FROM shadows s CROSS JOIN plain p ORDER BY p.rowid"},
		{name: "null_rowid_shadows", query: "SELECT a.lid,s.*,s.rowid,s._rowid_,s.oid FROM lhs a LEFT JOIN shadows s ON a.lid=s.id ORDER BY a.lid"},
		{name: "where_output_alias", query: "SELECT a.lid+10 AS shifted,b.rid FROM lhs a JOIN rhs b ON a.k=b.k WHERE shifted=11 ORDER BY b.rid"},
		{name: "on_output_alias", query: "SELECT a.lid+9 AS target,b.rid FROM lhs a LEFT JOIN rhs b ON b.rid=target ORDER BY a.lid,b.rid"},
		{name: "where_physical_before_alias", query: "SELECT -a.lid AS lid,b.rid FROM lhs a JOIN rhs b ON a.k=b.k WHERE lid=1 ORDER BY b.rid"},
		{name: "on_physical_before_alias", query: "SELECT a.lid+100 AS lid,b.rid FROM lhs a LEFT JOIN rhs b ON lid=1 AND b.rid=10 ORDER BY a.lid,b.rid"},
		{name: "order_bare_alias_first", query: "SELECT -a.lid AS lid,b.rid FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY lid,b.rid"},
		{name: "order_expression_physical_first", query: "SELECT -a.lid AS lid,b.rid FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY lid+0,b.rid"},
		{name: "order_alias_expression", query: "SELECT -a.lid AS backwards,b.rid FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY backwards+0,b.rid"},
		{name: "order_alias_over_ambiguous_physical", query: "SELECT a.lid AS k,b.rid FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY k,b.rid"},
		{name: "storage_classes", query: "SELECT a.lid,b.rid,a.r,b.score,a.payload,b.payload,typeof(a.r),typeof(b.score),typeof(a.payload),typeof(b.payload) FROM lhs a LEFT JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "ordinal_nulls_limit", query: "SELECT a.lid,b.rid,b.score FROM lhs a LEFT JOIN rhs b ON a.k=b.k ORDER BY 3 DESC NULLS FIRST,1,2 LIMIT 5 OFFSET 1"},
		{name: "empty_right_inner", query: "SELECT a.lid,e.eid FROM lhs a JOIN empty_source e ON a.k=e.k", emptyColumns: []string{"lid", "eid"}},
		{name: "empty_right_left", query: "SELECT a.lid,e.*,typeof(e.eid),e.rowid FROM lhs a LEFT JOIN empty_source e ON a.k=e.k ORDER BY a.lid"},
		{name: "empty_left", query: "SELECT e.eid,b.rid FROM empty_source e LEFT JOIN rhs b ON e.k=b.k", emptyColumns: []string{"eid", "rid"}},
		{name: "both_empty", query: "SELECT a.eid,b.eid FROM empty_source a CROSS JOIN empty_source b", emptyColumns: []string{"eid", "eid"}},
		{name: "zero_limit_labels", query: "SELECT a.*,b.* FROM lhs a LEFT JOIN rhs b ON a.k=b.k LIMIT 0", emptyColumns: []string{"lid", "k", "txt", "r", "payload", "flag", "rid", "k", "tag", "score", "payload"}},
		{name: "lazy_projection_case", query: "SELECT a.lid,b.rid,CASE WHEN b.rid IS NULL THEN 'missing' ELSE CASE WHEN b.rid>0 THEN b.tag ELSE abs(-9223372036854775808) END END AS chosen FROM lhs a LEFT JOIN rhs b ON a.k=b.k ORDER BY a.lid,b.rid"},
		{name: "lazy_on_case", query: "SELECT a.lid,b.rid FROM lhs a LEFT JOIN rhs b ON CASE WHEN a.lid>0 THEN a.k=b.k ELSE abs(-9223372036854775808) END ORDER BY a.lid,b.rid"},
		{name: "lazy_where_case", query: "SELECT a.lid,b.rid FROM lhs a JOIN rhs b ON a.k=b.k WHERE CASE WHEN b.rid>0 THEN 1 ELSE abs(-9223372036854775808) END ORDER BY a.lid,b.rid"},
		{name: "empty_skips_projection_error", query: "SELECT abs(-9223372036854775808) AS bad FROM empty_source a CROSS JOIN rhs b", emptyColumns: []string{"bad"}},
		{name: "empty_skips_on_error", query: "SELECT a.lid,e.eid FROM lhs a LEFT JOIN empty_source e ON abs(-9223372036854775808) ORDER BY a.lid"},
		{name: "parameters", query: "SELECT a.lid,b.rid FROM lhs a LEFT JOIN rhs b ON a.k=b.k AND b.rid>=? WHERE a.lid<=? ORDER BY a.lid,b.rid LIMIT ?", oracleQuery: "SELECT a.lid,b.rid FROM lhs a LEFT JOIN rhs b ON a.k=b.k AND b.rid>=11 WHERE a.lid<=4 ORDER BY a.lid,b.rid LIMIT 4", args: []any{11, 4, 4}},
	}
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared_%t", shared), func(t *testing.T) {
			db, path := relationalOracleFixture(t, shared)
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Logf("SQL: %s", tc.query)
					query := tc.oracleQuery
					if query == "" {
						query = tc.query
					}
					want := confExpected(t, path, query)
					if want.columns == nil {
						want.columns = tc.emptyColumns
					}
					confCompare(t, want, confNative(t, db, tc.query, tc.args...))
				})
			}
			writeIntegrity(t, path)
		})
	}
}

func TestRelationalOracleAddedDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "defaults.db")
	confCLI(t, path, `CREATE TABLE changed(id INTEGER PRIMARY KEY);
INSERT INTO changed VALUES(1);
ALTER TABLE changed ADD COLUMN text_default TEXT DEFAULT 'hello';
ALTER TABLE changed ADD COLUMN numeric_default NUMERIC DEFAULT '12';
ALTER TABLE changed ADD COLUMN real_default REAL DEFAULT 7;
INSERT INTO changed VALUES(2,'world',3,4);
CREATE TABLE keys(id INTEGER PRIMARY KEY);
INSERT INTO keys VALUES(1),(2),(3);`, false)
	defer confUnchanged(t, path)()
	confRun(t, path, []confCase{
		{name: "inner_defaults", query: "SELECT k.id,c.*,typeof(c.text_default),typeof(c.numeric_default),typeof(c.real_default) FROM keys k JOIN changed c ON k.id=c.id ORDER BY k.id"},
		{name: "left_defaults", query: "SELECT k.id,c.*,typeof(c.text_default),typeof(c.numeric_default),typeof(c.real_default) FROM keys k LEFT JOIN changed c ON k.id=c.id ORDER BY k.id"},
		{name: "defaults_in_on", query: "SELECT k.id,c.id FROM keys k LEFT JOIN changed c ON c.numeric_default='12' AND c.real_default='7' ORDER BY k.id,c.id"},
	})
}

func TestRelationalOracleBindingAndEvaluationErrors(t *testing.T) {
	db, path := relationalOracleFixture(t, false)
	for _, tc := range []struct{ query, category string }{
		{"SELECT k FROM lhs a JOIN rhs b ON a.k=b.k", "ambiguous column name"},
		{"SELECT a.lid AS k FROM lhs a JOIN rhs b ON a.k=b.k WHERE k=1", "ambiguous column name"},
		{"SELECT a.lid AS k FROM lhs a JOIN rhs b ON a.k=b.k ORDER BY k+0", "ambiguous column name"},
		{"SELECT rowid FROM lhs a JOIN rhs b ON a.k=b.k", "ambiguous column name"},
		{"SELECT lhs.lid FROM lhs a JOIN rhs b ON a.k=b.k", "no such column"},
		{"SELECT missing.* FROM lhs a JOIN rhs b ON a.k=b.k", "no such table"},
		{"SELECT x.* FROM lhs x,rhs x", "ambiguous column name"},
		{"SELECT * FROM lhs x,rhs x", "ambiguous column name"},
		{"SELECT a.lid FROM lhs a JOIN plain b ON k=1 JOIN rhs c ON c.k=a.k", "ambiguous column name"},
		{"SELECT a.eid FROM empty_source a JOIN plain b ON k=1 JOIN rhs c ON c.k=a.k", "ambiguous column name"},
		{"SELECT a.lid FROM lhs a JOIN rhs b ON missing=1", "no such column"},
		{"SELECT a.eid FROM empty_source a JOIN rhs b ON missing=1", "no such column"},
		{"SELECT a.eid,abs() FROM empty_source a CROSS JOIN rhs b", "wrong number of arguments"},
		{"SELECT a.lid FROM lhs a JOIN rhs b ON abs(-9223372036854775808)", "integer overflow"},
		{"SELECT abs(-9223372036854775808) FROM lhs a CROSS JOIN rhs b", "integer overflow"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			// The pinned shell reports these SQLITE_ERROR categories as text,
			// without printing the numeric primary code in this invocation mode.
			out, err := exec.Command(confOracle(t), "-readonly", "-batch", "-bail", path, tc.query).CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.category) {
				t.Fatalf("oracle should report %q: %v: %s", tc.category, err, out)
			}
			rows, err := db.Query(context.Background(), tc.query)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
				}
				err = rows.Err()
			}
			var detail *sqlite.Error
			if !errors.As(err, &detail) || detail.Code != sqlite.CodeError {
				t.Fatalf("native error = %v; want SQLITE_ERROR", err)
			}
		})
	}
}

func TestRelationalOracleBooleanNamesAndOrdinals(t *testing.T) {
	db, path := relationalOracleFixture(t, false)
	writeExec(t, db, `CREATE TABLE boolnames(id INTEGER PRIMARY KEY,"true" INTEGER,"false" INTEGER)`)
	writeExec(t, db, `INSERT INTO boolnames VALUES(1,9,2),(2,3,8),(3,6,5)`)
	for _, from := range []string{"boolnames a", "boolnames a,plain b WHERE b.rowid=1"} {
		for _, order := range []string{"true", "false", "+true", "(true) DESC", "-false"} {
			writeSame(t, db, path, "SELECT a.id FROM "+from+" ORDER BY "+order)
		}
	}
	writeSame(t, db, path, `SELECT TRUE,FALSE FROM boolnames a,plain b WHERE b.rowid=1 ORDER BY a.id`)
	writeSame(t, db, path, `SELECT (TRUE),(FALSE) FROM boolnames a,plain b WHERE b.rowid=1 ORDER BY a.id`)
	for _, from := range []string{"lhs a", "lhs a,plain b WHERE b.rowid=1"} {
		for _, key := range []string{"TRUE", "FALSE", "+TRUE", "-FALSE"} {
			writeSame(t, db, path, "SELECT -a.lid FROM "+from+" ORDER BY "+key)
		}
		for _, key := range []string{"true", "false"} {
			writeSame(t, db, path, "SELECT -a.lid AS \""+key+"\" FROM "+from+" ORDER BY "+key)
		}
	}
}

func TestRelationalOracleBooleanResultAliases(t *testing.T) {
	db, path := relationalOracleFixture(t, false)
	for _, name := range []string{"true", "false"} {
		writeSame(t, db, path, `SELECT a.lid-1 AS "`+name+`" FROM lhs a WHERE `+name+` ORDER BY a.lid`)
		writeSame(t, db, path, `SELECT -a.lid AS "`+name+`" FROM lhs a ORDER BY +`+name)
		writeSame(t, db, path, `SELECT -a.lid AS "`+name+`" FROM lhs a ORDER BY `+name+`+0`)
		writeSame(t, db, path, `SELECT 1 AS "`+name+`" WHERE `+name)

		writeSame(t, db, path, `SELECT a.lid-1 AS "`+name+`",b.rid FROM lhs a,rhs b WHERE `+name+` AND a.k=b.k ORDER BY a.lid,b.rid`)
		writeSame(t, db, path, `SELECT a.lid-1 AS "`+name+`",b.rid FROM lhs a JOIN rhs b ON `+name+` AND a.k=b.k ORDER BY a.lid,b.rid`)
	}
}
