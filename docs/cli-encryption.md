# CLI encrypted backup and rekey

This is a narrow implementation of goal §12.2 using the existing experimental
native encrypted storage APIs. It does not change the encrypted format, public
`sqlite.Options`/`sqlite.Result` layouts, KDF policy, platform restrictions or the
existing query/inspect/check/migrate syntax. Do not use irreplaceable data.
[Storage and acceptance boundaries](encryption-storage.md) still apply.

## Commands

```text
gosqlite backup <db> --out <new-archive> --key-provider <provider> [--out-key-provider <provider>] [--mode snapshot|wal|shared]
gosqlite rekey <db> --key-provider <new-provider> --current-key-provider <provider> [--mode snapshot|wal|shared]
```

Put the database path first, then separate flag/value pairs. Each flag may appear
once. `--flag=value`, positional extras and automatic mode detection are not
supported. With a path beginning `--`, use an unambiguous absolute path. Both
commands open an existing database with `NoCreate`; they never create a missing
source. Options and provider descriptors are validated without printing their
contents. `gosqlite help` lists both commands.

`--mode snapshot` is the default and selects layout 1. `wal` explicitly selects
layout 2; `shared` selects layout 3. The existing family must match. These are
independent encrypted modes, not native SQLite WAL flags.

### Backup

`--key-provider` supplies the source credential. `--out-key-provider` supplies an
independent archive credential; when omitted, the same acquired source credential
also protects the archive. This is explicit documented credential reuse, not a
conversion to plaintext. The archive always receives fresh database, epoch and
file identities and role-separated encryption keys.

The implementation opens the source read-only and calls
`DB.BackupEncrypted`. It copies one authenticated logical head, including all
selected committed encrypted WAL segments. The native copy operation validates
supported catalog grammar and B-tree/overflow/freelist ownership and holds its
snapshot through validation and output. This does not prove full index-to-table
content equality, external identity or freshness.

Snapshot/exclusive-WAL sources require writers to close before this read-only
connection can acquire its lifetime lease. Shared encrypted mode can copy a
pinned committed head while cooperative readers/writers continue; later commits
are not added to that archive. No plaintext online-backup capability is implied.

The destination must be absent and its parent must exist. The native archive
writer exclusively creates its encrypted output and preserves existing outputs;
it does not replace a prior backup. Partial encrypted output may remain after an
error and is not a usable successful backup. No raw file-family copying, external
SQLite process, plaintext spill, fallback or custom publication protocol is used.
A manifest alone is not a backup. Destination publication, owned descriptors,
locking, synchronization and error cleanup remain the existing native API's work.

**Plaintext backup is unimplemented in this CLI and its public copy API.**
`gosqlite backup plain.db --out new.db` returns an explicit unsupported error
before opening either path. It does not silently copy a main file or a live
SQLite/WAL family. `DB.BackupEncrypted` requires an encrypted session;
`DB.ExportPlaintext` explicitly exports an encrypted source; `ImportEncrypted`
creates an encrypted family from a supported rollback-mode plaintext snapshot.
None is a generic plaintext-to-plaintext backup API. A future plaintext snapshot
backup needs a public pinned-page/copy/publication implementation and separate
scope/evidence. Native plaintext WAL backup remains a phased core item.

The result is a **portable encrypted archive**, not an openable live database.
Restore it with `sqlite.RestoreEncrypted(ctx, archive, absentDestination,
archiveOptions, targetOptions)`. That API authenticates the complete archive,
validates supported logical page ownership and creates a new family with fresh
identities. It leaves the archive unchanged. This slice does not add a restore
subcommand. The runnable example below performs a complete native restore and
checks the rows. Do not pass the archive to `gosqlite query` or `sqlite.Open`.

### Rekey

`--current-key-provider` supplies the current credential; `--key-provider`
supplies the new credential. A writable existing encrypted session calls
`DB.Rekey` with the same mode. All other connections must close for rekey,
including shared-mode idle/read-only connections. The core validates the logical
copy, rewrites the complete selected head and publishes a fresh epoch/root using
its existing verified publication protocol. It preserves the logical database
identity and mode and advances the commit sequence; encrypted WAL modes select a
full new base and zero segments. A post-operation native verification completes
before success is emitted. Native WAL close checkpointing is not introduced.

**Rekey retains history.** Old roots/bases/segments remain decryptable with their
old credentials. It neither revokes old keys' access to history nor erases old
ciphertext, plaintext copies, backups or snapshots. It does not convert plaintext
in place, change encrypted mode or recover an interrupted operation. Keep the
current and replacement credentials available until successful verification.

## Local provider descriptors

| Descriptor | Input |
| --- | --- |
| `env-hex:NAME` | Exactly 64 ASCII hexadecimal characters encoding a 32-byte raw key |
| `env-password:NAME` | Exactly the environment value's 1–4,096 password bytes |
| `stdin-hex` | 64 hex characters through a pipe or redirected regular file, optionally followed by one LF or CRLF, then EOF |
| `stdin-password` | Exactly 1–4,096 password bytes through a pipe or redirected regular file, then EOF |

The credential kind is explicit; a 64-character password is still a password.
Hex is transport encoding, not a password KDF. Password mode uses the existing
Argon2id production default (64 MiB, three passes, four lanes); opening uses
bounded stored KDF parameters. This CLI does not expose quota or KDF overrides.

`NAME` must match `[A-Za-z_][A-Za-z0-9_]*`, at most 128 bytes. Only that exact
variable is read. No database path, unauthenticated ID or SQL selects another
secret. There is no shell evaluation, provider command execution, URL lookup,
raw-secret flag or implicit default variable. Two explicit stdin providers in one
command are rejected before opening a database or reading credentials. Backup
with only a stdin source provider reuses its single cached input for the target.
Use an env source with stdin target, or vice versa, when two distinct credentials
are needed. Empty input and malformed/oversized encodings are rejected.

Password bytes are never trimmed. A trailing newline changes the password.
Use a trusted noninteractive producer that emits exactly the intended bytes and
then closes its pipe. Terminals, character devices, directories and other input
kinds are refused; the CLI never prompts or changes terminal echo. The bounded
read uses one fixed input buffer (the accepted byte limit plus one sentinel
byte), without growing/reallocating credential-bearing input buffers. Reading a
pipe can still wait for its producer to close; context is checked before/after
reading but does not interrupt a blocked OS read. No detached reader or KDF
worker is launched to evade the native resource budget.

Acquisition is lazy, behind the native API's platform/KDF admission. A provider
caches one bounded credential per command and returns separately owned borrows.
Release and command cleanup clear controlled byte buffers best-effort, including
partially decoded hex. Provider/material formatting is always redacted.

**Do not put secret values in process arguments, shell history, filenames,
DSNs, SQL or logs.** Pass only descriptors/variable names in arguments. Arrange
for a trusted launcher to supply the environment or pipe. Avoid shell tracing,
printing environment dumps, untrusted secret-producing programs and putting
credential files in source control. Environment transport is not a vault:
privileged/local process inspection, inherited environments, launcher logs,
immutable Go strings, runtime/crypto copies and memory dumps can expose secrets.
Stdin reduces environment exposure but does not protect a compromised host.
This interface does not claim absolute memory erasure or invent persistent KMS
access. For application-specific key management use the native `KeyProvider`.

## Success, errors and operational scope

Both commands emit one JSON result only after the operation and connection
cleanup succeed. It contains fixed status/operation/format/mode fields, page
geometry and `retains_history`; no keys, descriptors, paths, IDs, SQL or data
rows are logged. Backup reports `gosqlite-encrypted-archive` with snapshot mode;
rekey reports `gosqlite-encrypted-family` and the preserved source mode.

Parsing/provider errors never echo submitted arguments. Native errors are mapped
to fixed categories; arbitrary provider/native error messages and their original
chains are not retained in the CLI error. Publication uncertainty/poison/recovery
categories take precedence over BUSY. Unknown failures are conservative. A later
verification, close, or stdout error can follow a successful publication, so a
nonzero exit does not prove the database was unchanged. Do not automatically
retry, replace or remove an output after an error. No failed-operation cleanup,
recovery, old-root fallback or repair workflow is promised by this CLI.

Linux ordinary-path runtime evidence is limited to the checks recorded for this
slice. Darwin support is source/compile only and retains ASCII target/inventory
restrictions; Windows/amd64 is source/compile only, restricted to its supported
local NTFS/OS profile and has no directory-sync barrier. **Windows shared-mode
rekey remains unsupported.** Other unsupported targets still fail closed. See
[native Darwin scope](native-encrypted-darwin.md) and
[native Windows scope](native-encrypted-windows.md); the later Darwin document
supersedes the earlier Windows document's historical Darwin-gap statement.

No wrong-key/tampered/corrupt/missing-selected-file disk scenario, native fault,
namespace mutation/race, crash/process termination/hot recovery or interrupted
publication/rekey acceptance is part of this work. No such tests were run here
or moved elsewhere. Cross compilation is not native runtime qualification.
Independent security/cryptographic/legal review and full release acceptance
remain outstanding.

## Runnable, disposable end-to-end example

From the repository root on Linux with Go 1.27.2 and existing offline module
cache:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -o /tmp/gosqlite-cli ./cmd/gosqlite
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted-cli /tmp/gosqlite-cli
```

The [example source](../examples/encrypted-cli/main.go) creates disposable random
keys and an encrypted database through the public API. It invokes the actual
binary for env-source/stdin-target backup and env-source/env-target rekey, then
reopens with the new key, restores the archive through the native API and checks
both sets of rows. Child arguments contain only paths and descriptors. It neither
prints credentials nor writes credential files. Children finish normally; its
private temporary database directory is removed on normal completion, which is
not secure erase. The example's keys are intentionally not recoverable later.

For an existing encrypted snapshot whose keys were supplied by your trusted
launcher, the command shapes are:

```sh
gosqlite backup /private/source.gse --out /private/new.backup --key-provider env-hex:GOSQLITE_CURRENT --out-key-provider env-hex:GOSQLITE_ARCHIVE
gosqlite rekey /private/source.gse --key-provider env-hex:GOSQLITE_NEW --current-key-provider env-hex:GOSQLITE_CURRENT
```

Use `--mode wal` or `--mode shared` only for families created in those modes.
There is no plaintext success example because that route is unsupported.
