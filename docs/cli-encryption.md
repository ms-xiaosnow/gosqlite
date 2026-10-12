# CLI encrypted backup, rekey and inspect

## 当前状态入口（2026-10-12）

本文件旧段落保留各自写作时点的源码、FAIL/HOLD/UNKNOWN 与 NOT_RUN；其中“当前”“缺失”“待合入”或 PENDING 不能覆盖[当前源码/证据矩阵](current-status-20261012.md)。已保存 main8bc 的发布、精确恢复与 CI 已闭合；本次 LEFT/inspect/Darwin/Connector/automatic UNIQUE 组合仅 SOURCE_ONLY；前序 Rows pin 已修正，新增 sqlite.go 的 WAL-tail 完整源 pin 经独立完整 ownership 复审在本后继窄修，automatic UNIQUE 原 oracle FAIL 及 d79 HOLD 保留，已接入独立审阅的空结果表头修复，组合运行仍全部 NOT_RUN。输入切片的有限成功不转授本组合，原生手工/物理门不变。

This is a narrow implementation of goal §12.2 using the existing experimental
native encrypted storage APIs. It does not change the encrypted format, public
`sqlite.Options`/`sqlite.Result` layouts, KDF policy, platform restrictions or the
existing plaintext inspect JSON and leading flags, or query/check/migrate syntax. Do not use irreplaceable data.
[Storage and acceptance boundaries](encryption-storage.md) still apply.

## Commands

```text
gosqlite backup <db> --out <new-archive> --key-provider <provider> [--out-key-provider <provider>] [--mode snapshot|wal|shared]
gosqlite rekey <db> --key-provider <new-provider> --current-key-provider <provider> [--mode snapshot|wal|shared]
gosqlite inspect [--extensions] <db> --key-provider <provider> [--mode snapshot|wal|shared]
```

For backup/rekey, put the database path first, then separate flag/value pairs. Each flag may appear
once. `--flag=value`, positional extras and automatic mode detection are not
supported. With a path beginning `--`, use an unambiguous absolute path. Backup and rekey
open an existing database with `NoCreate`; they never create a missing
source. Options and provider descriptors are validated without printing their
contents. `gosqlite help` lists these commands.

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

**Plaintext backup has a restricted, experimental source implementation.**
`gosqlite backup plain.db --out new.db` without encryption flags selects
`DB.Backup`; the public API also permits a pinned, explicitly read-only
`Tx.Backup`. Only checked ordinary rollback snapshots are in scope. This path
requires Linux ext-family admission, actual statx unique mount-ID fields,
Openat2 resolution protections, and no-replace publication in an existing
trusted/private directory with bounded ASCII main names. A private candidate
holds EXCLUSIVE before copying and stays protected through file Sync,
publication, directory Sync and directory retirement. These conditions are not
proof of general filesystem, concurrency or crash durability qualification.
WAL sources (including read-only WAL), non-Linux and other filesystems, including
tmpfs/overlay in the current environment, remain Unsupported. Native acceptance
is NOT_RUN; do not use irreplaceable data or retry/replace/remove artifacts after
an uncertain error. See the full [Backup contract](plaintext-backup.md) and
[separately gated manual procedure](testing/plaintext-backup-manual.md).
`DB.BackupEncrypted`, `DB.ExportPlaintext` and `ImportEncrypted` retain their
separate encrypted-source/archive/import contracts; no plaintext WAL-family
copy or fallback is introduced.

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

### Inspect: source implementation, runtime NOT_RUN

The new encrypted inspect route implements the goal §12.2 key/profile plumbing
using the existing public `OpenContext` and `DB.Inspect` APIs. This candidate's
parser/compiler/tests/native/race/build/full gates have **NOT_RUN**; earlier
backup/rekey or base-tree results do not qualify this new route.

`gosqlite inspect [--extensions] <db> --key-provider <provider>` explicitly
selects encrypted snapshot mode. Append `--mode wal` or `--mode shared` only for
an existing family in that profile. Provider/mode pairs follow the path; each
may appear once, in either order. No automatic profile detection, conversion,
recovery, credential-file input, secret-value argument or KDF override is added.
All four local descriptors below are supported by the unchanged provider.

Plaintext `inspect [--wal-readonly] [--extensions] <db>` retains its leading-flag
ordering flexibility and exact default/extension JSON field order and labels.
`--extensions` must still lead the path and remains opt-in. Writable native WAL
flags are rejected; native `--wal-readonly` cannot be mixed with encryption.
`--mode` without `--key-provider`, extra positionals, duplicates and malformed
providers are rejected without printing submitted values.

The encrypted route opens read-only/no-create, retains encrypted WAL on close
for wal/shared, and calls `DB.Inspect` once. The caller context reaches both
public OpenContext and Inspect. Recovery/publication options remain unset.
Its allowlisted JSON fields are `format: "gosqlite-encrypted-family"`, selected
`mode`, `metadata_scope: "logical SQLite metadata inspection only"`, `header`
and `schema`; `--extensions` additionally emits `extension_indexes` (including
`[]` when empty). Header/schema describe decrypted logical SQLite metadata, not
an ordinary physical SQLite 3 file. No provider, credential, descriptor, env
name, path, physical encryption header, identity or debugging state is emitted.
No `VerifyEncrypted`, extra verification snapshot or index scan is added.

Schema SQL is deliberate inspect output. `--extensions` also exposes persisted
application-chosen method/configuration text. Such metadata can itself contain
sensitive application content; the credential-plumbing non-disclosure promise
does not make arbitrary stored metadata secret-free. This is not an integrity,
index-usability, anti-rollback or production-security verdict.

Inspect joins its operation error with one checked DB Close attempt and emits
JSON only if both succeed. A writer failure remains failure and can leave a
partial JSON prefix. Encrypted errors, including writer errors, are reduced to
fixed categories with uncertainty/poison/recovery preceding retryable causes;
no arbitrary error chain is retained. A failed Close does **not** prove all
native owners retired. The CLI does not retry Close, perform recovery, or remove
uncertain artifacts. Native cleanup tracking remains the existing core's work.

The input runner owns and clears its provider only after the synchronous DB
operation/Close attempt and output handling; the executor merely borrows it.
The existing credential contract copies/releases borrowed material during key
derivation. Logical storage close clears crypto state/options before separately
managed native cleanup; that cleanup never reacquires credentials. Clearing the
CLI provider therefore does not assume native retirement, and makes no absolute
Go-memory erasure claim. Caller-owned providers passed to the executor are not
closed by it.

New test source separates descriptor-free argument/lifecycle/error models from
healthy correct-key native snapshot/wal/shared and default-password fixtures.
Each acquired fixture DB gets one checked Close attempt; successful and failed
directories remain, but disposable credentials are cleared and retained ciphertext is not
promised decryptable. No native failure/tamper/wrong-key/recovery/namespace or
other-platform qualification is included. All new validation remains NOT_RUN.

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

Backup and rekey emit one JSON result only after the operation and connection
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
No plaintext success example is provided here; the separately gated
[manual procedure](testing/plaintext-backup-manual.md) has its own qualification
requirements and remains NOT_RUN.
