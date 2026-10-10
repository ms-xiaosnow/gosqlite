//go:build oracle

package sql

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
)

// The audited runDiff helper checks the pinned official version/source, uses
// :memory: and waits for normal CLI completion. These are only pure scalar
// queries, without SQL writes or failure injection.
func TestExpressionReviewOracleNested(t *testing.T) {
	oracle := os.Getenv("SQLITE_ORACLE")
	if oracle == "" {
		t.Fatal("SQLITE_ORACLE required")
	}
	r := rand.New(rand.NewSource(0x5342494e))
	atoms := []string{"NULL", "0", "1", "2", "-7", "1.5", "'1'", "' 2x'", "X'32'", "''", "'中'"}
	ops := []string{"=", "!=", "<", ">=", "AND", "OR", "+", "-", "|", "||", "IS", "IS NOT"}
	types := []string{"INTEGER", "REAL", "NUMERIC", "BLOB", "TEXT", "FLOATING POINT", "'TEXT'", "[INTEGER]", "\"blob\"", "ınt"}
	var makeExpr func(int) string
	makeExpr = func(d int) string {
		if d == 0 || r.Intn(4) == 0 {
			return atoms[r.Intn(len(atoms))]
		}
		a := makeExpr(d - 1)
		switch r.Intn(7) {
		case 0:
			return "(CAST(" + a + " AS " + types[r.Intn(len(types))] + "))"
		case 1:
			return "(CASE " + a + " WHEN " + makeExpr(d-1) + " THEN " + makeExpr(d-1) + " ELSE " + makeExpr(d-1) + " END)"
		case 2:
			return "(CASE WHEN " + a + " THEN " + makeExpr(d-1) + " ELSE " + makeExpr(d-1) + " END)"
		case 3:
			return "(" + a + " BETWEEN " + makeExpr(d-1) + " AND " + makeExpr(d-1) + ")"
		case 4:
			return "(" + a + " NOT BETWEEN " + makeExpr(d-1) + " AND " + makeExpr(d-1) + ")"
		case 5:
			return "(" + a + " IN(" + makeExpr(d-1) + "," + makeExpr(d-1) + "," + makeExpr(d-1) + "))"
		default:
			return "(" + a + " " + ops[r.Intn(len(ops))] + " " + makeExpr(d-1) + ")"
		}
	}
	exprs := []string{}
	for range 2500 {
		exprs = append(exprs, makeExpr(3))
	}
	for _, op := range []string{"=", "!=", "<", ">=", "AND", "OR", "|", "IS", "IS NOT", "LIKE"} {
		for _, a := range []string{"0", "1", "2", "NULL"} {
			for _, b := range []string{"0", "1", "2", "NULL"} {
				if op != "OR" {
					exprs = append(exprs, fmt.Sprintf("%s BETWEEN %s %s 1 AND 2", a, b, op))
				}
				exprs = append(exprs, fmt.Sprintf("%s BETWEEN 0 AND %s %s 1", a, b, op), fmt.Sprintf("%s IN(0,1) %s %s", a, op, b), fmt.Sprintf("%s %s %s IN(0,1)", a, op, b))
			}
		}
	}
	runDiff(t, oracle, exprs, "", nil)
}

func TestExpressionReviewOracleCastTextPrefixes(t *testing.T) {
	r := rand.New(rand.NewSource(3540099))
	exprs := []string{}
	for range 800 {
		digits := make([]byte, 1+r.Intn(80))
		for i := range digits {
			digits[i] = byte('0' + r.Intn(10))
		}
		point := r.Intn(len(digits) + 1)
		s := string(digits[:point]) + "." + string(digits[point:]) + fmt.Sprintf("e%+d", r.Intn(801)-400)
		if r.Intn(2) == 0 {
			s = "-" + s
		}
		for _, typ := range []string{"INTEGER", "NUMERIC", "REAL"} {
			exprs = append(exprs, "CAST(' "+s+"trailer' AS "+typ+")")
		}
	}
	for _, typ := range []string{"STRICT", "WINDOW", "OVER", "FILTER"} {
		exprs = append(exprs, "CAST('1.5' AS "+typ+")")
	}
	runDiff(t, os.Getenv("SQLITE_ORACLE"), exprs, "", nil)
}

func TestExpressionReviewOracleCastKeywordGrammar(t *testing.T) {
	oracle := os.Getenv("SQLITE_ORACLE")
	version, err := exec.Command(oracle, "-batch", "-bail", ":memory:", "SELECT sqlite_version() || '|' || sqlite_source_id();").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != buildinfo.SQLiteBaseline+"|"+buildinfo.SQLiteSourceID {
		t.Fatalf("oracle identity: %s %v", version, err)
	}
	// Fixed names from the pinned public-domain tool/mkkeywordhash.c table,
	// plus STRICT/TRUE/FALSE, which are identifiers in this grammar context.
	names := strings.Fields(`ABORT ACTION ADD AFTER ALL ALTER ALWAYS ANALYZE AND AS ASC ATTACH AUTOINCREMENT BEFORE BEGIN BETWEEN BY CASCADE CASE CAST CHECK COLLATE COLUMN COMMIT CONFLICT CONSTRAINT CREATE CROSS CURRENT CURRENT_DATE CURRENT_TIME CURRENT_TIMESTAMP DATABASE DEFAULT DEFERRED DEFERRABLE DELETE DESC DETACH DISTINCT DO DROP END EACH ELSE ESCAPE EXCEPT EXCLUSIVE EXCLUDE EXISTS EXPLAIN FAIL FILTER FIRST FOLLOWING FOR FOREIGN FROM FULL GENERATED GLOB GROUP GROUPS HAVING IF IGNORE IMMEDIATE IN INDEX INDEXED INITIALLY INNER INSERT INSTEAD INTERSECT INTO IS ISNULL JOIN KEY LAST LEFT LIKE LIMIT MATCH MATERIALIZED NATURAL NO NOT NOTHING NOTNULL NULL NULLS OF OFFSET ON OR ORDER OTHERS OUTER OVER PARTITION PLAN PRAGMA PRECEDING PRIMARY QUERY RAISE RANGE RECURSIVE REFERENCES REGEXP REINDEX RELEASE RENAME REPLACE RESTRICT RETURNING RIGHT ROLLBACK ROW ROWS SAVEPOINT SELECT SET TABLE TEMP TEMPORARY THEN TIES TO TRANSACTION TRIGGER UNBOUNDED UNION UNIQUE UPDATE USING VACUUM VALUES VIEW VIRTUAL WHEN WHERE WINDOW WITH WITHIN WITHOUT STRICT TRUE FALSE`)
	for _, name := range names {
		q := "CAST('1.5' AS " + name + ")"
		// Each subprocess performs just one SELECT in memory and exits normally.
		out, wantErr := exec.Command(oracle, "-batch", "-bail", ":memory:", "SELECT "+q+";").CombinedOutput()
		_, gotErr := ParseExpr(q)
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("%s: native err=%v; SQLite err=%v output=%s", name, gotErr, wantErr, out)
		}
	}
	t.Logf("checked %d CAST typename keyword acceptance cases", len(names))
}
