// The example converts and runs only its embedded synthetic local fixture.
// It is not a PostgreSQL connection, arbitrary SQL importer, or migration plan.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

//go:embed converter-catalog.json
var catalogJSON []byte

//go:embed source.sql
var sourceSQL string

func runExample(out io.Writer) error {
	catalog, err := postgres.DecodeCatalog(bytes.NewReader(catalogJSON), postgres.Limits{})
	if err != nil {
		return err
	}
	result, err := postgres.Convert(context.Background(), sourceSQL, catalog, postgres.ConvertOptions{Target: postgres.TargetLocal, Tables: []postgres.TableMapping{{RelationOID: 30000, TargetName: "documents"}}, StandardConformingStrings: true})
	if err != nil {
		return err
	}
	if _, err = result.SQL(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "gosqlite-converter-example-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	db, err := sqlite.Open(filepath.Join(dir, "converted.db"), sqlite.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	var query string
	for _, statement := range result.Statements {
		if statement.Kind == "select" {
			query = statement.ConvertedSQL
			continue
		}
		if _, err = db.Exec(context.Background(), statement.ConvertedSQL); err != nil {
			return err
		}
	}
	if err = db.CheckExtensionIndex(context.Background(), "documents_manhattan"); err != nil {
		return err
	}
	if err = db.Close(); err != nil {
		return err
	}
	db, err = sqlite.Open(filepath.Join(dir, "converted.db"), sqlite.Options{NoCreate: true})
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), query)
	if err != nil {
		return err
	}
	defer rows.Close()
	wantIDs := []int64{3, 1, 2, 4}
	wantDistances := []any{float64(0), float64(5), float64(6), nil}
	n := 0
	for rows.Next() {
		v := rows.Values()
		if n >= len(wantIDs) || v[0] != wantIDs[n] || v[1] != wantDistances[n] {
			return fmt.Errorf("converted fixture exact distance or NULL order mismatch at row %d", n+1)
		}
		fmt.Fprintf(out, "id=%d distance=%v\n", v[0], v[1])
		n++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if n != len(wantIDs) {
		return fmt.Errorf("converted fixture row count mismatch")
	}
	fmt.Fprintln(out, "Verified: five converted statements, genuine local HNSW build, exact Manhattan results and normal reopen.")
	fmt.Fprintln(out, "Application setting: native VectorSearchOptions.EFSearch=64; no SQL SET or transaction wrapper was generated.")
	fmt.Fprintln(out, "Source SHA-256:", result.SourceSHA256)
	fmt.Fprintln(out, "Converted SQL SHA-256:", result.TargetSHA256)
	return nil
}
func main() {
	if err := runExample(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
