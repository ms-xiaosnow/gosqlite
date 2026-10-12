package sqlite

import (
	"context"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Source-only candidate. All new runtime stages remain NOT_RUN until the exact
// source and finite selectors receive separate review. Fixtures are intact,
// SQL-created 4096-byte rollback databases; no recovery or fault path is used.
// The historical fourteen statements below are preserved exactly, including
// all three ordinary secondary indexes and the text-valued raw columns.
const leftWherePrefilterHistoricalSeed = `CREATE TABLE t0(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE t1(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE tn(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE u(y INTEGER);
CREATE TABLE empty(y INTEGER);
CREATE TABLE names(id INTEGER PRIMARY KEY,"true" INTEGER,"false" INTEGER,v INTEGER);
INSERT INTO t0 VALUES(1,0,-9223372036854775808,'01','01');
INSERT INTO t1 VALUES(1,1,-9223372036854775808,'01','01');
INSERT INTO tn VALUES(1,NULL,-9223372036854775808,'01','01');
INSERT INTO u VALUES(2);
INSERT INTO names VALUES(1,0,1,-9223372036854775808);
CREATE INDEX fi0 ON t0(flag);
CREATE INDEX fi1 ON t1(flag);
CREATE INDEX fin ON tn(flag);`

// This is a separate healthy setup, not a mutation of the historical fixture.
const leftWherePrefilterNoIndexSeed = `CREATE TABLE t0(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE t1(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE tn(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE u(y INTEGER);
CREATE TABLE empty(y INTEGER);
CREATE TABLE names(id INTEGER PRIMARY KEY,"true" INTEGER,"false" INTEGER,v INTEGER);
INSERT INTO t0 VALUES(1,0,-9223372036854775808,'01','01');
INSERT INTO t1 VALUES(1,1,-9223372036854775808,'01','01');
INSERT INTO tn VALUES(1,NULL,-9223372036854775808,'01','01');
INSERT INTO u VALUES(2);
INSERT INTO names VALUES(1,0,1,-9223372036854775808);`

const leftWherePrefilterOrdinarySeed = `CREATE TABLE l(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER);
CREATE TABLE r(y INTEGER);
CREATE TABLE e(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER);
INSERT INTO l VALUES(1,0,-9223372036854775808),(2,1,7),(3,NULL,-9223372036854775808);
INSERT INTO r VALUES(2),(3);`

const leftWherePrefilterStorageSeed = `CREATE TABLE l(id INTEGER PRIMARY KEY,n NUMERIC,txt TEXT,raw BLOB,r REAL);
CREATE TABLE u(y INTEGER);
INSERT INTO u VALUES(2);
INSERT INTO l VALUES(1,1,'01',x'3031',1.5);
INSERT INTO l VALUES(2,9007199254740992,CAST(x'610062' AS TEXT),x'610062',2.0);
INSERT INTO l VALUES(3,9007199254740993,'9007199254740993',x'00',3.0);
INSERT INTO l VALUES(4,-9223372036854775808,'minimum',x'01',4.0);
INSERT INTO l VALUES(5,9223372036854775807,'maximum',x'ff',5.0);`

const leftWherePrefilterReason = "left-row equality prefilter; full WHERE recheck after join"
const leftWherePrefilterHistoricalQuery = "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99"

type leftWherePrefilterWork struct {
	opens, rows, pairs, nulls int64
}

type leftWherePrefilterCase struct {
	name, seed, query, family string
	want                      [][]any
	work                      leftWherePrefilterWork
	method                    string
}

func leftWherePrefilterFixture(t *testing.T, seed string) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "left-where.db"), Options{PageSize: 4096})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, q := range strings.Split(seed, ";") {
		if strings.TrimSpace(q) != "" {
			if _, err := db.Exec(context.Background(), q); err != nil {
				t.Fatal(q, err)
			}
		}
	}
	if info := db.Info(); info.PageSize != 4096 || info.ReadVersion != 1 || info.WriteVersion != 1 {
		t.Fatal("fixture is not a 4096-byte rollback database", info)
	}
	return db
}

func leftWherePrefilterError(t *testing.T, err error, code Code, family string) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != code || !strings.Contains(err.Error(), family) {
		t.Fatalf("error=%v; want code=%d containing %q", err, code, family)
	}
	if code != CodeMismatch && errors.Is(err, ErrMismatch) {
		t.Fatal("non-conversion error relabeled mismatch", err)
	}
}

func leftWherePrefilterReleased(t *testing.T, db *DB, r *Rows) {
	t.Helper()
	if r != nil {
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if r.RelationalStats().CurrentBytes != 0 || r.relational != nil && (r.relational.scalarBytes != 0 || r.relational.resultBytes != 0) {
			t.Fatal("terminal row/scalar/result reservation retained", r.RelationalStats())
		}
	}
	if len(db.rows) != 0 {
		t.Fatal("query retained a connection Rows lease", len(db.rows))
	}
}

// Call only after Next has reached EOF/error. Assert automatic retirement
// before an explicit Close can hide a missing terminal cleanup transition.
func leftWherePrefilterTerminal(t *testing.T, db *DB, r *Rows) {
	t.Helper()
	if r == nil || !r.closed || r.source != nil || r.release != nil || len(db.rows) != 0 {
		t.Fatal("terminal Next did not retire Rows/source/snapshot lease")
	}
	if r.relational == nil || r.RelationalStats().CurrentBytes != 0 || r.relational.scalarBytes != 0 || r.relational.resultBytes != 0 {
		t.Fatal("terminal Next retained row/scalar/result reservations", r.RelationalStats())
	}
	if r.tx != nil && r.tx.activeRows != 0 {
		t.Fatal("terminal Next retained the transaction Rows lease", r.tx.activeRows)
	}
}

func leftWherePrefilterCheckWork(t *testing.T, r *Rows, want leftWherePrefilterWork) {
	t.Helper()
	s := r.RelationalStats()
	got := leftWherePrefilterWork{s.SourceOpens, s.SourceRows, s.JoinPairs, s.NullExtensions}
	if got != want || r.Stats().BaseRowFetches != want.rows {
		t.Fatalf("actual source/join work=%+v base fetches=%d; want %+v", got, r.Stats().BaseRowFetches, want)
	}
	if r.Stats().IndexKeysRead != 0 || r.Stats().IndexPageReads != 0 || r.Stats().RowIDSeeks != 0 {
		t.Fatal("nested-loop prefilter claimed index work", r.Stats())
	}
}

// Find the input evaluator explicitly, including both aggregate lowering paths.
// Each join must consult its current rows pointer, not the lowered output Rows.
func leftWherePrefilterInput(t *testing.T, r *Rows) (*Rows, *relationalJoin) {
	t.Helper()
	input, src := r, r.source
	switch s := src.(type) {
	case *aggregateSource:
		input, src = s.child, s.child.source
	case *groupedAggregateSource:
		input, src = s.child, s.raw.source
	}
	j, ok := src.(*relationalJoin)
	if !ok || j.rows != input || j.budget != r.relational {
		t.Fatalf("join lost input evaluator/root budget: source=%T", src)
	}
	return input, j
}

func leftWherePrefilterRun(t *testing.T, db *DB, tc leftWherePrefilterCase) {
	t.Helper()
	before := db.counters
	r, err := db.Query(context.Background(), tc.query)
	if err != nil || r == nil {
		t.Fatalf("Query phase: Rows=%v error=%v", r, err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	input, j := leftWherePrefilterInput(t, r)
	if j.leftWhere == nil || j.leftWhere.fixedColumns != nil || len(j.leftWhere.constant) != 0 || input.where == nil || input.sel.Where == nil {
		t.Fatal("missing independent prefilter or complete post-join WHERE")
	}
	if input == r && r.where == j.leftWhere {
		t.Fatal("prefilter reused the transformed full-WHERE program")
	}
	method, access := tc.method, "AGGREGATE"
	if method == "" {
		method, access = "nested_loop", "JOIN"
	}
	plan := r.Plan()
	if plan.Access != access || plan.Method != method || !strings.Contains(plan.Reason, leftWherePrefilterReason) {
		t.Fatal("untruthful admitted plan", plan)
	}
	if s := r.RelationalStats(); s.SourceOpens != 0 || s.SourceRows != 0 || s.JoinPairs != 0 || s.NullExtensions != 0 {
		t.Fatal("Query evaluated an unsorted input", s)
	}
	var got [][]any
	for r.Next() {
		got = append(got, r.Values())
	}
	if tc.family == "" {
		if err := r.Err(); err != nil {
			t.Fatal("Rows.Err phase", err)
		}
	} else {
		leftWherePrefilterError(t, r.Err(), CodeError, tc.family)
	}
	if !reflect.DeepEqual(got, tc.want) {
		t.Fatalf("SQL=%q got=%#v want=%#v", tc.query, got, tc.want)
	}
	if r.Stats().ReturnedRows != int64(len(got)) {
		t.Fatal("returned row count", r.Stats())
	}
	leftWherePrefilterCheckWork(t, r, tc.work)
	leftWherePrefilterTerminal(t, db, r)
	leftWherePrefilterReleased(t, db, r)
	if db.counters != before || !reflect.DeepEqual(plan, r.Plan()) {
		t.Fatal("read changed connection counters or terminal plan")
	}
}

func TestLeftWherePrefilterResults(t *testing.T) {
	// Exactly 31 named cases; no generated SQL cross-product or random input.
	cases := []leftWherePrefilterCase{
		{name: "historical_indexed_reject", seed: leftWherePrefilterHistoricalSeed, query: leftWherePrefilterHistoricalQuery, work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "historical_no_indexes_reject", seed: leftWherePrefilterNoIndexSeed, query: leftWherePrefilterHistoricalQuery, work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "historical_kept_on_overflow", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=0", family: "integer overflow", work: leftWherePrefilterWork{2, 2, 1, 0}},
		{name: "invalid_escape_rejected", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON t.txt LIKE 'x' ESCAPE 'ab' WHERE t.flag=99", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "invalid_escape_kept", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON t.txt LIKE 'x' ESCAPE 'ab' WHERE t.flag=0", family: "ESCAPE", work: leftWherePrefilterWork{2, 2, 1, 0}},
		{name: "null_flag_rejected", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM tn t LEFT JOIN u ON abs(t.v) WHERE t.flag=99", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "null_literal_rejected", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM tn t LEFT JOIN u ON abs(t.v) WHERE t.flag=NULL", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "kept_false_on_null_extension", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y,typeof(u.y) FROM t0 t LEFT JOIN u ON t.flag WHERE t.flag=0", want: [][]any{{int64(1), nil, "null"}}, work: leftWherePrefilterWork{2, 2, 1, 1}},
		{name: "kept_two_matches", seed: leftWherePrefilterOrdinarySeed, query: "SELECT l.id,r.y FROM l LEFT JOIN r ON l.flag=1 WHERE l.flag=1", want: [][]any{{int64(2), int64(2)}, {int64(2), int64(3)}}, work: leftWherePrefilterWork{2, 5, 2, 0}},
		{name: "empty_left", seed: leftWherePrefilterOrdinarySeed, query: "SELECT e.id,r.y FROM e LEFT JOIN r ON abs(e.v) WHERE e.flag=99", work: leftWherePrefilterWork{1, 0, 0, 0}},
		{name: "empty_right_rejected", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,empty.y FROM t0 t LEFT JOIN empty ON abs(t.v) WHERE t.flag=99", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "empty_right_kept", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,empty.y FROM t0 t LEFT JOIN empty ON abs(t.v) WHERE t.flag=0", want: [][]any{{int64(1), nil}}, work: leftWherePrefilterWork{2, 1, 0, 1}},
		{name: "commuted_qualified", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE 99=t.flag", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "double_equals", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag==99", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "parenthesized_three_and", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE (t.id=1 AND (t.flag=99 AND '01'=t.txt))", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "direct_result_alias", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.flag AS chosen,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE chosen=99", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "physical_name_precedence", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id AS flag,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE flag=1", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "hidden_left_rowid", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t._rowid_=99", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "physical_left_ipk", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.id=99", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "numeric_text_affinity", seed: leftWherePrefilterStorageSeed, query: "SELECT l.n,typeof(l.n),u.y FROM l LEFT JOIN u ON 1 WHERE l.n='01'", want: [][]any{{int64(1), "integer", int64(2)}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "text_numeric_distinction", seed: leftWherePrefilterStorageSeed, query: "SELECT l.id FROM l LEFT JOIN u ON abs(-9223372036854775808) WHERE l.txt=1", work: leftWherePrefilterWork{1, 5, 0, 0}},
		{name: "blob_text_distinction", seed: leftWherePrefilterStorageSeed, query: "SELECT l.id FROM l LEFT JOIN u ON abs(-9223372036854775808) WHERE l.raw='01'", work: leftWherePrefilterWork{1, 5, 0, 0}},
		{name: "blob_literal", seed: leftWherePrefilterStorageSeed, query: "SELECT l.raw,typeof(l.raw),u.y FROM l LEFT JOIN u ON 1 WHERE l.raw=x'3031'", want: [][]any{{[]byte{0x30, 0x31}, "blob", int64(2)}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "embedded_nul_text_not_prefix", seed: leftWherePrefilterStorageSeed, query: "SELECT l.id FROM l LEFT JOIN u ON abs(-9223372036854775808) WHERE l.txt='a'", work: leftWherePrefilterWork{1, 5, 0, 0}},
		{name: "embedded_nul_blob_and_text", seed: leftWherePrefilterStorageSeed, query: "SELECT l.txt,typeof(l.txt),l.raw,typeof(l.raw) FROM l LEFT JOIN u ON 1 WHERE l.raw=x'610062'", want: [][]any{{"a\x00b", "text", []byte{0x61, 0, 0x62}, "blob"}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "real_literal", seed: leftWherePrefilterStorageSeed, query: "SELECT l.r,typeof(l.r) FROM l LEFT JOIN u ON 1 WHERE l.r=+1.5", want: [][]any{{float64(1.5), "real"}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "int_real_boundary_2pow53", seed: leftWherePrefilterStorageSeed, query: "SELECT l.n,typeof(l.n) FROM l LEFT JOIN u ON 1 WHERE l.n=9007199254740992.0", want: [][]any{{int64(9007199254740992), "integer"}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "integer_above_2pow53", seed: leftWherePrefilterStorageSeed, query: "SELECT l.n,typeof(l.n) FROM l LEFT JOIN u ON 1 WHERE l.n=9007199254740993", want: [][]any{{int64(9007199254740993), "integer"}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "signed_minimum", seed: leftWherePrefilterStorageSeed, query: "SELECT l.n,typeof(l.n) FROM l LEFT JOIN u ON 1 WHERE l.n=-9223372036854775808", want: [][]any{{int64(-9223372036854775808), "integer"}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "signed_maximum", seed: leftWherePrefilterStorageSeed, query: "SELECT l.n,typeof(l.n) FROM l LEFT JOIN u ON 1 WHERE l.n=+9223372036854775807", want: [][]any{{int64(9223372036854775807), "integer"}}, work: leftWherePrefilterWork{2, 6, 1, 0}},
		{name: "startup_contradictory_and_rejects_all", seed: leftWherePrefilterOrdinarySeed, query: "SELECT l.id,r.y FROM l LEFT JOIN r ON abs(l.v) WHERE l.flag=0 AND l.flag=1", work: leftWherePrefilterWork{}},
	}
	if len(cases) != 31 {
		t.Fatal("finite result inventory changed", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leftWherePrefilterRun(t, leftWherePrefilterFixture(t, tc.seed), tc)
		})
	}
}

type leftWherePrefilterValuer struct {
	calls *int
	err   error
}

func (v leftWherePrefilterValuer) Value() (driver.Value, error) {
	(*v.calls)++
	return int64(99), v.err
}

func TestLeftWherePrefilterPhaseAndOwnership(t *testing.T) {
	// Fourteen explicit preparation failures, followed by fifteen histories.
	binding := []struct {
		name, query, family string
		code                Code
		args                []any
	}{
		{name: "on_missing", query: "SELECT t.id FROM t0 t LEFT JOIN u ON missing=1 WHERE t.flag=99", family: "no such column", code: CodeError},
		{name: "where_missing", query: "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE missing=99", family: "no such column", code: CodeError},
		{name: "projection_missing_limit_zero", query: "SELECT missing FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT 0", family: "no such column", code: CodeError},
		{name: "order_missing_explain", query: "EXPLAIN QUERY PLAN SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 ORDER BY missing", family: "no such column", code: CodeError},
		{name: "on_missing_limit_zero", query: "SELECT t.id FROM t0 t LEFT JOIN u ON missing=1 WHERE t.flag=99 LIMIT 0", family: "no such column", code: CodeError},
		{name: "on_arity_explain", query: "EXPLAIN QUERY PLAN SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v,1) WHERE t.flag=99", family: "wrong number of arguments", code: CodeError},
		{name: "missing_parameter", query: "SELECT ?1 FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99", family: "missing parameter", code: CodeError},
		{name: "extra_parameter", query: leftWherePrefilterHistoricalQuery, args: []any{int64(1)}, family: "too many parameters", code: CodeError},
		{name: "nonzero_limit_mismatch", query: "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT NULL", family: "datatype mismatch", code: CodeMismatch},
		{name: "offset_mismatch", query: "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT 1 OFFSET NULL", family: "datatype mismatch", code: CodeMismatch},
		{name: "offset_binding_before_limit_overflow", query: "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT abs(-9223372036854775808) OFFSET missing", family: "no such column", code: CodeError},
		{name: "limit_mismatch_before_offset_overflow", query: "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT NULL OFFSET abs(-9223372036854775808)", family: "datatype mismatch", code: CodeMismatch},
		{name: "limit_overflow_before_prefilter", query: "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT abs(-9223372036854775808)", family: "integer overflow", code: CodeError},
		{name: "offset_overflow_before_prefilter", query: "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT 1 OFFSET abs(-9223372036854775808)", family: "integer overflow", code: CodeError},
	}
	if len(binding) != 14 {
		t.Fatal("finite binding inventory changed", len(binding))
	}
	for _, tc := range binding {
		t.Run(tc.name, func(t *testing.T) {
			db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
			before := db.counters
			r, err := db.Query(context.Background(), tc.query, tc.args...)
			if r != nil {
				leftWherePrefilterReleased(t, db, r)
				t.Fatal("preparation failure returned Rows")
			}
			leftWherePrefilterError(t, err, tc.code, tc.family)
			leftWherePrefilterReleased(t, db, nil)
			if before != db.counters {
				t.Fatal("preparation error changed connection counters")
			}
		})
	}
	t.Run("parameter_where_keeps_old_order", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		r, err := db.Query(context.Background(), "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=?1", int64(99))
		if err != nil {
			t.Fatal(err)
		}
		_, j := leftWherePrefilterInput(t, r)
		if j.leftWhere != nil || strings.Contains(r.Plan().Reason, leftWherePrefilterReason) {
			t.Fatal("parameterized WHERE was admitted")
		}
		if r.Next() {
			t.Fatal("parameter control returned a row")
		}
		leftWherePrefilterError(t, r.Err(), CodeError, "integer overflow")
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{2, 2, 1, 0})
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if db.counters != before {
			t.Fatal("parameter control changed counters")
		}
	})
	t.Run("having_keeps_old_plan", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		r, err := db.Query(context.Background(), "SELECT count(*) FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 HAVING count(*)>=0")
		if err != nil {
			t.Fatal(err)
		}
		_, j := leftWherePrefilterInput(t, r)
		if j.leftWhere != nil || strings.Contains(r.Plan().Reason, leftWherePrefilterReason) {
			t.Fatal("HAVING query was admitted")
		}
		if r.Next() {
			t.Fatal("unadmitted aggregate returned a row")
		}
		leftWherePrefilterError(t, r.Err(), CodeError, "integer overflow")
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{2, 2, 1, 0})
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if db.counters != before {
			t.Fatal("HAVING control changed counters")
		}
	})
	t.Run("promoted_on_alias_keeps_old_plan", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		r, err := db.Query(context.Background(), "SELECT t.flag AS chosen,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND chosen WHERE t.flag=99")
		if err != nil {
			t.Fatal(err)
		}
		_, j := leftWherePrefilterInput(t, r)
		if j.leftWhere != nil || strings.Contains(r.Plan().Reason, leftWherePrefilterReason) {
			t.Fatal("promoted ON alias query was admitted")
		}
		if r.Next() {
			t.Fatal("unadmitted alias query returned a row")
		}
		leftWherePrefilterError(t, r.Err(), CodeError, "integer overflow")
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{2, 2, 1, 0})
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if db.counters != before {
			t.Fatal("alias control changed counters")
		}
	})
	t.Run("valuer_binding_error_before_planning", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before, calls := db.counters, 0
		want := errors.New("left prefilter valuer sentinel")
		r, err := db.Query(context.Background(), "SELECT ?1 FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 LIMIT 0", leftWherePrefilterValuer{calls: &calls, err: want})
		if r != nil || !errors.Is(err, want) || calls != 1 {
			if r != nil {
				leftWherePrefilterReleased(t, db, r)
			}
			t.Fatalf("valuer phase: rows=%v err=%v calls=%d", r, err, calls)
		}
		leftWherePrefilterReleased(t, db, nil)
		if db.counters != before {
			t.Fatal("valuer error changed counters")
		}
	})
	t.Run("limit_zero_skips_on_and_offset", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		leftWherePrefilterRun(t, db, leftWherePrefilterCase{query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=0 LIMIT 0 OFFSET abs(-9223372036854775808)"})
	})
	t.Run("explain_validates_without_execution", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		r, err := db.Query(context.Background(), "EXPLAIN QUERY PLAN SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=0 LIMIT NULL OFFSET abs(-9223372036854775808)")
		if err != nil {
			t.Fatal(err)
		}
		plan := r.Plan()
		if plan.Access != "JOIN" || plan.Method != "nested_loop" || !strings.Contains(plan.Reason, leftWherePrefilterReason) || !r.Next() || !reflect.DeepEqual(r.Values(), []any{int64(0), int64(0), int64(0), plan.Detail()}) {
			t.Fatal("EXPLAIN row", r.Values(), r.Err(), plan)
		}
		if r.Next() || r.Err() != nil {
			t.Fatal("EXPLAIN executed a bound or ON", r.Err())
		}
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{})
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if db.counters != before {
			t.Fatal("EXPLAIN changed counters")
		}
	})
	t.Run("startup_mixed_where_keeps_old_order", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		r, err := db.Query(context.Background(), "SELECT t.id FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 AND 'a' LIKE 'x' ESCAPE 'ab'")
		if err != nil {
			t.Fatal(err)
		}
		_, j := leftWherePrefilterInput(t, r)
		if j.leftWhere != nil {
			t.Fatal("mixed error-capable WHERE was partially admitted")
		}
		if r.Next() {
			t.Fatal("startup error returned a row")
		}
		leftWherePrefilterError(t, r.Err(), CodeError, "ESCAPE")
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{})
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if db.counters != before {
			t.Fatal("startup error changed counters")
		}
	})
	t.Run("precanceled_context", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r, err := db.Query(ctx, leftWherePrefilterHistoricalQuery)
		if r != nil || !errors.Is(err, context.Canceled) {
			if r != nil {
				leftWherePrefilterReleased(t, db, r)
			}
			t.Fatal("pre-canceled query", err)
		}
		leftWherePrefilterReleased(t, db, nil)
		if db.counters != before {
			t.Fatal("cancellation changed counters")
		}
	})
	t.Run("cancel_after_prepare", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		r, err := db.Query(ctx, leftWherePrefilterHistoricalQuery)
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		if r.Next() || !errors.Is(r.Err(), context.Canceled) {
			t.Fatal("canceled Next", r.Err())
		}
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{})
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if db.counters != before {
			t.Fatal("cancellation changed counters")
		}
	})
	for _, tc := range []struct {
		name   string
		limits RelationalLimits
		want   error
	}{
		{name: "low_work", limits: RelationalLimits{MaxWork: 1}, want: ErrQueryWorkLimit},
		{name: "low_memory", limits: RelationalLimits{MaxBytes: 1}, want: ErrMemoryLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// These fixed limits fail preparation, not prefilter execution.
			db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
			before := db.counters
			r, err := db.Query(WithRelationalLimits(context.Background(), tc.limits), leftWherePrefilterHistoricalQuery)
			if r != nil || !errors.Is(err, tc.want) {
				if r != nil {
					leftWherePrefilterReleased(t, db, r)
				}
				t.Fatalf("preparation quota Rows=%v error=%v; want nil Rows and %v", r, err, tc.want)
			}
			leftWherePrefilterReleased(t, db, r)
			if db.counters != before {
				t.Fatal("quota changed counters")
			}
			leftWherePrefilterRun(t, db, leftWherePrefilterCase{query: leftWherePrefilterHistoricalQuery, work: leftWherePrefilterWork{1, 1, 0, 0}})
		})
	}
	t.Run("close_before_next", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		r, err := db.Query(context.Background(), leftWherePrefilterHistoricalQuery)
		if err != nil {
			t.Fatal(err)
		}
		if r.release == nil || len(db.rows) != 1 {
			t.Fatal("missing sole snapshot lease")
		}
		leftWherePrefilterReleased(t, db, r)
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{})
		if r.Next() || r.Err() != nil || db.counters != before {
			t.Fatal("unstarted Close changed error or counters", r.Err())
		}
	})
	t.Run("early_close_after_kept_pair", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterOrdinarySeed)
		before := db.counters
		r, err := db.Query(context.Background(), "SELECT l.id,r.y FROM l LEFT JOIN r ON 1 WHERE l.flag=1")
		if err != nil || !r.Next() || !reflect.DeepEqual(r.Values(), []any{int64(2), int64(2)}) {
			t.Fatal("first kept pair", err)
		}
		leftWherePrefilterCheckWork(t, r, leftWherePrefilterWork{2, 3, 1, 0})
		leftWherePrefilterReleased(t, db, r)
		if r.Err() != nil || db.counters != before {
			t.Fatal("early Close changed error or counters", r.Err())
		}
		leftWherePrefilterRun(t, db, leftWherePrefilterCase{query: "SELECT l.id,r.y FROM l LEFT JOIN r ON 1 WHERE l.flag=1", want: [][]any{{int64(2), int64(2)}, {int64(2), int64(3)}}, work: leftWherePrefilterWork{2, 5, 2, 0}})
	})
	t.Run("full_eof_and_reuse", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		leftWherePrefilterRun(t, db, leftWherePrefilterCase{query: leftWherePrefilterHistoricalQuery, work: leftWherePrefilterWork{1, 1, 0, 0}})
		leftWherePrefilterRun(t, db, leftWherePrefilterCase{query: "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON 1 WHERE t.flag=0", want: [][]any{{int64(1), int64(2)}}, work: leftWherePrefilterWork{2, 2, 1, 0}})
	})
	t.Run("transaction_on_error_then_reuse", func(t *testing.T) {
		db := leftWherePrefilterFixture(t, leftWherePrefilterHistoricalSeed)
		before := db.counters
		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := tx.Rollback(); err != nil {
				t.Error(err)
			}
		})
		r, err := tx.Query(context.Background(), "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=0")
		if err != nil {
			t.Fatal(err)
		}
		_, j := leftWherePrefilterInput(t, r)
		left, ok := j.left.(*relationalLeaf)
		if !ok || left.base != tx.reader || r.release != nil || r.tx != tx || tx.activeRows != 1 || j.leftWhere == nil {
			t.Fatal("join/prefilter acquired a separate transaction reader or lease")
		}
		if r.Next() {
			t.Fatal("kept error row produced output")
		}
		leftWherePrefilterError(t, r.Err(), CodeError, "integer overflow")
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if tx.activeRows != 0 || tx.done || db.counters != before {
			t.Fatal("ON error damaged transaction ownership or counters")
		}
		r, err = tx.Query(context.Background(), "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON 1 WHERE t.flag=0")
		if err != nil || !r.Next() || !reflect.DeepEqual(r.Values(), []any{int64(1), int64(2)}) || r.Next() || r.Err() != nil {
			t.Fatal("transaction reuse", err)
		}
		leftWherePrefilterTerminal(t, db, r)
		leftWherePrefilterReleased(t, db, r)
		if tx.activeRows != 0 || db.counters != before {
			t.Fatal("reused transaction retained a lease or changed counters")
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		leftWherePrefilterRun(t, db, leftWherePrefilterCase{query: leftWherePrefilterHistoricalQuery, work: leftWherePrefilterWork{1, 1, 0, 0}})
	})
}

func TestLeftWherePrefilterAggregateInput(t *testing.T) {
	// Exactly six input-lowering controls; HAVING stays outside this slice.
	cases := []leftWherePrefilterCase{
		{name: "global_count_rejected", seed: leftWherePrefilterHistoricalSeed, query: "SELECT count(*) FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99", want: [][]any{{int64(0)}}, method: "global", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "global_sum_rejected", seed: leftWherePrefilterHistoricalSeed, query: "SELECT sum(u.y) FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99", want: [][]any{{nil}}, method: "global", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "grouped_rejected", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.flag,count(*) FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99 GROUP BY t.flag", method: "grouped", work: leftWherePrefilterWork{1, 1, 0, 0}},
		{name: "global_count_two_matches", seed: leftWherePrefilterOrdinarySeed, query: "SELECT count(*) FROM l LEFT JOIN r ON 1 WHERE l.flag=1", want: [][]any{{int64(2)}}, method: "global", work: leftWherePrefilterWork{2, 5, 2, 0}},
		{name: "grouped_kept_null_extension", seed: leftWherePrefilterHistoricalSeed, query: "SELECT t.flag,count(*),count(u.y) FROM t0 t LEFT JOIN u ON t.flag WHERE t.flag=0 GROUP BY t.flag", want: [][]any{{int64(0), int64(1), int64(0)}}, method: "grouped", work: leftWherePrefilterWork{2, 2, 1, 1}},
		{name: "global_kept_on_overflow", seed: leftWherePrefilterHistoricalSeed, query: "SELECT count(*) FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=0", family: "integer overflow", method: "global", work: leftWherePrefilterWork{2, 2, 1, 0}},
	}
	if len(cases) != 6 {
		t.Fatal("finite aggregate inventory changed", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leftWherePrefilterRun(t, leftWherePrefilterFixture(t, tc.seed), tc)
		})
	}
}
