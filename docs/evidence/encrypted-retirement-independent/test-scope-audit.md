# Independent encrypted retirement review: execution scope

Review date: 2026-10-10. Baseline: 69008bf658611318154a112178913c693b679a2b.
All execution uses isolated encrypted-cleanup-freeze-vN sources, never the concurrently edited primary checkout. Added independent probes run only in encrypted-retirement-review-vN, a separate copy of the named freeze, and are preserved verbatim as .go.txt evidence.

## Audited selections

- internal/encstore cleanup_test.go: synthetic cleanupResource callbacks mutate only Go state; no native descriptors or OS failures. Includes synthetic database/sql connector disposal, limit and error-tree bounds, dependency order, retry history, secret clearing, and channel-based blocking tests.
- managed_status_default_test.go: explicit consumed/parent-pin state with no native descriptor. Darwin/Windows status/deferred-parent tests are test-compiled only.
- exact_locks_test.go, process_locks_test.go, handle_lifetime_test.go: integer/slice state and descriptor-free callbacks only. Cross-domain process composition uses a new memory darwinfile.Registry and synthetic close callbacks.
- format_test.go TestSnapshotManifestCanonicalMemory, wal_format_test.go TestWALFormatFrozenMemoryVectors, shared_format_test.go TestSharedLayoutFrozenMemoryModeAndProtocol: in-memory serialized bytes and fixed hashes; helpers walGoldenFixture, reviewMemoryMaster, walReviewManifest, walReviewSealRoot are in-memory provider/crypto operations. No file tamper, wrong-key file open, or native I/O failure is executed.
- pager encrypted_close_test.go: memory retirement owner and counting checkpoint closures. The independent actual-session BUSY probe uses an empty Store and active Tx; the early active-lease branch calls no Store or native method.
- sqlite encrypted cleanup classification test: errors and public sentinel handling only.
- TestEncryptedCleanupOrdinaryAllModesCloseAndProviders: intact snapshot/WAL/shared creation, ordinary commit/rollback, Rows invalidation, repeated Close, correct-key read-only reopen and exact provider counts.
- TestEncryptionOrdinaryLifecycleAndCopies and TestEncryptionPasswordNormalReopen: intact correct-key creation, ordinary commits and rollback, successful rekey, archive/restore/import/export, normal reopen. fileview.ReadFile is read-only inspection of valid artifacts.
- TestEncryptedPagerWALFinalDirtySegments: ordinary correct-key page commits, normal checkpoint/rollback/reopen, read-only valid-artifact inspection, ordinary exclusive-writer contention. No injected lock error.
- TestEncryptionSharedCloseBusyAndPasswordRefresh: genuine live writer contention makes the optional checkpoint BUSY; no native callback or syscall is replaced or failed artificially. Normal rollback, successful close and correct-key reopen follow.
- driver TestEncryptedConnectorOrdinaryLifecycle and TestEncryptedSharedConnectorIndependentPoolSnapshots: real database/sql over intact encrypted storage, ordinary transactions/read snapshots and stale-promotion rejection; no failed native operation is injected.
- query, encryptedExec, encryptionOptions, encryptedTestOptions and encryptedProvider helpers were read. They perform ordinary operations or memory provider copies/releases. No TestMain/init test hook was found in selected packages.
- The independent reaper probe uses a synthetic resource whose status method waits on a channel. It acquires no native descriptor and injects no native failure.

Before every ordinary test suite/probe run, the same source's `go run ./internal/testscope` must PASS. The guard is a reference screen, not behavioral proof; the audits above provide the behavioral selection. Tests are selected by explicit anchored regex, not broad `go test ./...` execution. Cross-target `go test -c` compiles but never executes test binaries.

## Execution environment

Tool: ../toolchains/go1.27.2/bin/go (go1.27.2 linux/amd64).
Environment: GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false.
GOCACHE=/workspace/scratch/5e06a74e5f65/durable-restart/go-cache.
GOPATH=/workspace/scratch/5e06a74e5f65/durable-restart/go-path.
Production and normal checks: CGO_ENABLED=0. Race tooling, when recorded: CGO_ENABLED=1 only for Go's race detector; production remains native Go.

## Never executed

No corrupted/tampered database or wrong-key/missing-selected-file case; namespace replacement, retargeting or race injection; crash, kill, process-exit or hot-recovery case; native I/O, lock, unmap, close, Sync, short-write, disk-full or permissions fault; interrupted publication or rekey. Those branches receive source/compile review only. No native Darwin or Windows runtime qualification is claimed. No manual test tags, remote operations, staging, commits, uploads or cache cleaning are performed by this reviewer.

## v2/v3 added laws

Audited the new cleanup memory tests before execution: absent-root result handling passes synthetic pending errors to a pure helper, constructor-order testing uses memory resources, ReaderAt error preservation uses a function-only ReaderAt, and admission status blocking uses a channel-only resource. The direct EncryptedSession test uses an empty Store plus active Tx; its subsequent lease-free close has KeepWALOnClose set and no native owner. None installs or executes a native fault. v3's only source change from v2 is the public ErrEncryptedCleanupPending GoDoc clarification. That exact difference is independently preserved in v2-to-v3-independent.patch. The v3 selected CGO0 and race suites were each immediately preceded by testscope; final target test binaries are compiled, never executed.
