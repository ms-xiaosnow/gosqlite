package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func runMigrateImport(ctx context.Context, args []string, out io.Writer, verify bool) error {
	flags := flag.NewFlagSet("migrate pgembedding import", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("input", "", "immutable CSV/NDJSON export file")
	manifest := flags.String("manifest", "", "bounded explicit export manifest")
	output := flags.String("output", "", "new SQLite destination; existing only with --resume")
	var apply, resume, dryRun bool
	var onError string
	var batchRows int
	var batchBytes, stopAfter int64
	if !verify {
		flags.BoolVar(&apply, "apply", false, "explicitly write local SQLite destination")
		flags.BoolVar(&resume, "resume", false, "resume matching committed job")
		flags.BoolVar(&dryRun, "dry-run", true, "read-only validation by default; --apply overrides omitted flag")
		flags.StringVar(&onError, "on-error", "stop", "stop or quarantine deterministic row errors")
		flags.IntVar(&batchRows, "batch-rows", 500, "bounded committed batch rows")
		flags.Int64Var(&batchBytes, "batch-bytes", 8<<20, "bounded decoded batch bytes")
		flags.Int64Var(&stopAfter, "stop-after-batches", 0, "deliberately return after committed batches, normal close")
	}
	seen := map[string]bool{}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		if seen[name] {
			return fmt.Errorf("duplicate migration option %q", name)
		}
		seen[name] = true
	}
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("invalid import options: %w", err)
	}
	if flags.NArg() != 0 || *input == "" || *manifest == "" || (apply || verify) && *output == "" || !verify && (apply && seen["dry-run"] && dryRun || !apply && !dryRun) {
		return fmt.Errorf("import requires --input and --manifest; local writes require --apply and --output, verification requires --output; explicit dry-run and apply conflict")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open(*manifest)
	if err != nil {
		return err
	}
	m, decodeErr := postgres.DecodeImportManifest(file)
	if err = errors.Join(decodeErr, file.Close()); err != nil {
		return err
	}
	var result postgres.ImportResult
	if verify {
		result, err = postgres.VerifyImport(ctx, *input, *output, m)
	} else {
		result, err = postgres.Import(ctx, *input, *output, m, postgres.ImportOptions{Apply: apply, Resume: resume, OnError: onError, BatchRows: batchRows, BatchBytes: batchBytes, StopAfterBatches: stopAfter})
	}
	if result.Status != "" {
		err = errors.Join(err, json.NewEncoder(out).Encode(result))
	}
	return err
}
