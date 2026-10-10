//go:build oracle

package testkit_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Native and official ordinary SQL on the same intact files. No fault,
// recovery, namespace mutation, process cancellation or disk tampering.
func TestOracleExpressionFeaturesQueries(t *testing.T) {
	confOracle(t)
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "expressions.db")
			db, err := sqlite.Open(path, sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			writeExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY,n INTEGER,t TEXT, r REAL, \"true\" INTEGER, d INTEGER DEFAULT (CASE WHEN 1 IN(1,2) THEN CAST('7' AS INTEGER) ELSE 0 END))")
			writeExec(t, db, "INSERT INTO e(id,n,t,r,\"true\") VALUES(1,12,'12',12,0),(2,NULL,'2',2,1),(3,2,'junk',NULL,NULL),(4,0,NULL,0,2),(5,12,'0012',12,0)")
			queries := []string{
				"SELECT ALL id,CASE TRUE WHEN 0 THEN 'zero' WHEN 1 THEN 'one' ELSE 'other' END,d FROM e ORDER BY id",
				"SELECT id,CASE n WHEN t THEN 'same' ELSE 'different' END,n IN(t),t IN(n), n BETWEEN t AND r FROM e ORDER BY id",
				"SELECT id,CAST(t AS INTEGER),CAST(t AS REAL),CAST(t AS NUMERIC),hex(CAST(n AS BLOB)),typeof(CAST(n AS TEXT)) FROM e ORDER BY id",
				"SELECT CAST(t AS INTEGER) AS chosen FROM e WHERE chosen='12' ORDER BY id",
				"SELECT id FROM e WHERE id NOT IN(2,NULL) ORDER BY id",
				"SELECT id FROM e WHERE id BETWEEN 2 AND 4 ORDER BY id",
				"SELECT id FROM e WHERE (CASE WHEN n IS NULL THEN 1 ELSE n=12 END) ORDER BY id",
				"SELECT id,n AS value FROM e ORDER BY value NULLS FIRST,id",
				"SELECT id,n AS value FROM e ORDER BY value NULLS LAST,id",
				"SELECT id,n FROM e ORDER BY 2 DESC NULLS FIRST,1",
				"SELECT id,n FROM e ORDER BY 2 DESC NULLS LAST,1",
				"SELECT id FROM e ORDER BY CASE WHEN t IN('12','0012') THEN NULL ELSE CAST(t AS INTEGER) END DESC NULLS FIRST,id LIMIT CAST('3' AS INTEGER) OFFSET CASE WHEN 1 THEN 1 END",
				"SELECT CASE WHEN 1 THEN 7 ELSE abs(-9223372036854775808) END FROM e WHERE id IN(1,3) ORDER BY id",
			}
			for _, q := range queries {
				writeSame(t, db, path, q)
			}
			writeExec(t, db, "UPDATE e SET t=CASE WHEN id BETWEEN 1 AND 3 THEN CAST(n AS TEXT) ELSE t END WHERE id IN(1,2,3,4)")
			writeExec(t, db, "DELETE FROM e WHERE CASE WHEN id NOT IN(1,2,3,4) THEN 1 ELSE 0 END")
			writeSame(t, db, path, "SELECT * FROM e ORDER BY id")
			writeIntegrity(t, path)
		})
	}
}
