# Experimental encrypted snapshot example

On supported Linux, from the repository root with Go 1.27.2:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted
```

The example creates isolated temporary directories and four independent
`crypto/rand` raw-key providers for create, rekey, portable backup, and restore.
It writes and checks a disposable SQL row, authenticates the snapshot, closes the
writer before a read-only reopen with the new key, creates an encrypted archive,
and checks a fresh restored family. It uses only correct credentials and intact
files. It does not run tamper, wrong-key-on-disk, crash, recovery, or fault tests.

No key is printed, passed through argv/SQL/a DSN, or saved to a file. The providers
are ephemeral: they are deliberately unsuitable for data that must survive this
process. The example best-effort clears owned buffers and removes only its own
temporary directory after closing all database handles. Neither operation is
secure erasure. Rekey retains previous encrypted generations decryptable with
their previous credentials. No plaintext export is performed.

Expected output:

```text
Created, wrote, queried, and authenticated an encrypted snapshot.
Rekeyed and reopened with the new key; old ciphertext is retained.
Created a portable encrypted backup and verified a fresh restored family.
Keys stayed in memory; disposable files will now be removed (not securely erased).
```

See [storage API and limits](../../docs/encryption-storage.md),
[primitive design and evidence](../../docs/encryption-primitives.md), and
[security boundaries](../../docs/security.md). This is experimental, snapshot-only
example. A separate [WAL example](../encrypted-wal) demonstrates dirty-page segments.
Neither mode has SEE compatibility, independent security audit,
stable format promise, or production-security/durability qualification.
