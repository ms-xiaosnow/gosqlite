package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Source-only finite driver candidate: all runtime stages are NOT_RUN. This
// historical fourteen-statement fixture is intentionally unchanged, including
// the indexes and text storage in raw. Only healthy 4096-byte rollback files,
// ordinary SQL, normal EOF/Close, and explicit transaction lifetime are used.
const leftWherePrefilterDriverSeed = `CREATE TABLE t0(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
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

func leftWherePrefilterDriverDB(t *testing.T) *sql.DB {
	t.Helper()
	c, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "left-where-driver.db"), sqlite.Options{PageSize: 4096})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(c)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, q := range strings.Split(leftWherePrefilterDriverSeed, ";") {
		if strings.TrimSpace(q) != "" {
			if _, err := db.ExecContext(context.Background(), q); err != nil {
				t.Fatal(q, err)
			}
		}
	}
	return db
}

func leftWherePrefilterDriverRead(t *testing.T, rows *sql.Rows, columns []string, want [][]any) {
	t.Helper()
	defer rows.Close()
	gotColumns, err := rows.Columns()
	if err != nil || !reflect.DeepEqual(gotColumns, columns) {
		t.Fatalf("columns=%#v error=%v; want %#v", gotColumns, err, columns)
	}
	var got [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		args := make([]any, len(columns))
		for i := range values {
			args[i] = &values[i]
		}
		if err := rows.Scan(args...); err != nil {
			t.Fatal(err)
		}
		got = append(got, values)
	}
	if rows.Err() != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("rows=%#v error=%v; want %#v", got, rows.Err(), want)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
}

func leftWherePrefilterDriverCounters(t *testing.T, query func(context.Context, string, ...any) *sql.Row) [3]int64 {
	t.Helper()
	var got [3]int64
	if err := query(context.Background(), "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&got[0], &got[1], &got[2]); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestLeftWherePrefilterDriver(t *testing.T) {
	// Exactly three histories; no driver implementation changes are required.
	t.Run("normal_eof_and_reuse", func(t *testing.T) {
		db := leftWherePrefilterDriverDB(t)
		before := leftWherePrefilterDriverCounters(t, db.QueryRowContext)
		rows, err := db.QueryContext(context.Background(), "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99")
		if err != nil {
			t.Fatal("rejecting QueryContext", err)
		}
		leftWherePrefilterDriverRead(t, rows, []string{"id", "y"}, nil)
		if db.Stats().InUse != 0 {
			t.Fatal("EOF retained pooled connection", db.Stats())
		}
		rows, err = db.QueryContext(context.Background(), "SELECT t.id,u.y,t.raw,typeof(t.raw) AS storage FROM t0 t LEFT JOIN u ON t.flag WHERE t.flag=0")
		if err != nil {
			t.Fatal("normal reuse", err)
		}
		leftWherePrefilterDriverRead(t, rows, []string{"id", "y", "raw", "storage"}, [][]any{{int64(1), nil, "01", "text"}})
		if after := leftWherePrefilterDriverCounters(t, db.QueryRowContext); after != before || db.Stats().InUse != 0 {
			t.Fatal("read changed counters or retained connection", before, after, db.Stats())
		}
	})
	t.Run("explicit_tx_on_error_then_reuse", func(t *testing.T) {
		db := leftWherePrefilterDriverDB(t)
		before := leftWherePrefilterDriverCounters(t, db.QueryRowContext)
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		rows, err := tx.QueryContext(context.Background(), "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=0")
		if err != nil {
			t.Fatal("ON error moved into QueryContext", err)
		}
		if rows.Next() {
			rows.Close()
			t.Fatal("error-producing ON returned a row")
		}
		var typed *sqlite.Error
		if err := rows.Err(); !errors.As(err, &typed) || typed.Code != sqlite.CodeError || !strings.Contains(err.Error(), "integer overflow") {
			rows.Close()
			t.Fatal("missing Rows.Err overflow", err)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		rows, err = tx.QueryContext(context.Background(), "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON 1 WHERE t.flag=0")
		if err != nil {
			t.Fatal("transaction became unusable", err)
		}
		leftWherePrefilterDriverRead(t, rows, []string{"id", "y"}, [][]any{{int64(1), int64(2)}})
		if after := leftWherePrefilterDriverCounters(t, tx.QueryRowContext); after != before {
			t.Fatal("ON error or query changed connection counters", before, after)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if after := leftWherePrefilterDriverCounters(t, db.QueryRowContext); after != before || db.Stats().InUse != 0 {
			t.Fatal("transaction did not release sole connection", before, after, db.Stats())
		}
	})
	t.Run("prepared_unrelated_projection_parameter", func(t *testing.T) {
		db := leftWherePrefilterDriverDB(t)
		before := leftWherePrefilterDriverCounters(t, db.QueryRowContext)
		stmt, err := db.PrepareContext(context.Background(), "SELECT ?1 AS marker,t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) WHERE t.flag=99")
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()
		// Exactly two executions distinguish unrelated projection binding from
		// a parameterized WHERE, without admitting a parameter to the prefilter.
		rows, err := stmt.QueryContext(context.Background(), "first")
		if err != nil {
			t.Fatal(err)
		}
		leftWherePrefilterDriverRead(t, rows, []string{"marker", "id", "y"}, nil)
		rows, err = stmt.QueryContext(context.Background(), []byte{0, 255})
		if err != nil {
			t.Fatal(err)
		}
		leftWherePrefilterDriverRead(t, rows, []string{"marker", "id", "y"}, nil)
		if err := stmt.Close(); err != nil {
			t.Fatal(err)
		}
		if after := leftWherePrefilterDriverCounters(t, db.QueryRowContext); after != before || db.Stats().InUse != 0 {
			t.Fatal("prepared reads changed counters or retained connection", before, after, db.Stats())
		}
	})
}
