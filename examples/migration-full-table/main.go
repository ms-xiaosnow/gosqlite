// Command migration-full-table prints the fixed synthetic whole-table review
// bundle. No database, SQL execution or remote interface is provided.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "This fixed synthetic example takes no arguments; use the transition CLI for explicit input paths.")
		os.Exit(1)
	}
	if err := run("examples/migration-full-table", os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(root string, out io.Writer) error {
	read := func(name string, decode func(io.Reader) error) error {
		f, err := os.Open(filepath.Join(root, name))
		if err != nil {
			return err
		}
		return errors.Join(decode(f), f.Close())
	}
	var c postgres.CatalogSnapshot
	var o postgres.FullTableOptions
	var e postgres.FullTableEvidence
	var err error
	if err = read("catalog.json", func(r io.Reader) error { c, err = postgres.DecodeCatalog(r, postgres.Limits{}); return err }); err != nil {
		return err
	}
	if err = read("options.json", func(r io.Reader) error { o, err = postgres.DecodeFullTableOptions(r); return err }); err != nil {
		return err
	}
	if err = read("evidence.json", func(r io.Reader) error { e, err = postgres.DecodeFullTableEvidence(r); return err }); err != nil {
		return err
	}
	b, err := postgres.GenerateFullTableTransition(context.Background(), c, o, e)
	if err != nil {
		return err
	}
	return postgres.RenderTransitionJSON(out, b)
}
