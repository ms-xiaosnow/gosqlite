# Experimental encrypted WAL example

On supported Linux, from the repository root with Go 1.27.2:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted-wal
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./examples/encrypted-wal
```

The example creates a private temporary directory and one `crypto/rand` raw-key
provider. It explicitly selects `sqlite.EncryptionWAL`, then creates a table,
inserts and updates a row through three changed commits. It queries the latest
value and authenticates the complete selected base/segment head. It closes with
`KeepWALOnClose: true`, reopens with the same correct key, and checks that the
head is unchanged. An explicit checkpoint writes a fresh full base and selects
zero segments. A final read-only reopen verifies the same data and metadata.

Expected output:

```text
Created, changed, queried, and authenticated an encrypted WAL head (3 segments).
Normally closed and reopened with the same key; the committed segments remain selected.
Checkpointed and reopened read-only: zero selected segments, same data, retained ciphertext history.
The key stayed in memory; disposable files will now be removed (not securely erased).
```

The key is never printed, saved to a file, or passed through argv, SQL or a DSN.
The provider is intentionally unsuitable for data that must survive this process.
All owned handles close before removal of the example's directory on normal
completion. Clearing owned buffers and deleting temporary files are not secure
erasure. Checkpoint itself retains old encrypted objects; it does not reclaim
history. No plaintext export is performed.

A writable encrypted connection holds an exclusive lifetime lease. Read-only
connections may coexist with each other, but cannot coexist with a writer. The
example closes each handle before opening the next one. Shared encrypted
writer/readers remain an unimplemented code requirement.

This uses only correct credentials, intact files, and normal API operations. It
does not run wrong-key-on-disk, disk corruption/tamper, crash, process-exit,
recovery or fault tests. Successful normal reopen does not establish crash
recovery. The independent container is experimental, Linux-only, and has no
SEE compatibility, stable-format promise, independent security audit, or
production-security/durability qualification.

See [storage API, close behavior and limits](../../docs/encryption-storage.md),
[ADR 0012](../../docs/decisions/0012-authenticated-wal-segments.md), and
[security boundaries](../../docs/security.md). The separate
[snapshot example](../encrypted/README.md) demonstrates mode-zero snapshot
creation, rekey and portable backup/restore.
