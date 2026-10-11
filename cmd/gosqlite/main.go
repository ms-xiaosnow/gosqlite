package main

import (
	"context"
	"encoding/json"
	"errors"
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
func runContext(ctx context.Context, args []string, out io.Writer) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		_, err := fmt.Fprintf(out, "gosqlite %s; Go %s; SQLite behavior baseline %s\n", buildinfo.Version, buildinfo.GoBaseline, buildinfo.SQLiteBaseline)
		return err
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
		_, err := fmt.Fprintln(out, "usage: gosqlite version\n       gosqlite query [--wal-exclusive|--wal-shared] <db> '<sql>'\n       gosqlite inspect [--extensions] <db>\n       gosqlite backup <db> --out <new-archive> --key-provider <provider> [--out-key-provider <provider>] [--mode snapshot|wal|shared]\n       gosqlite rekey <db> --key-provider <new-provider> --current-key-provider <provider> [--mode snapshot|wal|shared]\n       gosqlite migrate pgembedding inspect --catalog <catalog.json> [--format json|markdown]\n       gosqlite migrate pgembedding --input <export> --manifest <manifest.json> [--output <new.db> --apply [--resume]]\n       gosqlite migrate pgembedding verify --input <export> --manifest <manifest.json> --output <db>\n       gosqlite migrate pgembedding transition --catalog <catalog.json> --options <options.json> [--report <new.json>]\n       gosqlite check <db> [<extension-index>] [--key-provider <provider> --mode snapshot|wal|shared]\n       gosqlite check [--wal-exclusive|--wal-shared] <db> <extension-index>\nSQLite SQL MVP. One statement per invocation; SELECT emits JSON Lines, writes emit a result.\nEncrypted maintenance: env-hex:NAME, env-password:NAME, stdin-hex, stdin-password. See docs/cli-encryption.md; plaintext backup is unsupported.")
		return err
	}
	switch args[0] {
	case "backup", "rekey":
		return runMaintenance(ctx, args[0], args[1:], out)
	case "migrate":
		return runMigrate(ctx, args[1:], out)
	case "query":
		walExclusive := len(args) > 1 && args[1] == "--wal-exclusive"
		walShared := len(args) > 1 && args[1] == "--wal-shared"
		if walExclusive || walShared {
			args = append([]string{args[0]}, args[2:]...)
		}
		if len(args) != 3 {
			return fmt.Errorf("usage: gosqlite query [--wal-exclusive|--wal-shared] <db> '<sql>'")
		}
		stmt, err := sqlast.Parse(args[2])
		if err != nil {
			return err
		}
		if _, ok := stmt.(*sqlast.Transaction); ok {
			return fmt.Errorf("CLI executes one statement per connection; use the Go API for explicit transactions")
		}
		_, selectOnly := stmt.(*sqlast.Select)
		db, err := sqlite.OpenContext(ctx, args[1], sqlite.Options{ReadOnly: selectOnly && !walExclusive && !walShared, NoCreate: selectOnly, WALExclusive: walExclusive, WALShared: walShared})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
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
		defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
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
	case "check":
		return runCheck(ctx, args[1:], out)
	case "inspect":
		includeExtensions := len(args) > 1 && args[1] == "--extensions"
		if includeExtensions {
			args = append([]string{args[0]}, args[2:]...)
		}
		if len(args) != 2 {
			return fmt.Errorf("usage: gosqlite inspect [--extensions] <db>")
		}
		db, err := sqlite.OpenContext(ctx, args[1], sqlite.Options{ReadOnly: true})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
		inspection, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: includeExtensions})
		if err != nil {
			return err
		}
		if includeExtensions {
			return json.NewEncoder(out).Encode(struct {
				Format           string                          `json:"format"`
				Header           sqlite.FileInfo                 `json:"header"`
				Schema           []sqlite.SchemaEntry            `json:"schema"`
				ExtensionIndexes []sqlite.ExtensionIndexMetadata `json:"extension_indexes"`
			}{"SQLite format 3 (metadata inspection only)", inspection.Header, inspection.Schema, inspection.ExtensionIndexes})
		}
		return json.NewEncoder(out).Encode(struct {
			Format string               `json:"format"`
			Header sqlite.FileInfo      `json:"header"`
			Schema []sqlite.SchemaEntry `json:"schema"`
		}{"SQLite format 3 (metadata inspection only)", inspection.Header, inspection.Schema})
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
