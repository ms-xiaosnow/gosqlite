package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Relational driver tests use only normal SQL/transactions/cancellation and
// pure query quotas against intact files. No native I/O failures are injected.
func relationalDriverDB(t *testing.T, shared bool) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "relations.db")
	connector, err := gosqldriver.NewConnector(path, sqlite.Options{WALShared: shared, KeepWALOnClose: shared})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, q := range []string{
		"CREATE TABLE a(id INTEGER PRIMARY KEY,k NUMERIC,label TEXT)",
		"CREATE TABLE b(id INTEGER PRIMARY KEY,k TEXT,score REAL,payload BLOB)",
		"INSERT INTO a VALUES(1,1,'first'),(2,1,'duplicate'),(3,NULL,'null'),(4,2,'two')",
		"INSERT INTO b VALUES(10,'1',1,x'00ff'),(11,'01',2,x''),(12,'2',3,x'32')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return db, path
}

func relationalDriverRead(t *testing.T, rows *sql.Rows, wantColumns []string) [][]any {
	t.Helper()
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if wantColumns != nil && !reflect.DeepEqual(columns, wantColumns) {
		t.Fatalf("columns = %#v; want %#v", columns, wantColumns)
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
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestDriverRelationalPreparedTypesAndTransactions(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared_%t", shared), func(t *testing.T) {
			db, _ := relationalDriverDB(t, shared)
			ctx := context.Background()
			stmt, err := db.PrepareContext(ctx, "SELECT a.id,b.id,b.score,b.payload FROM a LEFT JOIN b ON a.k=b.k AND b.id>=:minimum WHERE a.id<=:maximum ORDER BY a.id,b.id")
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()
			want := [][]any{{int64(1), int64(10), float64(1), []byte{0, 255}}, {int64(1), int64(11), float64(2), []byte{}}, {int64(2), int64(10), float64(1), []byte{0, 255}}, {int64(2), int64(11), float64(2), []byte{}}, {int64(3), nil, nil, nil}, {int64(4), int64(12), float64(3), []byte{50}}}
			for range 2 {
				rows, err := stmt.QueryContext(ctx, sql.Named("maximum", 4), sql.Named("minimum", 10))
				if err != nil {
					t.Fatal(err)
				}
				if got := relationalDriverRead(t, rows, []string{"id", "id", "score", "payload"}); !reflect.DeepEqual(got, want) {
					t.Fatalf("prepared join = %#v; want %#v", got, want)
				}
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.ExecContext(ctx, "UPDATE b SET score=score+10 WHERE id=10"); err != nil {
				t.Fatal(err)
			}
			txStmt := tx.StmtContext(ctx, stmt)
			rows, err := txStmt.QueryContext(ctx, sql.Named("maximum", 1), sql.Named("minimum", 10))
			if err != nil {
				t.Fatal(err)
			}
			inside := [][]any{{int64(1), int64(10), float64(11), []byte{0, 255}}, {int64(1), int64(11), float64(2), []byte{}}}
			if got := relationalDriverRead(t, rows, nil); !reflect.DeepEqual(got, inside) {
				t.Fatalf("join must read transaction writes: %#v", got)
			}
			if err := txStmt.Close(); err != nil {
				t.Fatal(err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			rows, err = stmt.QueryContext(ctx, sql.Named("maximum", 4), sql.Named("minimum", 10))
			if err != nil {
				t.Fatal(err)
			}
			if got := relationalDriverRead(t, rows, nil); !reflect.DeepEqual(got, want) {
				t.Fatalf("rollback changed join result: %#v", got)
			}
			var right sql.NullInt64
			var score sql.NullFloat64
			if err := db.QueryRowContext(ctx, "SELECT b.id,b.score FROM a LEFT JOIN b ON a.k=b.k WHERE a.id=3").Scan(&right, &score); err != nil || right.Valid || score.Valid {
				t.Fatalf("null extension Scan = %#v,%#v: %v", right, score, err)
			}
			if err := db.QueryRowContext(ctx, "SELECT b.id FROM a JOIN b ON a.k=b.k WHERE a.id=3").Scan(&right); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("empty inner join: %v", err)
			}
		})
	}
}

func TestDriverRelationalCloseCancelAndLimits(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared_%t", shared), func(t *testing.T) {
			db, path := relationalDriverDB(t, shared)
			ctx := context.Background()
			query := "SELECT a.id,b.id FROM a CROSS JOIN b"
			rows, err := db.QueryContext(ctx, query)
			if err != nil || !rows.Next() {
				t.Fatalf("first row: %v", err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			// Reusing the sole pooled connection proves early close releases it.
			if _, err := db.ExecContext(ctx, "UPDATE a SET label='after close' WHERE id=1"); err != nil {
				t.Fatal(err)
			}
			cancelCtx, cancel := context.WithCancel(ctx)
			rows, err = db.QueryContext(cancelCtx, query)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			if !rows.Next() {
				cancel()
				t.Fatal(rows.Err())
			}
			cancel()
			for rows.Next() {
			}
			if !errors.Is(rows.Err(), context.Canceled) {
				t.Fatalf("canceled join: %v", rows.Err())
			}
			rows.Close()
			if _, err := db.QueryContext(cancelCtx, query); !errors.Is(err, context.Canceled) {
				t.Fatalf("already canceled join: %v", err)
			}
			for _, tc := range []struct {
				name   string
				limits sqlite.RelationalLimits
				want   error
			}{
				{name: "work", limits: sqlite.RelationalLimits{MaxWork: 1}, want: sqlite.ErrQueryWorkLimit},
				{name: "memory", limits: sqlite.RelationalLimits{MaxBytes: 1}, want: sqlite.ErrMemoryLimit},
			} {
				t.Run(tc.name, func(t *testing.T) {
					limited := sqlite.WithRelationalLimits(ctx, tc.limits)
					rows, err := db.QueryContext(limited, query)
					if err == nil {
						for rows.Next() {
						}
						err = rows.Err()
						rows.Close()
					}
					if !errors.Is(err, tc.want) {
						t.Fatalf("query quota error = %v; want %v", err, tc.want)
					}
				})
			}
			rows, err = db.QueryContext(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			if got := relationalDriverRead(t, rows, []string{"id", "id"}); len(got) != 12 {
				t.Fatalf("normal query after cancellation/quotas = %d rows; want 12", len(got))
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			connector, err := gosqldriver.NewConnector(path, sqlite.Options{WALShared: shared, NoCreate: true})
			if err != nil {
				t.Fatal(err)
			}
			reopened := sql.OpenDB(connector)
			defer reopened.Close()
			rows, err = reopened.QueryContext(ctx, "SELECT a.label,b.id FROM a JOIN b ON a.k=b.k WHERE a.id=1 ORDER BY b.id")
			if err != nil {
				t.Fatal(err)
			}
			want := [][]any{{"after close", int64(10)}, {"after close", int64(11)}}
			if got := relationalDriverRead(t, rows, nil); !reflect.DeepEqual(got, want) {
				t.Fatalf("reopened join = %#v", got)
			}
		})
	}
}

func TestDriverRelationalSharedSnapshotBorrowing(t *testing.T) {
	db, path := relationalDriverDB(t, true)
	ctx := context.Background()
	writer, err := sqlite.Open(path, sqlite.Options{WALShared: true, KeepWALOnClose: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	query := "SELECT a.id,b.id,b.score FROM a CROSS JOIN b"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	var firstA, firstB int64
	var firstScore float64
	if err := rows.Scan(&firstA, &firstB, &firstScore); err != nil || firstA != 1 || firstB != 10 || firstScore != 1 {
		t.Fatalf("first old-snapshot row = %d,%d,%v: %v", firstA, firstB, firstScore, err)
	}
	if _, err := writer.Exec(ctx, "UPDATE b SET score=score+100"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Checkpoint(ctx); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatalf("live relational snapshot should hold checkpoint: %v", err)
	}
	count := 1
	for rows.Next() {
		var a, b int64
		var score float64
		if err := rows.Scan(&a, &b, &score); err != nil {
			t.Fatal(err)
		}
		if score != float64(b-9) {
			t.Fatalf("restarted right leaf borrowed a new snapshot at a=%d,b=%d: %v", a, b, score)
		}
		count++
	}
	if err := rows.Err(); err != nil || count != 12 {
		t.Fatalf("snapshot count=%d: %v", count, err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	var newest float64
	if err := db.QueryRowContext(ctx, "SELECT b.score FROM a JOIN b ON a.k=b.k WHERE a.id=1 AND b.id=10").Scan(&newest); err != nil || newest != 101 {
		t.Fatalf("fresh snapshot score=%v: %v", newest, err)
	}
	// Closing one join must not release the read transaction's borrowed snapshot.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, "SELECT b.score FROM a JOIN b ON a.k=b.k WHERE a.id=? AND b.id=?")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if err := stmt.QueryRowContext(ctx, 1, 10).Scan(&newest); err != nil || newest != 101 {
		t.Fatalf("transaction initial score=%v: %v", newest, err)
	}
	if _, err := writer.Exec(ctx, "UPDATE b SET score=201 WHERE id=10"); err != nil {
		t.Fatal(err)
	}
	if err := stmt.QueryRowContext(ctx, 1, 10).Scan(&newest); err != nil || newest != 101 {
		t.Fatalf("transaction snapshot after join close score=%v: %v", newest, err)
	}
	if err := writer.Checkpoint(ctx); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatalf("transaction still owns snapshot: %v", err)
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT b.score FROM a JOIN b ON a.k=b.k WHERE a.id=1 AND b.id=10").Scan(&newest); err != nil || newest != 201 {
		t.Fatalf("post-transaction snapshot score=%v: %v", newest, err)
	}
}

func TestDriverRelationalUnsupportedForwardON(t *testing.T) {
	db, _ := relationalDriverDB(t, false)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE empty_source(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	// SQLite may accept a future relation in an inner ON. This executor's
	// deliberately narrower contract rejects it before fetching any source row.
	// The same original statement is prepared and executed twice unchanged.
	for _, first := range []string{"a", "empty_source"} {
		for _, join := range []string{"JOIN", "INNER JOIN", "CROSS JOIN", "LEFT JOIN", "LEFT OUTER JOIN", ","} {
			for _, alias := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/alias_%t", first, join, alias), func(t *testing.T) {
					projection, condition := "x.id", "future.id=y.id"
					if alias {
						projection, condition = "future.id AS future_id", "future_id=y.id"
					}
					query := fmt.Sprintf("SELECT %s FROM %s x %s b y ON %s JOIN b future ON future.id=y.id", projection, first, join, condition)
					stmt, err := db.PrepareContext(ctx, query)
					if err != nil {
						t.Fatal(err)
					}
					defer stmt.Close()
					for range 2 {
						rows, err := stmt.QueryContext(ctx)
						if rows != nil {
							rows.Close()
							t.Fatal("forward ON returned Rows instead of a planning error")
						}
						if !errors.Is(err, sqlite.ErrUnsupported) {
							t.Fatalf("original query %q: got %v; want ErrUnsupported", query, err)
						}
					}
				})
			}
		}
	}
}
