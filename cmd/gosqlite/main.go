package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"io"
	"os"
	"os/signal"
	"unicode/utf8"
)

func run(args []string, out io.Writer) error { return runContext(context.Background(), args, out) }
func runContext(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		_, err := fmt.Fprintf(out, "gosqlite %s; Go %s; SQLite behavior baseline %s\n", buildinfo.Version, buildinfo.GoBaseline, buildinfo.SQLiteBaseline)
		return err
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
		_, err := fmt.Fprintln(out, "usage: gosqlite version\n       gosqlite query <db> '<sql>'\n       gosqlite inspect <db>\nSQLite SQL MVP. One statement per invocation; SELECT emits JSON Lines, writes emit a result.")
		return err
	}
	switch args[0] {
	case "query":
		if len(args) != 3 {
			return fmt.Errorf("usage: gosqlite query <db> '<sql>'")
		}
		stmt, err := sqlast.Parse(args[2])
		if err != nil {
			return err
		}
		if _, ok := stmt.(*sqlast.Transaction); ok {
			return fmt.Errorf("CLI executes one statement per connection; use the Go API for explicit transactions")
		}
		_, selectOnly := stmt.(*sqlast.Select)
		db, err := sqlite.Open(args[1], sqlite.Options{ReadOnly: selectOnly})
		if err != nil {
			return err
		}
		defer db.Close()
		if !selectOnly {
			result, err := db.Exec(ctx, args[2])
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			id, _ := result.LastInsertId()
			return json.NewEncoder(out).Encode(struct {
				RowsAffected int64 `json:"rows_affected"`
				LastInsertID int64 `json:"last_insert_id"`
			}{affected, id})
		}
		rows, err := db.Query(ctx, args[2])
		if err != nil {
			return err
		}
		defer rows.Close()
		enc := json.NewEncoder(out)
		if err = enc.Encode(struct {
			Columns []string `json:"columns"`
		}{rows.Columns()}); err != nil {
			return err
		}
		for rows.Next() {
			for _, v := range rows.Values() {
				if text, ok := v.(string); ok && !utf8.ValidString(text) {
					return fmt.Errorf("unsupported CLI JSON encoding: invalid UTF-8 text")
				}
			}
			if err = enc.Encode(struct {
				Row []any `json:"row"`
			}{rows.Values()}); err != nil {
				return err
			}
		}
		return rows.Err()
	case "inspect":
		if len(args) != 2 {
			return fmt.Errorf("usage: gosqlite inspect <db>")
		}
		db, err := sqlite.Open(args[1], sqlite.Options{ReadOnly: true})
		if err != nil {
			return err
		}
		defer db.Close()
		schema, err := db.Schema(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(struct {
			Format string               `json:"format"`
			Header sqlite.FileInfo      `json:"header"`
			Schema []sqlite.SchemaEntry `json:"schema"`
		}{"SQLite format 3 (metadata inspection only)", db.Info(), schema})
	default:
		return fmt.Errorf("unsupported command %q", args[0])
	}
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := runContext(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gosqlite:", err)
		os.Exit(1)
	}
}
