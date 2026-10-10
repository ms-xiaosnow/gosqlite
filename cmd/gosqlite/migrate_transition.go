package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

// Transition generation reads local metadata and emits review-only text. It has
// no database connection, apply flag, or generated-SQL execution path.
func runMigrateTransition(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("migrate pgembedding transition", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	catalogPath := flags.String("catalog", "", "offline catalog JSON")
	optionsPath := flags.String("options", "", "strict bounded transition options JSON")
	report := flags.String("report", "", "new review-bundle JSON file; never overwrite")
	seen := map[string]bool{}
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(strings.SplitN(a, "=", 2)[0], "-")
		if seen[name] {
			return fmt.Errorf("duplicate transition option %q", name)
		}
		seen[name] = true
	}
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("invalid transition options: %w", err)
	}
	if flags.NArg() != 0 || *catalogPath == "" || *optionsPath == "" {
		return fmt.Errorf("transition requires --catalog, --options and no positional arguments; generation never applies SQL")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(*catalogPath)
	if err != nil {
		return err
	}
	c, decodeErr := postgres.DecodeCatalog(f, postgres.Limits{})
	if err = errors.Join(decodeErr, f.Close()); err != nil {
		return err
	}
	f, err = os.Open(*optionsPath)
	if err != nil {
		return err
	}
	o, decodeErr := postgres.DecodeTransitionOptions(f)
	if err = errors.Join(decodeErr, f.Close()); err != nil {
		return err
	}
	bundle, err := postgres.GenerateTransition(ctx, c, o)
	if err != nil {
		return err
	}
	var content bytes.Buffer
	if err = postgres.RenderTransitionJSON(&content, bundle); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if *report == "" {
		_, err = out.Write(content.Bytes())
		return err
	}
	dst, err := os.OpenFile(*report, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := dst.Write(content.Bytes())
	if err = errors.Join(writeErr, dst.Close()); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "review bundle written; no database was opened or SQL executed")
	return err
}
