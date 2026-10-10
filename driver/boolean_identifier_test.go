package driver_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverBooleanIdentifierPrepared(t *testing.T) {
	connector, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "boolean-names.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{`CREATE TABLE true(id INTEGER PRIMARY KEY,false INTEGER)`, `INSERT INTO true VALUES(1,7),(2,9)`} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	stmt, err := db.Prepare(`SELECT :value AS true, false AS false FROM true false WHERE true IS :value AND false.id=?2 ORDER BY true`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for _, v := range []any{int64(0), int64(1), nil, "x", int64(0)} {
		var a, b any
		if err = stmt.QueryRow(sql.Named("value", v), int64(2)).Scan(&a, &b); err != nil {
			t.Fatal(v, err)
		}
		if a != v || b != int64(9) {
			t.Fatal(v, a, b)
		}
	}
	// Preparing and reusing the same AST does not turn Boolean fallback into an
	// integer ORDER BY ordinal, or mutate aliases into prior binding values.
	constant, err := db.Prepare(`SELECT ?1 AS false FROM true WHERE false=?2 ORDER BY false`)
	if err != nil {
		t.Fatal(err)
	}
	defer constant.Close()
	for _, v := range []int64{7, 9, 7} {
		var got int64
		if err = constant.QueryRow(v+10, v).Scan(&got); err != nil || got != v+10 {
			t.Fatal(got, err)
		}
	}
	if _, err = stmt.Query(sql.Named("value", int64(1))); err == nil {
		t.Fatal("missing ?2 binding accepted")
	}
	if _, err = db.Exec(`UPDATE true SET false=:false WHERE id=:true`, sql.Named("false", int64(11)), sql.Named("true", int64(1))); err != nil {
		t.Fatal(err)
	}
	var got int64
	if err = db.QueryRow(`SELECT false FROM true WHERE id=1`).Scan(&got); err != nil || got != 11 {
		t.Fatal(got, err)
	}
}
