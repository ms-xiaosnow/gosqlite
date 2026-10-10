package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestSharedWALPooledConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared-pool.db")
	connector, err := driver.NewConnector(path, sqlite.Options{WALShared: true, KeepWALOnClose: true})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, "CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	stmt, err := db.PrepareContext(ctx, "INSERT INTO t VALUES(?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 1; i <= 50; i++ {
				id := worker*50 + i
				for {
					_, e := stmt.ExecContext(ctx, id, fmt.Sprint("row-", id))
					if e == nil {
						break
					}
					if !errors.Is(e, sqlite.ErrBusy) {
						errs <- e
						return
					}
					if e = ctx.Err(); e != nil {
						errs <- e
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	rows, err := db.QueryContext(ctx, "SELECT id,text FROM t ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		count++
		var id int
		var text string
		if err = rows.Scan(&id, &text); err != nil {
			t.Fatal(err)
		}
		if id != count || text != fmt.Sprint("row-", count) {
			t.Fatalf("row%d: %d %q", count, id, text)
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if count != 200 {
		t.Fatal(count)
	}
	if err = stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	native, err := sqlite.Open(path, sqlite.Options{WALShared: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = native.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if err = native.Close(); err != nil {
		t.Fatal(err)
	}
}
