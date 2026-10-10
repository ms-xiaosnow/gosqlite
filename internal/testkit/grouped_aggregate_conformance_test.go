//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"os/exec"
	"strings"
	"testing"
)

// Ordinary SQL-created intact fixtures and completed read-only pinned CLI
// comparisons only; no physical mutations or failure injection.
func TestGroupedAggregateOracleValues(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	confCLI(t, path, `CREATE TABLE g(id INTEGER PRIMARY KEY,k,v,t TEXT,n NUMERIC,r REAL);
INSERT INTO g VALUES(1,NULL,2,'1',1,1),(2,NULL,NULL,'1',1,1),(3,1,3,'1',1,1),(4,1.0,4,'1',1,1),(5,'1',5,'01',1,1),(6,x'31',6,'1.0',1,1),(7,2,7,'2',2,2),(8,2.0,8,'2',2,2),(9,9007199254740992,9,'3',3,3),(10,9007199254740993,10,'3',3,3),(11,9007199254740992.0,11,'4',4,4),(12,9223372036854775807,12,'4',4,4),(13,9223372036854775808.0,13,'4',4,4);
CREATE TABLE true(false); INSERT INTO true VALUES(1),(1),(2);`, false)
	queries := []string{
		"SELECT k,count(*),sum(v),total(v),avg(v),min(v),max(v) FROM g GROUP BY k",
		"SELECT k,typeof(k),count(*) FROM g GROUP BY k",
		"SELECT k,count(*) FROM g GROUP BY k HAVING count(*)>1 ORDER BY k DESC",
		"SELECT k,typeof(k),count(*) FROM g GROUP BY k HAVING typeof(k)='integer' ORDER BY k",
		"SELECT k,typeof(k),count(*),max(v) FROM g GROUP BY k HAVING typeof(k)='integer' ORDER BY k",
		"SELECT k,typeof(k),min(v),max(v),min(v) FROM g GROUP BY k ORDER BY k",
		"SELECT k,typeof(k),max(v),min(v),max(v) FROM g GROUP BY k ORDER BY k",
		"SELECT k,typeof(k),count(*) FROM g GROUP BY k HAVING min(v)>0 ORDER BY max(v)",
		"SELECT k,count(*) AS cnt FROM g GROUP BY k HAVING cnt>1 ORDER BY cnt DESC,k",
		"SELECT k+0 AS gkey,count(*) FROM g GROUP BY gkey ORDER BY gkey",
		"SELECT k+0,typeof(k+0),count(*) FROM g GROUP BY k+0 ORDER BY 1",
		"SELECT t,count(*) FROM g GROUP BY t HAVING t=1 ORDER BY t",
		"SELECT n,count(*) FROM g GROUP BY n HAVING n='1' ORDER BY n",
		"SELECT r,typeof(r),count(*) FROM g GROUP BY r HAVING r='1' ORDER BY r",
		"SELECT CAST(k AS TEXT),count(*) FROM g GROUP BY CAST(k AS TEXT) ORDER BY 1",
		"SELECT CAST(k AS BLOB),count(*) FROM g GROUP BY 1 ORDER BY 1",
		"SELECT k,count(*) FROM g GROUP BY k ORDER BY sum(v) DESC,k LIMIT 3 OFFSET 2",
		"SELECT k,count(*) FROM g GROUP BY k ORDER BY 2 DESC,1 NULLS LAST LIMIT 3,4",
		"SELECT k,count(*) FROM g WHERE id>2 GROUP BY k HAVING sum(v)>5 ORDER BY count(*)+1,k",
		"SELECT k FROM g GROUP BY k ORDER BY k",
		"SELECT k,count(*) FROM g WHERE 0 GROUP BY k HAVING count(*)>0",
		"SELECT 7,count(*) GROUP BY 1 HAVING count(*)=1",
		"SELECT count(*) FROM g HAVING count(*)>1",
		"SELECT count(*) AS cnt FROM g HAVING cnt=13",
		"SELECT count(*) FROM empty_source HAVING count(*)=0",
		"SELECT count(*) FROM empty_source HAVING count(*)>0",
		"SELECT a.x,count(*),sum(b.id) FROM a LEFT JOIN b ON a.x=b.x GROUP BY a.x HAVING count(*)>1 ORDER BY a.x",
		"SELECT a.x,count(*),count(b.id) FROM a LEFT JOIN b ON 0 GROUP BY a.x ORDER BY a.x",
		"SELECT a.id%2,b.x,count(*) FROM a,b GROUP BY a.id%2,b.x ORDER BY 1,2",
		"SELECT false,count(*) FROM true GROUP BY false HAVING false>0 ORDER BY false",
		"SELECT false+1 AS true,count(*) AS false FROM true GROUP BY true HAVING count(*)>0 ORDER BY true",
	}
	for _, q := range queries {
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
	for _, key := range []string{"k", "t", "n", "r", "k+0", "CAST(k AS TEXT)", "typeof(k)", "CASE WHEN k IS NULL THEN x'00' ELSE k END"} {
		var cols []string
		for _, fn := range []string{"count", "sum", "total", "avg", "min", "max"} {
			call := fn + "(v)"
			cols = append(cols, call, "typeof("+call+")")
		}
		q := fmt.Sprintf("SELECT %s,%s FROM g GROUP BY %s ORDER BY 1", key, strings.Join(cols, ","), key)
		t.Run(key, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
}

func TestGroupedAggregateOraclePromotedCapabilities(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	confCLI(t, path, `CREATE TABLE t(x); INSERT INTO t VALUES(NULL),(1),(1),(2);
 CREATE TABLE false(true); INSERT INTO false VALUES(8),(8),(3);`, false)
	for _, q := range []string{"SELECT sum(x) FROM t GROUP BY x", "SELECT count(*) FROM t HAVING count(*)>0", "SELECT x FROM t GROUP BY x", "SELECT true FROM false GROUP BY true"} {
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
}
func TestGroupedAggregateOracleKeyRepresentations(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	confCLI(t, path, `CREATE TABLE keys(id INTEGER PRIMARY KEY,k,v,w);
 INSERT INTO keys VALUES(1,1,2,4),(2,1.0,3,2),(3,1,1,3),(4,0.0,NULL,NULL),(5,0,NULL,NULL),(6,-0.0,NULL,NULL),(7,'1',3,2),(8,x'31',3,2),(9,NULL,4,2),(10,NULL,5,3),(11,CAST(x'610062' AS TEXT),6,1),(12,CAST(x'610063' AS TEXT),7,0),(13,x'610062',8,1),(14,x'',9,1),(15,'',10,1);`, false)
	for _, key := range []string{"k", "k+0", "+k", "CASE WHEN k IS NULL THEN 1 ELSE k END", "CAST(k AS NUMERIC)", "k,k+0"} {
		for _, calls := range []string{"count(*)", "min(v)", "max(v)", "min(v),max(v),min(v)", "max(v),min(v),max(v)", "min(v),min(w)", "CASE WHEN count(*)>1 THEN min(v) ELSE max(v) END"} {
			q := fmt.Sprintf("SELECT %s,%s FROM keys WHERE id NOT IN(11,12) GROUP BY %s ORDER BY 1", key, calls, key)
			t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
		}
	}
	for _, q := range []string{
		"SELECT typeof(k),hex(k),count(*),min(v),max(v) FROM keys GROUP BY k ORDER BY k",
		"SELECT typeof(k),hex(k),count(*) FROM keys GROUP BY k HAVING typeof(k)='text' ORDER BY k",
		"SELECT k,typeof(k),count(*) FROM keys WHERE id NOT IN(11,12) GROUP BY k HAVING typeof(k)='integer'",
		"SELECT k,typeof(k),count(*),max(v) FROM keys WHERE id NOT IN(11,12) GROUP BY k HAVING count(*)>1 AND typeof(k)='real'",
		"SELECT k,typeof(k),count(*) FROM keys WHERE id NOT IN(11,12) GROUP BY k ORDER BY max(v),min(w)",
		"SELECT k,typeof(k),max(v) FROM keys WHERE id NOT IN(11,12) GROUP BY k HAVING min(w)>=0 ORDER BY min(v)",
		"SELECT k+0,typeof(k+0),count(*) FROM keys WHERE id NOT IN(11,12) GROUP BY k+0 HAVING typeof(k+0)='integer'",
		"SELECT k,k+0,typeof(k),typeof(k+0),max(v) FROM keys WHERE id NOT IN(11,12) GROUP BY k,k+0",
		"SELECT k,CASE WHEN count(*)>1 THEN min(v) ELSE max(v) END,typeof(k) FROM keys WHERE id NOT IN(11,12) GROUP BY k",
	} {
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
}

func TestGroupedAggregateOracleErrorTimingAndBounds(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	confCLI(t, path, `CREATE TABLE group_errors(k,v); INSERT INTO group_errors VALUES(1,2),(2,-9223372036854775808),(3,3);
 CREATE TABLE group_overflow(k,v); INSERT INTO group_overflow VALUES(1,1),(2,9223372036854775807),(2,1);`, false)
	for _, q := range []string{
		"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k LIMIT 1",
		"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k ORDER BY k DESC LIMIT 1",
		"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k HAVING k=1",
		"SELECT k,sum(v) FROM group_overflow GROUP BY k LIMIT 1",
		"SELECT k,count(*)+abs(-9223372036854775808) FROM group_errors GROUP BY k LIMIT 1 OFFSET 9",
		"SELECT k,CASE WHEN k=1 THEN abs(-9223372036854775808) ELSE count(*) END FROM group_errors GROUP BY k LIMIT 1 OFFSET 1",
		"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k LIMIT 0 OFFSET NULL",
		"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k LIMIT 0 OFFSET abs(-9223372036854775808)",
		"SELECT x,count(*) FROM empty_source GROUP BY x HAVING count(*)>0",
		"SELECT x,count(*) FROM empty_source GROUP BY x HAVING 0",
		"SELECT count(*)+abs(-9223372036854775808) FROM a HAVING count(*)=0",
		"SELECT count(*)+abs(-9223372036854775808) FROM a HAVING count(*)>0 LIMIT 1 OFFSET 1",
	} {
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
	for _, tc := range []struct{ q, part string }{
		{"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k", "integer overflow"},
		{"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k HAVING count(*)=0", "integer overflow"},
		{"SELECT k,sum(v) FROM group_overflow GROUP BY k HAVING 0", "integer overflow"},
		{"SELECT k,sum(abs(v)) FROM group_errors GROUP BY k ORDER BY k+0 DESC LIMIT 1", "integer overflow"},
		{"SELECT x,count(*) FROM empty_source GROUP BY x HAVING abs(-9223372036854775808)", "integer overflow"},
		{"SELECT count(*) FROM a GROUP BY missing LIMIT 0", "no such column"},
		{"SELECT count(*) FROM a GROUP BY count(*) LIMIT 0", "aggregate"},
		{"SELECT count(*) FROM a GROUP BY x HAVING sum(sum(id)) LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a GROUP BY x HAVING abs() LIMIT 0", "wrong number of arguments"},
		{"SELECT count(*) FROM a GROUP BY x LIMIT NULL OFFSET missing", "no such column"},
		{"SELECT x FROM a HAVING count(*)>0", "non-aggregate"},
	} {
		t.Run(tc.q, func(t *testing.T) {
			out, err := exec.Command(confOracle(t), "-readonly", "-batch", "-bail", path, tc.q).CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.part) {
				t.Fatalf("oracle want %s got %v %s", tc.part, err, out)
			}
			r, err := db.Query(context.Background(), tc.q)
			if err == nil {
				for r.Next() {
				}
				err = r.Err()
				r.Close()
			}
			var detail *sqlite.Error
			if !errors.As(err, &detail) || detail.Code != sqlite.CodeError || !strings.Contains(err.Error(), tc.part) {
				t.Fatalf("native want error %s got %v", tc.part, err)
			}
		})
	}
	for _, q := range []string{"EXPLAIN QUERY PLAN SELECT x,count(*) FROM a GROUP BY x HAVING abs(-9223372036854775808) LIMIT NULL OFFSET 1.5", "EXPLAIN QUERY PLAN SELECT x,count(*) FROM a GROUP BY x LIMIT abs(-9223372036854775808)"} {
		out, err := exec.Command(confOracle(t), "-readonly", "-batch", "-bail", path, q).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Next() || r.RelationalStats().SourceOpens != 0 {
			t.Fatal(r.Err(), r.RelationalStats())
		}
		r.Close()
	}
}

func TestGroupedAggregateOracleAliasesAndExpressions(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	queries := []string{
		"SELECT id%2 AS x,count(*) FROM a GROUP BY x ORDER BY x",
		"SELECT id%2 AS g,count(*) FROM a GROUP BY g ORDER BY g",
		"SELECT id%2 AS g,count(*) FROM a GROUP BY +1 ORDER BY 1",
		"SELECT id%2 AS g,count(*) FROM a GROUP BY (1) ORDER BY 1",
		"SELECT x AS n,count(*) FROM a GROUP BY x HAVING n='one' ORDER BY x",
		"SELECT true,count(*) FROM a GROUP BY true",
		"SELECT true AS false,count(*) FROM boolnames GROUP BY false ORDER BY false",
		"SELECT false AS true,count(*) FROM boolnames GROUP BY true ORDER BY true",
		"SELECT x AS amount,count(*) AS amount FROM a GROUP BY x ORDER BY amount",
		"SELECT x AS id,count(*) FROM a GROUP BY x ORDER BY id+0",
	}
	// Physical name shadowing can resolve to a non-key bare source expression;
	// those two narrower-profile cases are asserted as unsupported separately.
	for _, q := range queries {
		if q == "SELECT id%2 AS x,count(*) FROM a GROUP BY x ORDER BY x" || q == "SELECT x AS n,count(*) FROM a GROUP BY x HAVING n='one' ORDER BY x" || q == "SELECT true AS false,count(*) FROM boolnames GROUP BY false ORDER BY false" || q == "SELECT false AS true,count(*) FROM boolnames GROUP BY true ORDER BY true" || q == "SELECT x AS id,count(*) FROM a GROUP BY x ORDER BY id+0" {
			if _, err := confQuoted(confCLI(t, path, q, true)); err != nil {
				t.Fatal(err)
			}
			r, err := db.Query(context.Background(), q)
			if r != nil {
				r.Close()
			}
			if !errors.Is(err, sqlite.ErrUnsupported) {
				t.Fatal(q, err)
			}
			continue
		}
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
	keys := []string{"id%2", "x", "x+0", "CAST(x AS NUMERIC)", "CASE WHEN x IS NULL THEN 0 ELSE x END", "typeof(x)", "id%2,x"}
	for _, key := range keys {
		for _, calls := range []string{"count(*),sum(id)", "min(id),max(id)", "max(id),min(id)", "CASE WHEN count(*)>1 THEN min(id) ELSE max(id) END"} {
			cols := strings.Split(key, ",")
			projection := key
			for _, c := range cols {
				projection += ",typeof(" + c + ")"
			}
			q := "SELECT " + projection + "," + calls + " FROM a GROUP BY " + key + " ORDER BY " + key
			t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
		}
	}
	q := "SELECT id%?1 AS g,sum(id+?2) AS s FROM a GROUP BY g HAVING sum(id+?2)>?3 ORDER BY g DESC LIMIT ?4 OFFSET ?5"
	want := confExpected(t, path, "SELECT id%2 AS g,sum(id+10) AS s FROM a GROUP BY g HAVING sum(id+10)>20 ORDER BY g DESC LIMIT 2 OFFSET 0")
	confCompare(t, want, confNative(t, db, q, 2, 10, 20, 2, 0))
}
func TestGroupedAggregateOracleTypedLazyBoundary(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	confCLI(t, path, "CREATE TABLE grouped_typed(id INTEGER PRIMARY KEY,v VECTOR(2)); INSERT INTO grouped_typed VALUES(1,'[1,2]'),(2,7);", false)
	for _, q := range []string{
		"SELECT id,count(*) FROM grouped_typed GROUP BY id",
		"SELECT CASE WHEN id=1 THEN v ELSE NULL END,count(*) FROM grouped_typed GROUP BY CASE WHEN id=1 THEN v ELSE NULL END",
		"SELECT typeof(CASE WHEN id=1 THEN v ELSE NULL END),count(*) FROM grouped_typed GROUP BY CASE WHEN id=1 THEN v ELSE NULL END",
		"SELECT id,count(v) FROM grouped_typed GROUP BY id LIMIT 1",
	} {
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
	r, err := db.Query(context.Background(), "SELECT id,count(v) FROM grouped_typed GROUP BY id")
	if err != nil {
		t.Fatal(err)
	}
	if r.Next() || !errors.Is(r.Err(), sqlite.ErrMismatch) {
		t.Fatal(r.Values(), r.Err())
	}
	r.Close()
}

func TestGroupedAggregateOracleTopKProjectionTiming(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	confCLI(t, path, "CREATE TABLE top_projection(k,v); INSERT INTO top_projection VALUES(1,1),(2,2),(3,3);", false)
	for _, q := range []string{
		"SELECT k,CASE WHEN k=3 THEN abs(-9223372036854775808) ELSE count(*) END FROM top_projection GROUP BY k ORDER BY k+0 LIMIT 1",
		"SELECT k,CASE WHEN k=1 THEN abs(-9223372036854775808) ELSE count(*) END FROM top_projection GROUP BY k ORDER BY k+0 DESC LIMIT 1",
		"SELECT k,CASE WHEN k=3 THEN abs(-9223372036854775808) ELSE count(*) END FROM top_projection GROUP BY k ORDER BY k+0 LIMIT 1 OFFSET 1",
		"SELECT k,count(*) FROM top_projection GROUP BY k ORDER BY -k LIMIT 2",
		"SELECT k,count(*) FROM top_projection GROUP BY k ORDER BY count(*) DESC LIMIT 2",
		"SELECT k,count(*) FROM top_projection GROUP BY k ORDER BY count(*) LIMIT 2 OFFSET 1",
	} {
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
}

func TestGroupedAggregateOracleRowidStream(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	confCLI(t, path, "CREATE TABLE stream_group(id INTEGER PRIMARY KEY,v); INSERT INTO stream_group VALUES(1,1),(2,2),(3,3),(4,4);", false)
	for _, keys := range []string{"id", "id,id", "id,v", "id+0,id", "v,id"} {
		for _, tail := range []string{" LIMIT 1", " LIMIT 1 OFFSET 1", " ORDER BY " + keys + " LIMIT 1", " ORDER BY " + keys + " DESC LIMIT 1", " ORDER BY id DESC LIMIT 1", " ORDER BY id DESC,id ASC LIMIT 1", " ORDER BY id NULLS LAST LIMIT 1"} {
			for _, bad := range []int{1, 2, 3, 4} {
				q := fmt.Sprintf("SELECT id,count(*) FROM stream_group WHERE CASE WHEN id=%d THEN abs(-9223372036854775808) ELSE 1 END GROUP BY %s%s", bad, keys, tail)
				t.Run(q, func(t *testing.T) {
					out, officialErr := exec.Command(confOracle(t), "-readonly", "-batch", "-bail", path, q).CombinedOutput()
					if officialErr == nil {
						confCompare(t, confExpected(t, path, q), confNative(t, db, q))
						return
					}
					if !strings.Contains(string(out), "integer overflow") {
						t.Fatal(officialErr, string(out))
					}
					r, err := db.Query(context.Background(), q)
					if err == nil {
						if r.Next() {
							t.Fatal("error published partial answer", r.Values())
						}
						err = r.Err()
						r.Close()
					}
					if err == nil || !strings.Contains(err.Error(), "integer overflow") {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
