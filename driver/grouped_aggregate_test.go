package driver_test

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

// Normal prepared statements and rollback on intact ordinary/shared fixtures.
func TestDriverGroupedAggregatePreparedAndTransaction(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			db, _ := relationalDriverDB(t, shared)
			ctx := context.Background()
			stmt, err := db.PrepareContext(ctx, "SELECT a.k AS k,count(*) AS n,sum(b.score) AS total FROM a LEFT JOIN b ON a.k=b.k GROUP BY a.k HAVING count(*)>:minimum ORDER BY k LIMIT :limit")
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()
			want := [][]any{{int64(1), int64(4), float64(6)}}
			for range 3 {
				rows, err := stmt.QueryContext(ctx, sql.Named("limit", 9), sql.Named("minimum", 1))
				if err != nil {
					t.Fatal(err)
				}
				if got := relationalDriverRead(t, rows, []string{"k", "n", "total"}); !reflect.DeepEqual(got, want) {
					t.Fatal(got)
				}
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err = tx.ExecContext(ctx, "UPDATE b SET score=score+10 WHERE id=10"); err != nil {
				t.Fatal(err)
			}
			txstmt := tx.StmtContext(ctx, stmt)
			rows, err := txstmt.QueryContext(ctx, sql.Named("minimum", 1), sql.Named("limit", 9))
			if err != nil {
				t.Fatal(err)
			}
			if got := relationalDriverRead(t, rows, nil); !reflect.DeepEqual(got, [][]any{{int64(1), int64(4), float64(26)}}) {
				t.Fatal(got)
			}
			if err = txstmt.Close(); err != nil {
				t.Fatal(err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			rows, err = stmt.QueryContext(ctx, sql.Named("minimum", 1), sql.Named("limit", 9))
			if err != nil {
				t.Fatal(err)
			}
			if got := relationalDriverRead(t, rows, nil); !reflect.DeepEqual(got, want) {
				t.Fatal(got)
			}
		})
	}
}
