# CLI encrypted backup/rekey slice: frozen v1 handoff

## Result

Usable native-backed `gosqlite backup` and `gosqlite rekey` commands are implemented.
They support explicit environment-variable names and controlled noninteractive
stdin, with explicit raw-hex versus Argon2id password kinds. The existing public
API, encryption format, target publication/ownership paths and platform policy
are unchanged. Plaintext backup is explicitly unsupported; no generic successful
backup or raw main/WAL file copy is claimed.

- Frozen source: `cli-backup-rekey-freeze-v1`
- Requested immutable baseline: `plain-final-verify`, Git tree
  `7d3184f87165e11e12560884ec6e4f92c9bfd039`
- Final full source tree: `acba5181381f6ee4beb56f1b33ee1a84b6b7d5e1`
- Reconstruction patch: `cli-backup-rekey-evidence/source.patch`
- Patch SHA-256: `09d3bd11dc91273e9fa5a166b28f71d32cd246b63426fd971a57f6d2a7ae83bb`
- No commit was created. No upload, remote write or dependency download occurred.

## Exact ownership

Seven owned paths; 1,252 baseline files and 1,258 final files. Six additions and
one narrow help/dispatcher modification. Every unowned path/content/mode remains
identical, including goal, public Options/Result, core encryption/backup APIs,
central progress/compatibility/matrix documents and test-scope guard.

1. `cmd/gosqlite/main.go`: backup/rekey dispatcher and help only
2. `cmd/gosqlite/maintenance.go`: CLI parser, native execution and redacted results/errors
3. `cmd/gosqlite/key_provider.go`: bounded env/stdin key-provider adapter
4. `cmd/gosqlite/maintenance_test.go`: parser/error memory tests and ordinary correct-key round trips
5. `cmd/gosqlite/key_provider_test.go`: memory-only provider tests
6. `docs/cli-encryption.md`: exact CLI/provider/storage/platform/security scope and examples
7. `examples/encrypted-cli/main.go`: real subprocess end-to-end disposable example

`owned-hashes.json` records baseline/final hashes, modes and sizes, with null
baseline entries for new files. `baseline-inventory.json` and
`final-inventory.json` are full path inventories; `final-all-sha256.txt` is the
portable SHA check list. `source.patch` passed whitespace checks, clean apply
checks and full readback in independent `cli-backup-rekey-patch-check`.

## Actual semantics

Backup opens an existing encrypted source read-only/no-create and calls
`DB.BackupEncrypted`. Native copy validation and operation-pinned source views
are retained, including selected committed encrypted-WAL pages. The destination
is an absent, portable encrypted archive with fresh identity and no history;
it is not a live DB and must be restored with native `sqlite.RestoreEncrypted`.
No restore CLI was added. Omitted `--out-key-provider` intentionally reuses the
source's provider; explicit target credentials are supported.

Snapshot/exclusive-WAL backup requires writers closed under current native
lifetime leases. Shared encrypted backup pins one committed head through the
existing API. Generic plaintext-to-plaintext backup is not implemented by that
API; no-provider plaintext syntax returns unsupported before any source or target
open. Native plaintext online backup remains a separate core work item.

Rekey opens existing encrypted storage writable/no-create, supplies the current
provider to Open and the target provider to `DB.Rekey`, and keeps the same mode.
It retains logical identity, advances the sequence and preserves old ciphertext.
A native VerifyEncrypted runs afterward. Shared rekey requires all other handles
to close. No implicit mode upgrade, plaintext conversion, recovery or cleanup
fallback is introduced. KeepWALOnClose avoids incidental close checkpointing.

Both commands report success JSON only after close completes. Rekey/backup may
have published even if later verification/close/output fails; errors do not
promise non-mutation or safe automatic retry.

## Source/error/security review

This is implementation-owner source review, not an independent cryptographic
review. A separate reviewer has received the immutable freeze and evidence.

- maintenance.go:29–97 parses fixed flag/value pairs without formatting submitted
  data. Missing providers/invalid descriptors/duplicate or unknown flags reject
  with constant errors; no `flag.FlagSet` diagnostic echoes raw values.
- maintenance.go:100–136 binds input solely to LookupEnv or stdin and rejects
  non-regular/non-pipe stdin. Two explicit stdin descriptors reject before open.
- key_provider.go:39–70 recognizes only four local descriptor forms and ASCII
  variable names. There is no shell/URL/provider command or dynamic routing from
  untrusted KeyRequest path/IDs. Provider acquisition remains lazy behind the
  core's existing KDF/platform preflight.
- key_provider.go:73–158 caches one bounded acquisition, explicitly distinguishes
  credential kinds, uses bounded stdin reads, exact password bytes and strict
  hex length/decoding. Malformed hex clears partial output; failed acquisition
  is not silently retried.
- key_provider.go:35–36,174–175 redacts provider/material formatting. All owned
  borrows/cache/input byte buffers clear best-effort. Environment/Go immutable
  strings and runtime/crypto copies remain a documented exposure; no absolute
  erasure or environment secrecy claim is made.
- maintenance.go:159–193 delegates only to OpenContext, BackupEncrypted, Rekey,
  VerifyEncrypted and Close. No raw live-family copy, alternate native open,
  weaker lock/rename/overwrite or credential policy is implemented in the CLI.
- maintenance.go:200–224 maps only static sentinels/category text and does not
  retain original arbitrary error chains; uncertainty/poison/recovery precede
  BUSY. Only static/non-secret geometry/status goes to stdout. No native failure
  was injected to exercise exceptional disk paths.
- stdin reads can wait for EOF despite cancellation; the documented producer
  must complete. The CLI does not change terminal modes or create detached
  workers, and inherits the core's noninterruptible bounded Argon2 behavior.
- The CLI uses default resource/KDF bounds only. No quota override was added.

## Runnable examples and evidence

Build and run the documented Linux disposable workflow:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -o /tmp/gosqlite-cli ./cmd/gosqlite
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted-cli /tmp/gosqlite-cli
```

The example generates keys in memory, creates data through native Go API, runs
the actual binary with env-source/stdin-target backup and env-source/env-target
rekey, restores through native API, reads both correct-key results and normally
cleans its own disposable directory. No credential values are printed or stored
in argv/files. Native restore proves the archive is usable, not merely opaque.

`test-audit.txt` records the pre-run behavior audit. `checks.sh`/`checks.txt`/
`checks.exit` record the work-tree execution. `frozen-checks.sh`/
`frozen-checks.txt`/`frozen-checks.exit` record the exact immutable source rerun.
Each uses sibling Go 1.27.2 with GOTOOLCHAIN=local/GOPROXY=off/GOSUMDB=off,
existing offline caches, CGO0 except the selected CGO1 race run.

Selected checks include the source-reference guard before normal/race tests;
new provider/parser/redaction tests; correct-key snapshot/WAL/shared portable
backup, correct-key rekey, restore/data readback; generated raw-to-password
rotation; one-stdin default archive credential reuse; existing version/query/
inspect/native-WAL/shared/grams CLI contracts; vet of changed packages; full
pure-Go builds; actual CLI subprocess example; race; cross-builds for
linux/arm64, darwin/arm64, darwin/amd64, windows/amd64 and windows/arm64; module
verification. Broad old-baseline or manual-tag suites were not run.

## Remaining boundaries

- Plaintext-to-plaintext backup API/CLI and plaintext online/WAL backup are absent.
- Encrypted query/inspect/check CLI flags and a restore CLI are outside this slice;
  native APIs support the demonstrated setup/restore/readback.
- Darwin runtime remains unexecuted; ASCII target/inventory restrictions remain.
- Windows runtime remains unexecuted; supported NTFS/OS/native-backend restrictions
  and missing directory barrier remain; Windows shared-mode rekey is unsupported.
- Other unsupported targets remain fail-closed. Cross-build success is not runtime
  or filesystem acceptance.
- Wrong-key/tamper/corruption/missing-selected-file, namespace mutation/race,
  native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permission faults,
  crash/kill/abrupt process exit/hot recovery and interrupted publication/rekey
  were not executed or relocated. Native subprocesses completed normally.
- No security/durability/release acceptance, historical credential revocation,
  secure deletion, hostile-namespace safety or whole-family rollback prevention
  is asserted. Existing explicit experimental-storage warnings remain.
