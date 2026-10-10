package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

// The inspector deliberately has no database handle or execution path. Later
// apply/resume modes must remain separate, explicit commands.
func runMigrate(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "pgembedding" {
		return fmt.Errorf("usage: gosqlite migrate pgembedding inspect --catalog <catalog.json> [--format json|markdown]; inspection is always read-only")
	}
	args = args[1:]
	if len(args) > 0 && args[0] == "inspect" {
		args = args[1:]
	}
	flags := flag.NewFlagSet("migrate pgembedding inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	catalog := flags.String("catalog", "", "versioned offline catalog JSON")
	format := flags.String("format", "json", "json or markdown")
	dryRun := flags.Bool("dry-run", true, "read-only inspection (must remain true)")
	report := flags.String("report", "", "new report file (never overwrite)")
	// Flag package accepts repeated options by default; reject them before parsing.
	seen := map[string]bool{}
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(strings.SplitN(a, "=", 2)[0], "-")
		if seen[name] {
			return fmt.Errorf("duplicate migration option %q", name)
		}
		seen[name] = true
	}
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("invalid migration options: %w", err)
	}
	if flags.NArg() != 0 || *catalog == "" || !*dryRun || (*format != "json" && *format != "markdown") {
		return fmt.Errorf("inspection requires --catalog, --dry-run=true, no positional arguments and --format json|markdown")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(*catalog)
	if err != nil {
		return err
	}
	c, decodeErr := postgres.DecodeCatalog(f, postgres.Limits{})
	closeErr := f.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	r, err := postgres.Inspect(ctx, c, postgres.InspectOptions{})
	if err != nil {
		return err
	}
	render := postgres.RenderJSON
	if *format == "markdown" {
		render = postgres.RenderMarkdown
	}
	if *report == "" {
		return render(out, r)
	}
	// Reports are optional artifacts. No file is created until catalog validation
	// and inspection succeed; an existing file, including a database, is refused.
	dst, err := os.OpenFile(*report, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	writeErr := render(dst, r)
	closeErr = dst.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	_, err = fmt.Fprintln(out, "inspection report written; no database was opened")
	return err
}
