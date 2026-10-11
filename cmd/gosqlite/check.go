package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const checkUsage = "usage: gosqlite check [--wal-readonly] <db> [<extension-index>] [--key-provider <provider>] [--mode snapshot|wal|shared]; named extension checks also accept leading --wal-exclusive or --wal-shared"

type checkCommandOptions struct {
	path, index, provider   string
	mode                    sqlite.EncryptionMode
	walExclusive, walShared, walReadOnly bool
}

func parseCheck(args []string) (checkCommandOptions, error) {
	var o checkCommandOptions
	if len(args) > 0 && (args[0] == "--wal-exclusive" || args[0] == "--wal-shared" || args[0] == "--wal-readonly") {
		o.walExclusive = args[0] == "--wal-exclusive"
		o.walShared = args[0] == "--wal-shared"
		o.walReadOnly = args[0] == "--wal-readonly"
		args = args[1:]
	}
	if len(args) == 0 || args[0] == "" || strings.HasPrefix(args[0], "--") {
		return o, errors.New(checkUsage)
	}
	o.path = args[0]
	args = args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		o.index = args[0]
		args = args[1:]
		if o.index == "" {
			return checkCommandOptions{}, errors.New(checkUsage)
		}
	}
	seen := map[string]bool{}
	for len(args) > 0 {
		if len(args) < 2 || args[1] == "" || seen[args[0]] {
			return checkCommandOptions{}, errors.New(checkUsage)
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
				return checkCommandOptions{}, errors.New(checkUsage)
			}
		default:
			return checkCommandOptions{}, errors.New(checkUsage)
		}
		args = args[2:]
	}
	if seen["--mode"] && o.provider == "" || o.provider != "" && (o.walExclusive || o.walShared || o.walReadOnly) {
		return checkCommandOptions{}, errors.New(checkUsage)
	}
	if o.index == "" && (o.walExclusive || o.walShared) {
		return checkCommandOptions{}, fmt.Errorf("%w: full check requires a read-only open; --wal-exclusive/--wal-shared require writable coordination; use --wal-readonly for the Linux live-index subset or DB.Check on an existing configured connection", sqlite.ErrUnsupported)
	}
	return o, nil
}

func runCheck(ctx context.Context, args []string, out io.Writer) error {
	return runCheckWithInput(ctx, args, out, credentialInput{lookupEnv: os.LookupEnv, stdin: func() (io.Reader, error) {
		info, err := os.Stdin.Stat()
		if err != nil || (!info.Mode().IsRegular() && info.Mode()&os.ModeNamedPipe == 0) {
			return nil, errors.New("stdin must be a noninteractive pipe or redirected regular file")
		}
		return os.Stdin, nil
	}})
}
func runCheckWithInput(ctx context.Context, args []string, out io.Writer, input credentialInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o, err := parseCheck(args)
	if err != nil {
		return err
	}
	var provider *cliKeyProvider
	if o.provider != "" {
		provider, err = newCLIKeyProvider(o.provider, input)
		if err != nil {
			return err
		}
		defer provider.close()
	}
	result, err := executeCheck(ctx, o, provider)
	if err != nil {
		if provider != nil {
			return maintenanceError("check", err)
		}
		return err
	}
	return json.NewEncoder(out).Encode(result)
}
func executeCheck(ctx context.Context, o checkCommandOptions, provider *cliKeyProvider) (_ any, resultErr error) {
	opts := sqlite.Options{ReadOnly: !o.walExclusive && !o.walShared, NoCreate: true, WALExclusive: o.walExclusive, WALShared: o.walShared || o.walReadOnly, KeepWALOnClose: o.walExclusive || o.walShared}
	if provider != nil {
		opts.Encryption = &sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: provider.kind, Mode: o.mode}
		opts.KeepWALOnClose = o.mode != sqlite.EncryptionSnapshot
	}
	db, err := sqlite.OpenContext(ctx, o.path, opts)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	if o.index != "" {
		if err = db.CheckExtensionIndex(ctx, o.index); err != nil {
			return nil, err
		}
		return struct {
			Status string `json:"status"`
			Scope  string `json:"scope"`
			Index  string `json:"index"`
		}{"ok", "extension source/posting/frequency consistency only", o.index}, nil
	}
	result, err := db.Check(ctx)
	if err != nil {
		return nil, err
	}
	return struct {
		Status string `json:"status"`
		sqlite.CheckResult
	}{"ok", result}, nil
}
