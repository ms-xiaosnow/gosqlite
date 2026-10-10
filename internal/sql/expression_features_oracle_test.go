//go:build oracle

package sql

import (
	"fmt"
	"os"
	"testing"
)

// Independent pinned official SQLite, in memory only. The helper verifies
// sqlite_version and sqlite_source_id before comparing storage class and value.
func TestDiffSQLiteExpressionFeatures(t *testing.T) {
	oracle := os.Getenv("SQLITE_ORACLE")
	if oracle == "" {
		t.Fatal("SQLITE_ORACLE required")
	}
	atoms := []string{"NULL", "0", "1", "-1", "2", "1.5", "'1'", "'2'", "'junk'", "X'31'", "''"}
	exprs := []string{}
	for _, a := range atoms {
		exprs = append(exprs, a+" IN ()", a+" NOT IN ()")
		for _, b := range atoms {
			exprs = append(exprs, fmt.Sprintf("CASE %s WHEN %s THEN 7 ELSE 'no' END", a, b), fmt.Sprintf("CASE WHEN %s THEN %s ELSE NULL END", a, b), fmt.Sprintf("%s IN (%s)", a, b))
			for _, c := range atoms {
				exprs = append(exprs, fmt.Sprintf("%s BETWEEN %s AND %s", a, b, c), fmt.Sprintf("%s NOT BETWEEN %s AND %s", a, b, c), fmt.Sprintf("%s IN (%s,%s)", a, b, c), fmt.Sprintf("%s NOT IN (%s,%s)", a, b, c))
			}
		}
	}
	castAtoms := append(atoms, "1.0", "'123e+5'", "'1.0junk'", "'1.e'", "'-'", "'.'", "'0x12'", "'9223372036854775807'", "'9223372036854775808'", "'-9223372036854775809'", "'9007199254740993'", "'9007199254740993.0'", "'2251799813685247.0'", "'2251799813685248.0'", "'-2251799813685248.0'", "'-2251799813685249.0'", "1e999", "-1e999", "'1e-999'", "X'310032FF'", "-0.0")
	for _, a := range castAtoms {
		for _, typ := range []string{"INTEGER", "REAL", "NUMERIC", "TEXT", "BLOB", "VARCHAR(4)", "DECIMAL(10,2)", "FLOATING POINT", "NONE", "VECTOR(3)", `""`, `"NULL"`, "'TEXT'", "", `"x" INT`, "weird /*INT*/ BLOB", "END", "TRUE", "STRICT", "WINDOW", "OVER", "FILTER"} {
			exprs = append(exprs, fmt.Sprintf("CAST(%s AS %s)", a, typ))
		}
	}
	exprs = append(exprs, "absent IN ()", "absent NOT IN ()", "1 BETWEEN 0=0 AND 2", "1 BETWEEN 0 AND 0=0", "1 = 2 BETWEEN 0 AND 1", "1 IN (1,2) = 1", "NOT 1 IN (2,3)", "1 NOT BETWEEN 0 AND 2 AND 0", "1 BETWEEN 0 AND 2 OR 0", "2 BETWEEN 1 BETWEEN 0 AND 2 AND 3", "CASE WHEN 0 THEN abs(-9223372036854775808) ELSE 7 END", "abs(-9223372036854775808) IN ()")
	runDiff(t, oracle, exprs, "", nil)
}

func TestDiffSQLiteExpressionFeatureAffinity(t *testing.T) {
	oracle := os.Getenv("SQLITE_ORACLE")
	if oracle == "" {
		t.Fatal("SQLITE_ORACLE required")
	}
	ctx := &EvalContext{Row: map[string]any{"i": int64(12), "t": "12", "r": float64(12), "b": []byte("12")}, Affinity: func(_, n string) string {
		switch n {
		case "i":
			return "INTEGER"
		case "t":
			return "TEXT"
		case "r":
			return "REAL"
		}
		return "BLOB"
	}}
	exprs := []string{}
	atoms := []string{"i", "t", "r", "b", "12", "'12'", "CAST(12 AS TEXT)", "CAST('12' AS NUMERIC)", "+i", "NULL"}
	for _, a := range atoms {
		for _, b := range atoms {
			exprs = append(exprs, fmt.Sprintf("CASE %s WHEN %s THEN 1 ELSE 0 END", a, b), fmt.Sprintf("%s IN (%s)", a, b), fmt.Sprintf("%s IN (%s,NULL)", a, b))
			for _, c := range atoms {
				exprs = append(exprs, fmt.Sprintf("%s BETWEEN %s AND %s", a, b, c))
			}
		}
	}
	runDiff(t, oracle, exprs, "CREATE TABLE tab(i INTEGER,t TEXT,r REAL,b BLOB); INSERT INTO tab VALUES(12,'12',12,X'3132');", ctx)
}
