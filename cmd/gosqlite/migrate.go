package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
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
	if len(args) > 0 && args[0] == "convert" {
		return runMigrateConvert(ctx, args[1:], out)
	}
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

// Conversion reads source files and writes only an optional new text artifact.
// It never opens a database or executes the input or converted statements.
func runMigrateConvert(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("migrate pgembedding convert", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	catalogPath := flags.String("catalog", "", "offline catalog JSON")
	inputPath := flags.String("input", "", "bounded source SQL file")
	target := flags.String("target", "", "local or postgres")
	table := flags.String("table", "", "explicit source relation OID=target table name")
	schema := flags.String("target-schema", "", "PostgreSQL target relation schema")
	vectorSchema := flags.String("vector-schema", "", "reviewed intended pgvector extension schema")
	standard := flags.Bool("standard-conforming-strings", true, "source ordinary string semantics (must remain true)")
	format := flags.String("format", "json", "json or sql; unsupported scripts never produce sql output")
	report := flags.String("report", "", "new report/SQL file, never overwrite")
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
		return fmt.Errorf("invalid conversion options: %w", err)
	}
	if flags.NArg() != 0 || *catalogPath == "" || *inputPath == "" || *table == "" || (*target != "local" && *target != "postgres") || (*format != "json" && *format != "sql") || !*standard {
		return fmt.Errorf("conversion requires --catalog, --input, --target local|postgres, --table OID=name and --format json|sql with standard-conforming-strings=true")
	}
	parts := strings.SplitN(*table, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("table mapping must be OID=name")
	}
	oid, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil || oid == 0 {
		return fmt.Errorf("table mapping needs a positive relation OID")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open(*catalogPath)
	if err != nil {
		return err
	}
	catalog, decodeErr := postgres.DecodeCatalog(file, postgres.Limits{})
	closeErr := file.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	file, err = os.Open(*inputPath)
	if err != nil {
		return err
	}
	source, readErr := io.ReadAll(io.LimitReader(file, postgres.MaxConvertSQLBytes+1))
	closeErr = file.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	result, err := postgres.Convert(ctx, string(source), catalog, postgres.ConvertOptions{Target: postgres.SQLTarget(*target), Tables: []postgres.TableMapping{{RelationOID: uint32(oid), TargetName: parts[1], TargetSchema: *schema}}, VectorSchema: *vectorSchema, StandardConformingStrings: true})
	if err != nil {
		return err
	}
	// Prepare fully before creating any artifact. SQL mode cannot mix converted
	// statements and unsupported original evidence into an apply-ready script.
	var content strings.Builder
	if *format == "json" {
		err = postgres.RenderConversionJSON(&content, result)
	} else {
		var sql string
		sql, err = result.SQL()
		if err == nil {
			if *target == "postgres" {
				content.WriteString("-- REVIEW ONLY: PostgreSQL text was not executed. Review extension transition, settings and bind transformations in the JSON report.\n")
			} else {
				content.WriteString("-- Local parser-checked SQL. Review separate JSON bind transformations and application EFSearch settings before use.\n")
			}
			content.WriteString(sql)
		}
	}
	if err != nil {
		return err
	}
	if *report == "" {
		_, err = io.WriteString(out, content.String())
	} else {
		dst, e := os.OpenFile(*report, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, writeErr := io.WriteString(dst, content.String())
		closeErr := dst.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		_, err = fmt.Fprintln(out, "conversion artifact written; no database was opened and no SQL was executed")
	}
	if err != nil {
		return err
	}
	if result.Status != "converted" {
		return fmt.Errorf("conversion has unsupported statements; original SQL is report evidence only")
	}
	return nil
}
