package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const inspectUsage = "usage: gosqlite inspect [--wal-readonly] [--extensions] <db> [--key-provider <provider>] [--mode snapshot|wal|shared]"

type inspectCommandOptions struct {
	path, provider          string
	mode                    sqlite.EncryptionMode
	walReadOnly, extensions bool
}

// Preserve the legacy leading flags. Encryption uses explicit trailing pairs;
// parser failures never echo submitted flags, paths, descriptors or values.
func parseInspect(args []string) (inspectCommandOptions, error) {
	flags, tail, err := parseReadOnlyWALFlags(args, true)
	if err != nil || len(tail) == 0 || tail[0] == "" || flags.walExclusive || flags.walShared {
		return inspectCommandOptions{}, errors.New(inspectUsage)
	}
	o := inspectCommandOptions{path: tail[0], walReadOnly: flags.walReadOnly, extensions: flags.extensions}
	seen := map[string]bool{}
	for args = tail[1:]; len(args) > 0; args = args[2:] {
		if len(args) < 2 || args[1] == "" || seen[args[0]] {
			return inspectCommandOptions{}, errors.New(inspectUsage)
		}
		seen[args[0]] = true
		switch args[0] {
		case "--key-provider":
			o.provider = args[1]
		case "--mode":
			switch args[1] {
			case "snapshot":
				o.mode = sqlite.EncryptionSnapshot
			case "wal":
				o.mode = sqlite.EncryptionWAL
			case "shared":
				o.mode = sqlite.EncryptionWALShared
			default:
				return inspectCommandOptions{}, errors.New(inspectUsage)
			}
		default:
			return inspectCommandOptions{}, errors.New(inspectUsage)
		}
	}
	if seen["--mode"] && o.provider == "" || o.provider != "" && o.walReadOnly {
		return inspectCommandOptions{}, errors.New(inspectUsage)
	}
	return o, nil
}

func runInspect(ctx context.Context, args []string, out io.Writer) error {
	return runInspectWithInput(ctx, args, out, credentialInput{lookupEnv: os.LookupEnv, stdin: func() (io.Reader, error) {
		info, err := os.Stdin.Stat()
		if err != nil || (!info.Mode().IsRegular() && info.Mode()&os.ModeNamedPipe == 0) {
			return nil, errors.New("stdin must be a noninteractive pipe or redirected regular file")
		}
		return os.Stdin, nil
	}})
}

func runInspectWithInput(ctx context.Context, args []string, out io.Writer, input credentialInput) error {
	return runInspectWithOpener(ctx, args, out, input, openInspectDB)
}

// The callback is command-local: models can check ownership without a native
// descriptor. The production route always uses the public context-aware open.
type inspectDB interface {
	Inspect(context.Context, sqlite.InspectOptions) (sqlite.InspectResult, error)
	Close() error
}

type inspectOpener func(context.Context, string, sqlite.Options) (inspectDB, error)

func openInspectDB(ctx context.Context, path string, opts sqlite.Options) (inspectDB, error) {
	return sqlite.OpenContext(ctx, path, opts)
}

func runInspectWithOpener(ctx context.Context, args []string, out io.Writer, input credentialInput, open inspectOpener) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o, err := parseInspect(args)
	if err != nil {
		return err
	}
	var provider *cliKeyProvider
	if o.provider != "" {
		provider, err = newCLIKeyProvider(o.provider, input)
		if err != nil {
			return err
		}
		// This runner owns this provider. The executor only borrows it, and
		// returns after its checked Close attempt, before any stdout is attempted.
		// The core copies/releases provider borrows synchronously. A failed
		// Close can retain native cleanup owners, but cleanup never reacquires
		// credentials. Clearing this provider proves no native retirement.
		defer provider.close()
	}
	result, err := executeInspect(ctx, o, provider, open)
	if err == nil {
		err = json.NewEncoder(out).Encode(result)
	}
	if err != nil && provider != nil {
		return inspectError(err)
	}
	return err
}

func executeInspect(ctx context.Context, o inspectCommandOptions, provider *cliKeyProvider, open inspectOpener) (any, error) {
	opts := sqlite.Options{ReadOnly: true, NoCreate: true, WALShared: o.walReadOnly}
	if provider != nil {
		opts.Encryption = &sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: provider.kind, Mode: o.mode}
		opts.KeepWALOnClose = o.mode != sqlite.EncryptionSnapshot
	}
	db, err := open(ctx, o.path, opts)
	if err != nil {
		return nil, err
	}
	inspection, inspectErr := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: o.extensions})
	// Attempt Close exactly once. A close error can retain native owners; do
	// not claim retirement or retry. No successful result may escape before a
	// successful Close, even when Inspect succeeded. Preserve both errors.
	if err := errors.Join(inspectErr, db.Close()); err != nil {
		return nil, err
	}
	return inspectOutput(o, provider != nil, inspection), nil
}

func inspectOutput(o inspectCommandOptions, encrypted bool, inspection sqlite.InspectResult) any {
	if encrypted {
		mode := "snapshot"
		if o.mode == sqlite.EncryptionWAL {
			mode = "wal"
		} else if o.mode == sqlite.EncryptionWALShared {
			mode = "shared"
		}
		if o.extensions {
			return struct {
				Format           string                          `json:"format"`
				Mode             string                          `json:"mode"`
				MetadataScope    string                          `json:"metadata_scope"`
				Header           sqlite.FileInfo                 `json:"header"`
				Schema           []sqlite.SchemaEntry            `json:"schema"`
				ExtensionIndexes []sqlite.ExtensionIndexMetadata `json:"extension_indexes"`
			}{"gosqlite-encrypted-family", mode, "logical SQLite metadata inspection only", inspection.Header, inspection.Schema, inspection.ExtensionIndexes}
		}
		return struct {
			Format        string               `json:"format"`
			Mode          string               `json:"mode"`
			MetadataScope string               `json:"metadata_scope"`
			Header        sqlite.FileInfo      `json:"header"`
			Schema        []sqlite.SchemaEntry `json:"schema"`
		}{"gosqlite-encrypted-family", mode, "logical SQLite metadata inspection only", inspection.Header, inspection.Schema}
	}
	// Keep the exact legacy plaintext field ordering, labels and omission rules.
	if o.extensions {
		return struct {
			Format           string                          `json:"format"`
			Header           sqlite.FileInfo                 `json:"header"`
			Schema           []sqlite.SchemaEntry            `json:"schema"`
			ExtensionIndexes []sqlite.ExtensionIndexMetadata `json:"extension_indexes"`
		}{"SQLite format 3 (metadata inspection only)", inspection.Header, inspection.Schema, inspection.ExtensionIndexes}
	}
	return struct {
		Format string               `json:"format"`
		Header sqlite.FileInfo      `json:"header"`
		Schema []sqlite.SchemaEntry `json:"schema"`
	}{"SQLite format 3 (metadata inspection only)", inspection.Header, inspection.Schema}
}

// Like maintenanceError, retain only safe sentinels, with uncertainty first.
// Inspect-specific wording avoids rekey/destination advice. Neither arbitrary
// native/provider/writer errors nor their secret-bearing chains are retained.
func inspectError(err error) error {
	for _, candidate := range []struct {
		cause error
		text  string
	}{
		{sqlite.ErrCommitUncertain, "storage state uncertain; do not retry automatically"},
		{sqlite.ErrPoisoned, "storage ownership is poisoned; do not retry automatically"},
		{sqlite.ErrRecoveryRequired, "storage requires recovery; no recovery was attempted"},
		{sqlite.ErrUnsupported, "unsupported source, mode, platform or operation"},
		{encryption.ErrKeyProvider, "credential provider unavailable or invalid"},
		{encryption.ErrCredential, "credential provider unavailable or invalid"},
		{sqlite.ErrAuthentication, "authentication failed"},
		{sqlite.ErrCorrupt, "source validation failed"},
		{sqlite.ErrBusy, "database is busy"},
		{sqlite.ErrMemoryLimit, "resource limit exceeded"},
		{sqlite.ErrFull, "storage limit exceeded"},
		{context.Canceled, "operation canceled"},
		{context.DeadlineExceeded, "deadline exceeded"},
	} {
		if errors.Is(err, candidate.cause) {
			return fmt.Errorf("inspect: %s: %w", candidate.text, candidate.cause)
		}
	}
	return errors.New("inspect failed; metadata output may be incomplete")
}
