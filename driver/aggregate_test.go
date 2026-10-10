package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"reflect"
	"testing"
)

// Ordinary prepared statements, read-your-write/rollback, close and quotas on
// intact files. Both driver modes share the native aggregate executor.
func TestDriverGlobalAggregatePreparedAndTransaction(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared_%t", shared), func(t *testing.T) {
			db, _ := relationalDriverDB(t, shared)
			ctx := context.Background()
			stmt, err := db.PrepareContext(ctx, "SELECT count(*) AS n,sum(b.score) AS total,max(b.payload) AS payload FROM a LEFT JOIN b ON a.k=b.k WHERE a.id<=:maximum ORDER BY n LIMIT :limit")
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()
			want := [][]any{{int64(6), float64(9), []byte("2")}}
			for range 3 {
				rows, err := stmt.QueryContext(ctx, sql.Named("limit", 1), sql.Named("maximum", 4))
				if err != nil {
					t.Fatal(err)
				}
				got := relationalDriverRead(t, rows, []string{"n", "total", "payload"})
				if !reflect.DeepEqual(got, want) {
					t.Fatal(got, want)
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
			txstmt := tx.StmtContext(ctx, stmt)
			rows, err := txstmt.QueryContext(ctx, sql.Named("maximum", 4), sql.Named("limit", 1))
			if err != nil {
				t.Fatal(err)
			}
			got := relationalDriverRead(t, rows, nil)
			if !reflect.DeepEqual(got, [][]any{{int64(6), float64(29), []byte("2")}}) {
				t.Fatal(got)
			}
			if err := txstmt.Close(); err != nil {
				t.Fatal(err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			rows, err = stmt.QueryContext(ctx, sql.Named("maximum", 4), sql.Named("limit", 1))
			if err != nil {
				t.Fatal(err)
			}
			if got := relationalDriverRead(t, rows, nil); !reflect.DeepEqual(got, want) {
				t.Fatal(got)
			}
			var count int64
			var sum, avg sql.NullFloat64
			var total float64
			if err := db.QueryRow("SELECT count(*),sum(score),avg(score),total(score) FROM b WHERE 0").Scan(&count, &sum, &avg, &total); err != nil || count != 0 || sum.Valid || avg.Valid || total != 0 {
				t.Fatal(count, sum, avg, total, err)
			}
			rows, err = db.Query("SELECT count(*) FROM a,b")
			if err != nil {
				t.Fatal(err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("UPDATE a SET label='after aggregate close' WHERE id=1"); err != nil {
				t.Fatal(err)
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if _, err := db.QueryContext(canceled, "SELECT count(*) FROM a,b"); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if _, err := db.QueryContext(sqlite.WithRelationalLimits(ctx, sqlite.RelationalLimits{MaxWork: 1}), "SELECT count(*) FROM a"); !errors.Is(err, sqlite.ErrQueryWorkLimit) {
				t.Fatal(err)
			}
		})
	}
}
