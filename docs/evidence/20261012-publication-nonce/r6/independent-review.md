# R6 nonce/reopen independent source review

## Result

**CLEAR_SOURCE_ONLY** for the exact four-file overlay identified below, applied to the reviewed `edcc8be3e4c0d0dc46066020dc900a5effcaee30` source base. No blocking issue found in the added nonce-domain assertions, ordinary reopen/rekey/checkpoint relationships, read-only helper, or the selected call closure.

This is source review, not a compile, sourceguard, test, race, native/platform, durability, or cryptographic qualification result. No Go command, gofmt, compiler, parser executable, database, test, native toolchain, or CI was run. No file in `gosqlite` or in the overlay was edited by this reviewer. The only created deliverable is this review report.

## Reviewed identity

- Base repository: `restart-20261012T2033/gosqlite`.
- HEAD: `edcc8be3e4c0d0dc46066020dc900a5effcaee30`.
- HEAD tree: `1149df0b8d733e93597acca07d373d029d4df913`.
- Overlay: `restart-20261012T2033/nonce-reopen-overlay`.
- The actual helper is **228 lines**, rather than the 221-line description in the assignment. Hashes, not that stale line count, identify the reviewed source.
- Overlay inventory is exactly three Go test files plus one Chinese note.

SHA-256:

```text
3b5b1368e89e4973d14555226d092033c045958e034aa81c489bc288818e179b  sqlite/encryption_nonce_review_test.go
3bc91721e37ac002a5569ba72ee3fb37bc3b50150b59f65e0854347eefe1e3cb  sqlite/encryption_wal_test.go
06975a23a57fbb7c8d77c94d737a637d96844f2cfc9fe49d5713823f156c4eb9  sqlite/encryption_review_test.go
3d510c4a991b1b0166fd27794d6efe1b2b6bb82e57032e4b83e25db1832404a7  docs/testing/encryption-nonce-lifecycle-zh.md
```

The overlay hashes were reread unchanged during the review. The two existing test files differ from HEAD by additions only: 26 lines in the WAL test and 9 in the snapshot/rekey test. No original assertion or flow was deleted or replaced. The new helper has no conflicting same-package declaration in the reviewed base.

At the first repository status check the base worktree/index was clean. During this review another task staged changes to `internal/testscope/wal_tail_publication.go`, `internal/vfs/backup_publication_model_test.go`, and `internal/vfs/wal_publication_model_test.go`. These are outside this overlay. The guard change only updates the frozen hash of the WAL publication model test, from `738128...` to `465813...`; it does not change the nonce overlay's restrictions. This report does not certify those parallel model edits or the final integrated tree. The reviewed KDF/record format, lifecycle production sources, and two base nonce lifecycle tests remained unchanged.

## Production mapping: key domain and nonce

The symbolic tuple is appropriate for these same-credential fixtures:

```text
(full Epoch, FileID, FileDescriptor.Role, RecordHeader.Role,
 NoncePrefix[4] || big-endian(sequence)[8])
```

- `encryption/key.go:59-69`: Epoch includes DatabaseID, EpochID, credential Kind, all three Argon2 parameters, KDFSalt, and RootSalt. Storing the whole comparable Epoch includes MemoryKiB and Parallelism as well as Passes, even though the pure-value controls individually vary only Passes.
- `encryption/kdf.go:134-159`: raw mode uses the supplied key input; password mode uses the declared salt and exact Argon2 parameters. Raw mode's unused KDF fields must be zero (`key.go:71-91`).
- `encryption/key.go:203-210`: the master is derived from the credential input, RootSalt, DatabaseID, and EpochID.
- `encryption/key.go:260-299`: per-record-role file keys use the master, FileID as HKDF salt, DatabaseID/EpochID, file role, and record role. Full Epoch equality is required before these ciphers are obtained.
- `encryption/record.go:132-138,333-337`: the nonce encoding matches the helper byte-for-byte. Record-role separation matters because each role uses its own derived AEAD key.
- `encryption/record.go:340-350`: header digest, record header, page routing and frame ordinal are AAD. Geometry, commit sequence, transaction identity, parent identity, plaintext length, and ciphertext are correctly excluded as key-domain discriminators. They cannot hide same-key/same-nonce reuse.

The helper never gets provider material, master/file keys, or decrypted metadata. The tests keep the application credential fixed within each observed history, including same-credential rekey. This is a public symbolic derivation-domain assertion, not a proof that distinct KDF inputs can never collide cryptographically.

The pure-value entry verifies the exact 12 nonce bytes, separations by public domain fields and record role, and invariance under the selected AAD/length changes. Its values need not be valid encoded headers because it does not validate, derive, encrypt, decrypt, or write them.

## Retention and publication relationships

- `captureEncryptedNonces` creates a fresh pair set per physical inventory. It rejects duplicate object identities and duplicate symbolic key-domain/nonce pairs within that inventory.
- Object identity is used only to track retained objects. Each stored object also contains the complete decoded Header and SHA-256 of the complete bytes. This is separate from the narrower pair key, so AAD/ciphertext differences cannot make a repeated pair appear fresh.
- `assertEncryptedNonceAdvance` requires every earlier object to remain with the same header and digest, all earlier pairs to remain, exactly two new object identities, a fresh current root FileID, and a successor commit sequence.
- Consequently a root moved to the `.retained` name is the same encryption event. Re-observation is not counted as another Seal. A new object reusing an old pair is rejected by the later inventory's single pair set, in which both retained and new objects are present.
- The two new objects must be one root and one object of the requested base/WAL role, use the published Epoch, and agree on the complete descriptor after normalizing role and root record count.
- That descriptor relation exactly follows `internal/encstore/publish.go:289-306` and `wal_format.go:254-258`.
- The removed WAL parent/root-hash assumption is not present. `Store.HeadCommitID` returns the logical WAL head for WAL and the root digest only for snapshot (`snapshot.go:146-150`). `nextDescriptor` uses that head (`publish.go:15-30`). The prior root's byte digest lives separately in the encrypted WAL manifest (`publish.go:141-143,271`).
- The existing snapshot-only SHA-256 parent-root assertion is retained and remains correct; it is not generalized to WAL.

## Bounded, read-only observation and geometry

The helper is limited to private, intact, quiescent, small generated fixtures; it is not a hostile-input parser or recovery utility.

- Directory members must number 1..64. Only the exact current root, recognized `.gse-` object suffixes, and the exact lock name are accepted; the lock is skipped before reading it.
- Non-lock members must be regular files with sizes from HeaderSize through 4 MiB. A read must return exactly the stat size.
- It uses the existing `fileview.ReadFile`. On non-Darwin this is `os.ReadFile`; on Darwin it uses read-only VFS access and the descriptor-close registry, then closes the file, preserving existing live-lock ownership behavior. No new bare descriptor ownership or write primitive is introduced.
- `ReadHeader` reads a fixed 512-byte header, validates fixed format/version/suite and Epoch constraints, and validates descriptor geometry before the helper uses its counts (`format.go:88-160,203-242`). Base record count is page count plus two; a root has one record; WAL has at least metadata/data/footer and no more than page count plus two.
- Per-file record count is at most 256, plaintext size at most 65536, and all accepted record pairs together at most 4096. Record decoding allocates at most 24 + 65536 + 16 bytes after validating the fixed header (`format.go:294-323`), which is safe for 32-bit int as well as 64-bit int.
- A successful `ReadRecord` always contains its fixed header before the helper slices it. The helper checks the precise sequence, expected metadata/data/footer order, declared extent, and EOF. Nonzero record counts are guaranteed by header validation; bounded loop increments and the count-minus-one expression cannot overflow or underflow here.
- It deliberately does not invent encrypted WAL page routing or authenticate retained files. Per-record page-size equality and authenticated page/frame routing remain production-reader checks, not new claims of this nonce helper. Original SQL and VerifyEncrypted checks remain present.
- The successor `+1` assertion is safe for the intended handful of real publications: production `nextDescriptor` rejects MaxUint64 before incrementing and the fixture has a fixed small history. This helper is not presented as a general arbitrary-header sequence-wrap validator.
- `ReadDir` and the existing `ReadFile` inspect a test-owned quiescent directory. The stat/read check is not a hostile concurrent-growth allocation guarantee; neither the code nor the scope requires one. No symlink/replacement/fault campaign is introduced.

## Exact selector and call closure

```text
^(TestEncryptionWALPasswordOrdinaryReopenAndCancellation|TestEncryptionReviewFreshIdentityAndRetainedBytes|TestEncryptionPublicNonceDomainIdentity)$
```

All three selected functions are ordinary untagged `sqlite_test` entries. There are no selected subtests, Parallel calls, goroutines, init functions, TestMain, benchmark/fuzz entry, environment controller, or manual tag in the overlay. The existing `TestEncryptionWALNativeOptionPreflight` remains in the WAL source file but is excluded by the anchored selector.

Selected paths:

1. WAL password lifecycle: encryptionOptions -> sqlite.Open -> ordinary SQL via encryptedExec -> Begin/Exec -> already-existing cancelled Commit and normal Rollback -> Close with KeepWALOnClose -> correct-credential read-only opens/query -> normal busy-writer and mode-preflight refusals -> ordinary writable reopen -> second INSERT -> query -> normal Close checkpoint -> correct-credential read-only reopen/VerifyEncrypted -> read-only plaintext-marker/sidecar checks. Added calls observe inventories and compare them; they do not mutate fixture bytes.
2. Snapshot lifecycle: encryptionOptions -> sqlite.Open -> ordinary create/insert/delete/insert -> Begin/Exec/Rollback -> same-credential Rekey -> VerifyEncrypted -> Close -> correct-credential read-only reopen/query. Added calls compare rollback inventory, the successful fresh base/root publication, and post-rekey reopen inventory.
3. Pure-value entry: encryptedPublicNoncePair -> copy plus binary.BigEndian.PutUint64 and struct equality. No database or file operation.

New helper closure:

```text
captureEncryptedNonces
  testing helpers/failures
  filepath.Dir/Base/Join, os.ReadDir, DirEntry.Info
  strings.HasPrefix/HasSuffix
  fileview.ReadFile (read-only; closes owned handle)
  bytes.NewReader
  encryption.ReadHeader -> DecodeHeader -> Header/Epoch/descriptor validation
  Header.Identity
  sha256.Sum256
  encryption.ReadRecord -> DecodeRecordHeader -> bounded record validation
  encryptedPublicNoncePair -> copy + binary.BigEndian.PutUint64
  bytes.Reader.Len
assertEncryptedNonceAdvance
  testing helpers/failures, scalar/struct equality, map iteration
```

Shared helpers are unchanged: `encryptionOptions` generates the one fixture credential and registers credential clearing/release-count cleanup; `encryptedExec` calls DB.Exec; `query` calls DB.Query and closes Rows. No backup/restore, format upgrade, wrong credential, corruption, recovery, physical fault, process launch, ANN, campaign, or manual capability is reached by the selected tests. The existing cancelled-context/busy/mode checks are normal API conditions on intact files, not physical failure injection.

An anchored `-run` does not eliminate the package's imports/init. The new file adds only ordinary standard-library imports and already-used encryption/fileview packages, with no initialization effect. Existing default test imports can include testoracle and the SQL driver; testoracle has no initializer and launches only when explicitly called, and the driver's initializer only registers its database/sql driver. Neither is called to open a database or launch a subprocess by this selector. Existing oracle/manual entries remain excluded under ordinary no-tag selection.

## Ownership and preservation

The only ownership change is `defer db.Close()` immediately after the successful writable reopen in the WAL entry (overlay line 103). Go captures that receiver when the defer is registered, so later reassignment of `db` cannot transfer the close to the final read-only connection. It protects the newly added pre-write and post-write assertions. The existing explicit successful Close remains and surfaces its error; the deferred repeat is safe after retirement (`sqlite.go:427-464`, `encrypted_session.go:343-361`). The final read-only handle retains its own defer.

The initial WAL writer retains its existing defer and explicit close; the snapshot test retains its existing closure-based deferred owner. `query` retains deferred Rows.Close. This patch makes no broader claim about repairing pre-existing assertion-failure cleanup in the intervening multi-reader block, which it does not alter.

All original key-provider cleanup, row assertions, retained-byte assertions, VerifyEncrypted checks, busy/mode checks, and plaintext/sidecar assertions are preserved. The added full two-row query strengthens the ordinary reopen write check.

## Sourceguard and integration conditions

Source inspection found no new restricted syscall/process reference, recovery option, manual tag/import consumer, frozen manual entry, or mutable launch reference. `internal/testscope/main.go` scans ordinary test files and rejects manual tags in GOFLAGS; these files fit that existing ordinary path. None of the three overlay paths is named in the current frozen manual-source maps. No guard whitelist or pin change is needed for the nonce overlay.

This is an expectation from source review, not a sourceguard PASS. Before declaring the R6 code slice closed, bind the final integrated source identity and obtain the exact three-entry ordinary and race terminal results plus sourceguard under the already-authorized execution scope. Keep raw output and fail/skip/zero-match checks. No broader selector, oracle/manual tag, generic framework, additional profile, or physical/native campaign follows from this review.

Relevant unchanged source hashes:

```text
bcc56e630a263954a2f01dc8e22a22fd6030d6eed5a58b75411d3d32d5c4ffea  encryption/key.go
6d45dde2da4cb19162c0126fd4f850ff63ccd0f6e49ef93a3e370319e35de266  encryption/kdf.go
ec712e053e32b7c5d5e323f82fcb9eae1b0da1a3e9668215a3790d4a8a3f86c4  encryption/format.go
6f887e2ab32835bec9a7f15b1b7fe828e0e87b723f5bc136364c78e068359957  encryption/record.go
adffe2f3c8c4db58a39e3719a72c5ee69f389eccefde876b4e36285dc663ae93  internal/encstore/publish.go
d3c9cffda1e609d3b3c63b93f0f7faa575832f94b26f3f9d4c85ce133e0c03cf  internal/encstore/snapshot.go
2c6bb2e03b4f413b668ec76bb812bf8842287f889c2b647fa480c7e3a508c836  sqlite/encryption_test.go
124187a0a45eb2dc31f81ca2eedc3482f5a5064ca4dbe4566531636a6cac5ada  sqlite/sqlite_test.go
6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2  internal/testscope/main.go
```

The R6 assertion establishes the limited connection between ordinary reopen, key domains, and observed nonce-bearing publications. It does not observe all unpersisted writer states; existing pure-memory writer tests remain responsible for poisoned-copy/no-resume behavior. R5 interrupted rekey, physical/platform/power-loss work, external cryptographic qualification, and whole-product VERIFIED status are outside this report.
