// Command migration-transition generates reviewed text from the synthetic M9
// fixture. It never connects to PostgreSQL or executes its generated SQL.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("migration-transition", flag.ContinueOnError)
	approach := flags.String("approach", "shadow", "shadow or retain")
	mode := flags.String("mode", "batches", "batches or single")
	dir := flags.String("out-dir", "", "new directory for review artifacts; default prints JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	f, err := os.Open("testdata/pgembedding-0.3.6/catalog.json")
	if err != nil {
		return err
	}
	c, err := postgres.DecodeCatalog(f, postgres.Limits{})
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	o := postgres.TransitionOptions{RelationOID: 30000, AttNum: 3, Dimensions: 3, Approach: postgres.TypedCopyShadow, Mode: postgres.TransitionBatches,
		VectorSchema: "public", TargetVersion: "0.8.7", ShadowColumn: "embedding_vector", StateSchema: "public", StateTable: "documents_vector_migration", BatchSize: 2,
		Indexes: []postgres.TransitionIndex{{SourceIndexOID: 30100, Name: "documents_vector_l2", Method: "hnsw", M: 16, EFConstruction: 64, EFSearch: 64}, {SourceIndexOID: 30101, Name: "documents_vector_cosine", Method: "hnsw", M: 16, EFConstruction: 64, EFSearch: 64}, {SourceIndexOID: 30102, Name: "documents_vector_l1", Method: "hnsw", M: 16, EFConstruction: 64, EFSearch: 64}}}
	switch *approach {
	case "shadow":
	case "retain":
		o.Approach = postgres.RetainArrayCast
		o.ShadowColumn = ""
	default:
		return fmt.Errorf("approach must be shadow or retain")
	}
	switch *mode {
	case "batches":
	case "single":
		o.Mode = postgres.TransitionSingle
	default:
		return fmt.Errorf("mode must be batches or single")
	}
	if o.Approach == postgres.RetainArrayCast || o.Mode == postgres.TransitionSingle {
		o.StateSchema = ""
		o.StateTable = ""
		o.BatchSize = 0
	}
	b, err := postgres.GenerateTransition(context.Background(), c, o)
	if err != nil {
		return err
	}
	if *dir == "" {
		return postgres.RenderTransitionJSON(out, b)
	}
	// This explicit output option creates a new directory, never overwrites one.
	// A failed write leaves a partial review bundle and returns an error; no
	// atomic publication or interrupted-publication qualification is claimed.
	if err = os.Mkdir(*dir, 0700); err != nil {
		return err
	}
	for _, a := range b.Artifacts {
		if filepath.Base(a.Name) != a.Name {
			return fmt.Errorf("invalid generated artifact name")
		}
		f, err := os.OpenFile(filepath.Join(*dir, a.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := io.WriteString(f, a.Content)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	_, err = fmt.Fprintf(out, "Generated %d review artifacts; status=%s; no PostgreSQL SQL executed.\n", len(b.Artifacts), b.Report.Status)
	return err
}
