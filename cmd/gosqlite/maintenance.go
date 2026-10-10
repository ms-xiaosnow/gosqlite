package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const backupUsage = "usage: gosqlite backup <db> --out <new-archive> --key-provider <provider> [--out-key-provider <provider>] [--mode snapshot|wal|shared]"
const rekeyUsage = "usage: gosqlite rekey <db> --key-provider <new-provider> --current-key-provider <provider> [--mode snapshot|wal|shared]"
const providerHelp = "providers: env-hex:NAME, env-password:NAME, stdin-hex, stdin-password; credentials must never be arguments"

// The maintenance parser deliberately never formats an argument, including an
// unknown flag or invalid descriptor. A user may have pasted a secret there.
// Unlike flag.FlagSet, it also never prints a rejected flag value.
type maintenanceOptions struct {
	path, destination string
	source, target    string
	mode              sqlite.EncryptionMode
}

func parseMaintenance(command string, args []string) (maintenanceOptions, error) {
	var o maintenanceOptions
	usage := backupUsage
	if command == "rekey" {
		usage = rekeyUsage
	}
	if len(args) == 0 || args[0] == "" || strings.HasPrefix(args[0], "--") {
		return o, errors.New(usage)
	}
	o.path = args[0]
	seen := make(map[string]bool)
	var key string
	for i := 1; i < len(args); i += 2 {
		flag := args[i]
		if i+1 >= len(args) || args[i+1] == "" || seen[flag] {
			return maintenanceOptions{}, errors.New(usage)
		}
		seen[flag] = true
		value := args[i+1]
		switch flag {
		case "--key-provider":
			key = value
		case "--out":
			if command != "backup" {
				return maintenanceOptions{}, errors.New(usage)
			}
			o.destination = value
		case "--out-key-provider":
			if command != "backup" {
				return maintenanceOptions{}, errors.New(usage)
			}
			o.target = value
		case "--current-key-provider":
			if command != "rekey" {
				return maintenanceOptions{}, errors.New(usage)
			}
			o.source = value
		case "--mode":
			switch value {
			case "snapshot":
				o.mode = sqlite.EncryptionSnapshot
			case "wal":
				o.mode = sqlite.EncryptionWAL
			case "shared":
				o.mode = sqlite.EncryptionWALShared
			default:
				return maintenanceOptions{}, errors.New("mode must be snapshot, wal or shared")
			}
		default:
			return maintenanceOptions{}, errors.New(usage)
		}
	}
	if command == "backup" {
		if o.destination == "" {
			return maintenanceOptions{}, errors.New(usage)
		}
		if key == "" {
			return maintenanceOptions{}, fmt.Errorf("%w: backup currently requires an encrypted source and --key-provider; plaintext backup is not implemented", sqlite.ErrUnsupported)
		}
		o.source = key
		// Omission deliberately reuses the same provider instance. In particular,
		// a single stdin credential can protect both source and archive.
	} else {
		o.target = key
		if o.source == "" || o.target == "" {
			return maintenanceOptions{}, errors.New(rekeyUsage)
		}
	}
	return o, nil
}

func runMaintenance(ctx context.Context, command string, args []string, out io.Writer) error {
	return runMaintenanceWithInput(ctx, command, args, out, credentialInput{
		lookupEnv: os.LookupEnv,
		stdin: func() (io.Reader, error) {
			info, err := os.Stdin.Stat()
			if err != nil || (!info.Mode().IsRegular() && info.Mode()&os.ModeNamedPipe == 0) {
				return nil, errors.New("stdin must be a noninteractive pipe or redirected regular file")
			}
			return os.Stdin, nil
		},
	})
}

func runMaintenanceWithInput(ctx context.Context, command string, args []string, out io.Writer, input credentialInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o, err := parseMaintenance(command, args)
	if err != nil {
		return err
	}
	source, err := newCLIKeyProvider(o.source, input)
	if err != nil {
		return err
	}
	defer source.close()
	target := source
	if o.target != "" {
		target, err = newCLIKeyProvider(o.target, input)
		if err != nil {
			return err
		}
		defer target.close()
		if source.fromStdin && target.fromStdin {
			return errors.New("only one explicit stdin provider is allowed per command; use an environment provider for the other credential")
		}
	}
	result, err := executeMaintenance(ctx, command, o, source, target)
	if err != nil {
		return maintenanceError(command, err)
	}
	// The connection (including its cleanup) has completed before emitting ok.
	// An output error does not undo a completed backup/rekey.
	if err := json.NewEncoder(out).Encode(result); err != nil {
		return errors.New("maintenance completed but the result could not be written; do not repeat the operation automatically")
	}
	return nil
}

type maintenanceResult struct {
	Status         string `json:"status"`
	Operation      string `json:"operation"`
	Format         string `json:"format"`
	Mode           string `json:"mode"`
	PageSize       uint32 `json:"page_size"`
	PageCount      uint32 `json:"page_count"`
	RetainsHistory bool   `json:"retains_history"`
}

func executeMaintenance(ctx context.Context, command string, o maintenanceOptions, source, target *cliKeyProvider) (_ maintenanceResult, resultErr error) {
	sourceOptions := sqlite.EncryptionOptions{KeyProvider: source, CredentialKind: source.kind, Mode: o.mode}
	db, err := sqlite.OpenContext(ctx, o.path, sqlite.Options{
		Encryption: &sourceOptions, ReadOnly: command == "backup", NoCreate: true,
		// Avoid an incidental checkpoint on close in the rekey route. Rekey
		// itself publishes a full mode-preserving base through the native API.
		KeepWALOnClose: o.mode != sqlite.EncryptionSnapshot,
	})
	if err != nil {
		return maintenanceResult{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	targetOptions := sqlite.EncryptionOptions{KeyProvider: target, CredentialKind: target.kind, Mode: o.mode}
	var info sqlite.EncryptionInfo
	format := "gosqlite-encrypted-family"
	if command == "backup" {
		targetOptions.Mode = sqlite.EncryptionSnapshot
		info, err = db.BackupEncrypted(ctx, o.destination, targetOptions)
		format = "gosqlite-encrypted-archive"
	} else {
		err = db.Rekey(ctx, targetOptions)
		if err == nil {
			info, err = db.VerifyEncrypted(ctx)
		}
	}
	if err != nil {
		return maintenanceResult{}, err
	}
	mode := "snapshot"
	if info.Mode == sqlite.EncryptionWAL {
		mode = "wal"
	} else if info.Mode == sqlite.EncryptionWALShared {
		mode = "shared"
	}
	return maintenanceResult{"ok", command, format, mode, info.PageSize, info.PageCount, info.RetainsHistory}, nil
}

// Only fixed categories reach stderr. Do not print database paths, catalog SQL,
// arbitrary native/provider errors or descriptors. Keep cleanup/publication
// uncertainty ahead of retryable categories. Returned wrapping is also limited
// to known static sentinels; it does not retain a secret-bearing error chain.
func maintenanceError(command string, err error) error {
	for _, candidate := range []struct {
		cause error
		text  string
	}{
		{sqlite.ErrCommitUncertain, "publication state uncertain; do not retry automatically"},
		{sqlite.ErrPoisoned, "storage ownership is poisoned; do not retry automatically"},
		{sqlite.ErrRecoveryRequired, "storage requires recovery; no recovery was attempted"},
		{sqlite.ErrUnsupported, "unsupported source, mode, platform or operation; see docs/cli-encryption.md"},
		{encryption.ErrKeyProvider, "credential provider unavailable or invalid"},
		{encryption.ErrCredential, "credential provider unavailable or invalid"},
		{sqlite.ErrAuthentication, "authentication failed"},
		{sqlite.ErrCorrupt, "source validation failed"},
		{sqlite.ErrBusy, "database is busy; rekey requires all other connections to close"},
		{sqlite.ErrMemoryLimit, "resource limit exceeded"},
		{sqlite.ErrFull, "storage limit exceeded"},
		{context.Canceled, "operation canceled; inspect state before retrying"},
		{context.DeadlineExceeded, "deadline exceeded; inspect state before retrying"},
		{os.ErrExist, "destination already exists; choose a new path"},
	} {
		if errors.Is(err, candidate.cause) {
			return fmt.Errorf("%s: %s: %w", command, candidate.text, candidate.cause)
		}
	}
	return fmt.Errorf("%s failed; output may be incomplete or publication state uncertain; do not retry automatically", command)
}
