package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func runMigrateImportDistances(ctx context.Context, input, output, optionsPath string, m postgres.ImportManifest, out io.Writer) error {
	file, err := os.Open(optionsPath)
	if err != nil {
		return err
	}
	options, decodeErr := postgres.DecodeImportDistanceOptions(file)
	if err = errors.Join(decodeErr, file.Close()); err != nil {
		return err
	}
	result, err := postgres.VerifyImportDistances(ctx, input, output, m, options)
	return errors.Join(err, json.NewEncoder(out).Encode(result))
}
