# Encrypted shared readers and one writer

Run from the repository root on Linux with Go 1.27.2:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted-shared
```

This disposable example explicitly chooses `EncryptionWALShared` (layout 3).
Independent connections hold old read snapshots while another connection commits.
A third reader can start while an IMMEDIATE writer has unpublished changes.
Stale read-to-write promotion returns `ErrBusySnapshot`; ending that transaction
and starting another observes the latest committed head. Checkpoint works while
another reader holds an old immutable selection.

The provider generates a process-local random key, never prints or persists it,
and cannot reopen the database after the process exits. Use real trusted key
management for persistent applications. Files are removed only after normal
close; cleanup is not secure deletion. This is ordinary correct-key cooperative
use, not crash, hostile-namespace, cryptographic-security or production acceptance.
See [the storage contract](../../docs/encryption-storage.md) and
[ADR 0016](../../docs/decisions/0016-shared-encrypted-coordination.md).
