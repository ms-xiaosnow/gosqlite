package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Descriptor-free DB owners: no file handles, native fault injection, disk
// authentication failure, recovery or namespace behavior is modeled here.
type inspectMemoryDB struct {
	inspect func(context.Context, sqlite.InspectOptions) (sqlite.InspectResult, error)
	close   func() error
}

func (d *inspectMemoryDB) Inspect(ctx context.Context, opts sqlite.InspectOptions) (sqlite.InspectResult, error) {
	return d.inspect(ctx, opts)
}
func (d *inspectMemoryDB) Close() error { return d.close() }

type inspectMemoryWriter func([]byte) (int, error)

func (w inspectMemoryWriter) Write(p []byte) (int, error) { return w(p) }

func TestCLIInspectMemoryArguments(t *testing.T) {
	const marker = "private-input-must-not-echo"
	for i, args := range [][]string{
		nil, {}, {""}, {"db", "extra"}, {"db", "extra", "extra"},
		{"--wal-exclusive", "db"}, {"--wal-shared", "db"},
		{"--wal-readonly", "--wal-readonly", "db"}, {"--wal-readonly", "--wal-shared", "db"},
		{"--extensions", "--extensions", "db"}, {"--" + marker, "db"},
		{"db", "--extensions"}, {"db", "--wal-readonly"}, {"db", "--key-provider"},
		{"db", "--key-provider", ""}, {"db", "--key-provider=" + marker},
		{"db", "--key-provider", marker}, {"db", "--key-provider", "env-hex:" + marker},
		{"db", "--key-provider", "env-hex:"}, {"db", "--key-provider", "env-password:1KEY"},
		{"db", "--key-provider", "stdin-hex", "--key-provider", "stdin-hex"},
		{"db", "--key-provider", "stdin-hex", "--mode", "snapshot", "--mode", "wal"},
		{"db", "--mode", "snapshot"}, {"db", "--mode", "wal"}, {"db", "--mode", "shared"},
		{"db", "--key-provider", "stdin-hex", "--mode", marker},
		{"db", "--key-provider", "stdin-hex", "--mode"},
		{"db", "--key-provider", "stdin-hex", "--mode", ""},
		{"db", "--password", marker}, {"db", "--raw-key", marker},
		{"--wal-readonly", "db", "--key-provider", "stdin-hex"},
		{"--key-provider", "stdin-hex", "db"},
	} {
		t.Run(fmt.Sprintf("rejected_%02d", i), func(t *testing.T) {
			input := credentialInput{
				lookupEnv: func(string) (string, bool) { t.Fatal("argument check read credential"); return "", false },
				stdin: func() (io.Reader, error) { t.Fatal("argument check read stdin"); return nil, nil },
			}
			open := func(context.Context, string, sqlite.Options) (inspectDB, error) { t.Fatal("invalid arguments reached opener"); return nil, nil }
			var out bytes.Buffer
			err := runInspectWithOpener(context.Background(), args, &out, input, open)
			if err == nil || out.Len() != 0 || strings.Contains(err.Error(), marker) {
				t.Fatal("invalid arguments accepted, echoed or emitted output")
			}
		})
	}
}

func TestCLIInspectMemoryOptions(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "caller")
	for _, mode := range []struct { name string; value sqlite.EncryptionMode }{
		{"default", sqlite.EncryptionSnapshot}, {"snapshot", sqlite.EncryptionSnapshot}, {"wal", sqlite.EncryptionWAL}, {"shared", sqlite.EncryptionWALShared},
	} {
		for _, descriptor := range []string{"env-hex:KEY", "env-password:PASSWORD", "stdin-hex", "stdin-password"} {
			t.Run(mode.name+"/"+descriptor, func(t *testing.T) {
				args := []string{"--extensions", "memory-only", "--key-provider", descriptor}
				if mode.name != "default" { args = append(args, "--mode", mode.name) }
				var provider *cliKeyProvider
				opened, inspected, closed := 0, 0, 0
				open := func(got context.Context, path string, opts sqlite.Options) (inspectDB, error) {
					opened++
					if got != ctx || path != "memory-only" || opts.Encryption == nil { t.Fatal("opener lost context, path or encryption") }
					var ok bool
					provider, ok = opts.Encryption.KeyProvider.(*cliKeyProvider)
					if !ok || provider.closed || provider.attempted { t.Fatal("provider was not lazy and owned") }
					kind := encryption.RawKey256
					if strings.Contains(descriptor, "password") { kind = encryption.PasswordArgon2id }
					want := sqlite.Options{ReadOnly: true, NoCreate: true, KeepWALOnClose: mode.value != sqlite.EncryptionSnapshot,
						Encryption: &sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: kind, Mode: mode.value}}
					if !reflect.DeepEqual(opts, want) { t.Fatal("encrypted options differ from explicit read-only profile; unrelated options must stay zero") }
					return &inspectMemoryDB{
						inspect: func(got context.Context, opts sqlite.InspectOptions) (sqlite.InspectResult, error) {
							inspected++
							if got != ctx || !opts.IncludeExtensions { t.Fatal("Inspect lost caller context or extensions") }
							return sqlite.InspectResult{ExtensionIndexes: []sqlite.ExtensionIndexMetadata{}}, nil
						},
						close: func() error { closed++; if provider.closed { t.Fatal("provider cleared before DB close") }; return nil },
					}, nil
				}
				input := credentialInput{lookupEnv: func(string) (string, bool) { t.Fatal("lazy model read input"); return "", false }, stdin: func() (io.Reader, error) { t.Fatal("lazy model read stdin"); return nil, nil }}
				var out bytes.Buffer
				if err := runInspectWithOpener(ctx, args, &out, input, open); err != nil { t.Fatal(err) }
				if opened != 1 || inspected != 1 || closed != 1 || provider == nil || !provider.closed { t.Fatal("ownership counts or provider cleanup wrong") }
			})
		}
	}
	for _, args := range [][]string{
		{"memory-only"}, {"--extensions", "memory-only"}, {"--wal-readonly", "memory-only"},
		{"--extensions", "--wal-readonly", "memory-only"}, {"--wal-readonly", "--extensions", "memory-only"},
	} {
		o, err := parseInspect(args)
		if err != nil { t.Fatal(err) }
		open := func(got context.Context, path string, opts sqlite.Options) (inspectDB, error) {
			want := sqlite.Options{ReadOnly: true, NoCreate: true, WALShared: o.walReadOnly}
			if got != ctx || path != "memory-only" || !reflect.DeepEqual(opts, want) { t.Fatal("plaintext default/options changed") }
			return &inspectMemoryDB{inspect: func(got context.Context, opts sqlite.InspectOptions) (sqlite.InspectResult, error) {
				if got != ctx || opts.IncludeExtensions != o.extensions { t.Fatal("plaintext inspection options changed") }
				return sqlite.InspectResult{}, nil
			}, close: func() error { return nil }}, nil
		}
		if _, err := executeInspect(ctx, o, nil, open); err != nil { t.Fatal(err) }
	}
}

func TestCLIInspectMemoryCanceled(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stop()
	for _, tc := range []struct { name string; ctx context.Context; want error }{
		{"canceled", canceled, context.Canceled}, {"expired", expired, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := credentialInput{lookupEnv: func(string) (string, bool) { t.Fatal("canceled input read"); return "", false }, stdin: func() (io.Reader, error) { t.Fatal("canceled stdin read"); return nil, nil }}
			open := func(context.Context, string, sqlite.Options) (inspectDB, error) { t.Fatal("canceled opener called"); return nil, nil }
			for _, descriptor := range []string{"env-hex:KEY", "stdin-password"} {
				var out bytes.Buffer
				err := runInspectWithOpener(tc.ctx, []string{"memory-only", "--key-provider", descriptor}, &out, input, open)
				if !errors.Is(err, tc.want) || out.Len() != 0 { t.Fatal("canceled command lost error or emitted output") }
			}
		})
	}
}

func TestCLIInspectMemoryLifecycle(t *testing.T) {
	inspectFailure, closeFailure := errors.New("model inspect failure"), errors.New("model close failure")
	for _, tc := range []struct { name string; inspectErr, closeErr error }{
		{"success", nil, nil}, {"inspect", inspectFailure, nil}, {"close", nil, closeFailure}, {"both", inspectFailure, closeFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := newCLIKeyProvider("stdin-hex", credentialInput{})
			if err != nil { t.Fatal(err) }
			defer provider.close()
			inspected, closed := 0, 0
			open := func(context.Context, string, sqlite.Options) (inspectDB, error) {
				return &inspectMemoryDB{
					inspect: func(context.Context, sqlite.InspectOptions) (sqlite.InspectResult, error) { inspected++; return sqlite.InspectResult{}, tc.inspectErr },
					close: func() error { closed++; if provider.closed { t.Fatal("caller provider retired early") }; return tc.closeErr },
				}, nil
			}
			result, err := executeInspect(context.Background(), inspectCommandOptions{}, provider, open)
			if inspected != 1 || closed != 1 || provider.closed || provider.attempted { t.Fatal("executor did not borrow provider and attempt its DB Close once") }
			if tc.inspectErr != nil && !errors.Is(err, tc.inspectErr) || tc.closeErr != nil && !errors.Is(err, tc.closeErr) { t.Fatal("joined error lost a cause") }
			if (tc.inspectErr != nil || tc.closeErr != nil) && (err == nil || result != nil) { t.Fatal("failure returned a complete-looking result") }
			if tc.inspectErr == nil && tc.closeErr == nil && (err != nil || result == nil) { t.Fatal("healthy model result missing") }
		})
	}
}

func TestCLIInspectMemoryOutputOwnership(t *testing.T) {
	for _, stage := range []string{"success", "open", "inspect", "close", "writer"} {
		t.Run(stage, func(t *testing.T) {
			const marker = "model-private-error-and-credential"
			var provider *cliKeyProvider
			var owned []byte
			var events []string
			input := credentialInput{stdin: func() (io.Reader, error) { return strings.NewReader(marker), nil }}
			open := func(ctx context.Context, _ string, opts sqlite.Options) (inspectDB, error) {
				events = append(events, "open")
				provider = opts.Encryption.KeyProvider.(*cliKeyProvider)
				material, err := provider.Acquire(ctx, encryption.KeyRequest{Kind: encryption.PasswordArgon2id})
				if err != nil { t.Fatal(err) }
				material.Release()
				owned = provider.key
				if stage == "open" { return nil, errors.New(marker) }
				return &inspectMemoryDB{
					inspect: func(context.Context, sqlite.InspectOptions) (sqlite.InspectResult, error) {
						events = append(events, "inspect")
						if stage == "inspect" { return sqlite.InspectResult{}, errors.New(marker) }
						return sqlite.InspectResult{}, nil
					},
					close: func() error {
						events = append(events, "close")
						if provider.closed || !bytes.Equal(owned, []byte(marker)) { t.Fatal("provider cleared during DB ownership") }
						if stage == "close" { return errors.New(marker) }
						return nil
					},
				}, nil
			}
			var out bytes.Buffer
			writer := inspectMemoryWriter(func(p []byte) (int, error) {
				events = append(events, "write")
				if !reflect.DeepEqual(events, []string{"open", "inspect", "close", "write"}) || provider.closed { t.Fatal("output preceded checked Close or provider cleanup order changed") }
				if stage == "writer" { return 0, errors.New(marker) }
				return out.Write(p)
			})
			err := runInspectWithOpener(context.Background(), []string{"memory-only", "--key-provider", "stdin-password"}, writer, input, open)
			want := []string{"open", "inspect", "close"}
			if stage == "open" { want = []string{"open"} }
			if stage == "success" || stage == "writer" { want = append(want, "write") }
			if !reflect.DeepEqual(events, want) || provider == nil || !provider.closed || !bytes.Equal(owned, make([]byte, len(owned))) { t.Fatal("incorrect Close attempt or provider clearing") }
			if stage == "success" {
				if err != nil || out.Len() == 0 { t.Fatal("healthy model did not emit") }
			} else if err == nil || out.Len() != 0 || strings.Contains(err.Error(), marker) { t.Fatal("failure emitted success or disclosed arbitrary error") }
		})
	}
	// The legacy plaintext output-writer error remains the original cause.
	want := errors.New("plaintext in-memory writer failure")
	open := func(context.Context, string, sqlite.Options) (inspectDB, error) {
		return &inspectMemoryDB{inspect: func(context.Context, sqlite.InspectOptions) (sqlite.InspectResult, error) { return sqlite.InspectResult{}, nil }, close: func() error { return nil }}, nil
	}
	err := runInspectWithOpener(context.Background(), []string{"memory-only"}, inspectMemoryWriter(func([]byte) (int, error) { return 0, want }), credentialInput{}, open)
	if !errors.Is(err, want) { t.Fatal("plaintext output error cause changed") }
}

func TestCLIInspectMemoryErrorRedaction(t *testing.T) {
	const marker = "private-error-must-not-escape"
	private := errors.New(marker)
	for _, cause := range []error{sqlite.ErrCommitUncertain, sqlite.ErrPoisoned, sqlite.ErrRecoveryRequired, sqlite.ErrUnsupported, encryption.ErrKeyProvider, encryption.ErrCredential, sqlite.ErrAuthentication, sqlite.ErrCorrupt, sqlite.ErrBusy, sqlite.ErrMemoryLimit, sqlite.ErrFull, context.Canceled, context.DeadlineExceeded} {
		err := inspectError(errors.Join(private, cause))
		if strings.Contains(fmt.Sprintf("%v %s %+v %#v", err, err, err, err), marker) || errors.Is(err, private) || !errors.Is(err, cause) { t.Fatal("redaction lost safe sentinel or retained arbitrary error") }
	}
	for _, cause := range []error{sqlite.ErrCommitUncertain, sqlite.ErrPoisoned, sqlite.ErrRecoveryRequired} {
		err := inspectError(errors.Join(sqlite.ErrBusy, private, cause))
		if !errors.Is(err, cause) || errors.Is(err, sqlite.ErrBusy) || strings.Contains(err.Error(), marker) { t.Fatal("uncertainty/ownership lost to busy") }
	}
	// Synthetic cleanup markers model precedence only; no real retained handle.
	cleanup := inspectError(errors.Join(sqlite.ErrEncryptedCleanupPending, sqlite.ErrPoisoned, sqlite.ErrBusy, private))
	if !errors.Is(cleanup, sqlite.ErrPoisoned) || errors.Is(cleanup, sqlite.ErrBusy) || strings.Contains(cleanup.Error(), marker) { t.Fatal("cleanup uncertainty lost to busy") }
	err := inspectError(private)
	if err.Error() != "inspect failed; metadata output may be incomplete" || errors.Is(err, private) { t.Fatal("unknown error did not use fixed safe text") }
}

func TestCLIInspectMemoryOutputAllowlist(t *testing.T) {
	const metadata = "application-chosen metadata is deliberate output"
	inspection := sqlite.InspectResult{Header: sqlite.FileInfo{PageSize: 4096}, Schema: []sqlite.SchemaEntry{{SQL: metadata}}, ExtensionIndexes: []sqlite.ExtensionIndexMetadata{}}
	for _, extensions := range []bool{false, true} {
		o := inspectCommandOptions{path: "PRIVATE_PATH", provider: "PRIVATE_DESCRIPTOR", mode: sqlite.EncryptionWALShared, extensions: extensions}
		raw, err := json.Marshal(inspectOutput(o, true, inspection))
		if err != nil { t.Fatal(err) }
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil { t.Fatal(err) }
		want := 5
		if extensions { want++ }
		if len(fields) != want || string(fields["format"]) != `"gosqlite-encrypted-family"` || string(fields["mode"]) != `"shared"` || string(fields["metadata_scope"]) != `"logical SQLite metadata inspection only"` { t.Fatal("encrypted output allowlist changed") }
		if !strings.Contains(string(raw), metadata) || strings.Contains(string(raw), o.path) || strings.Contains(string(raw), o.provider) { t.Fatal("metadata omitted or private plumbing serialized") }
		if extensions && string(fields["extension_indexes"]) != "[]" || !extensions && fields["extension_indexes"] != nil { t.Fatal("extension opt-in changed") }
	}
}
