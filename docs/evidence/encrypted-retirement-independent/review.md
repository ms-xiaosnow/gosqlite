# Independent review: encrypted retirement ownership

Date: 2026-10-10. Result: **no remaining blocking finding in the reviewed v3 slice**. This is a scoped source/model review plus intact Linux ordinary-runtime evidence. It is not native failure qualification, production-security approval, or completion of M3/M8/M10.

## Exact source and independence

- Durable baseline: `69008bf658611318154a112178913c693b679a2b`.
- Final immutable source: `../encrypted-cleanup-freeze-v3`.
- Its 33-path owned-source manifest, `docs/evidence/encrypted-cleanup-source-sha256.txt`, hashes to `25d5541df56656535db32ff1e344e3888869e1e1c84efd49f828b823434a9fa8`.
- Every owned hash was independently checked before/after validation. `v3-all-go-source-sha256.txt` additionally records every Go source and go.mod/go.sum. `v3-base-comparison.txt` proves zero unowned changes to baseline files; `v3-new-source-comparison.txt` proves zero unowned new Go sources. Concurrent JOIN/migration changes are absent. sqlite.go contains only the encrypted classification and Close GoDoc hunks.
- v1 was reviewed and rejected without changing its freeze. The owner corrected the findings in v2. v3 differs from v2 solely by the requested public-sentinel GoDoc clarification, independently verified in `v2-to-v3-independent.patch`.
- This reviewer did not stage, commit, change remotes, upload artifacts, clean caches, or modify implementation files in the primary checkout. Independent probes were added only to isolated reviewer copies. All reusable evidence here is separate from the implementation owner's evidence.

## Findings and resolution

1. **Active-session BUSY erased options.** v1 EncryptedSession.CloseWithError cleared KeepWALOnClose and other options even when an active lease prevented logical closure. An independent descriptor-free integration probe failed. v2/v3 clear options only after `s.closed` becomes true. The same independent probe now passes, and the final suite includes a direct session law checking options and ownership.
2. **Absent-root probes lost original errors and ownership.** v1 Store.Check/checkAuthority replaced non-NotExist errors with ErrChanged. Windows metadata errors can carry independent managed cleanup scopes. v2/v3 preserve/adopt errors through a pure shared result helper, reserving ErrChanged for an actually present path. New descriptor-free tests prove original-cause retention and lifecycle/directory barriers.
3. **Some constructor/temporary closes adopted dependencies too late.** Joining an operation error after closing could release a parent pin before adopting an independent native probe token. v2/v3 adopt before closing at Store/Open/CreateFrom, object/segment/archive, inventory, standalone checked-file, probe/rename, pager header-failure and public restore boundaries. Direct resource close and constructor-error steps also stop behind unresolved dependencies. These native exceptional routes are source-reviewed, not executed.
4. **Admission could block in resource status.** v1 used a group TryLock but then called potentially blocking RetirementComplete on live resources. A channel-only independent probe failed. v2/v3 admission consumes only atomic per-entry completion proofs and never invokes status, native close, or error traversal. Proofs are published only after full completion predicates under the existing native lock/gate. The same independent probe passes; final laws also prove completed-entry reclamation without waiting on a separate blocked status.
5. **Pre-existing read diagnostic loss.** readExact discarded an underlying ReaderAt error. v2/v3 retain that cause with ErrFormat. A function-only ReaderAt test covers this without any native failure.
6. **Public GoDoc precision.** v3 explicitly distinguishes real cleanup-token errors (also ErrPoisoned/CodeIOErr) from the bare sentinel. Existing public classification laws agree.

Preserved negative evidence: `v1-busy-options-failure.txt`, `v1-busy-options-probe.go.txt`, `v1-admission-failure.txt`, `v1-admission-probe.go.txt`. Their successful unchanged probes are recorded in `v2-independent-probes-pass.txt`. The v1 focused suite had passed; those passes were not treated as a substitute for independent findings.

## Ownership source proof

- Common admission reserves one of 1024 opening/live/pending root scopes and one of 16384 common managed leases before the native acquisition callback. Lower Darwin/Windows physical registries remain independently bounded. Failed validation returns its acquired managed owner before upper cleanup; unknown-inode Darwin quarantine retains a terminal pre-reserved obligation. No eviction or numeric-handle reconstruction was introduced.
- The common registry owns native managed resources directly, role tags, dependencies and diagnostic errors. It has no Store/session/DB/provider/crypto-reader/page-buffer field or application cleanup callback. Logical Store closure clears selected crypto readers, master material, selection buffers and provider-containing options before retained native retirement. Opaque external errors can themselves retain arbitrary data; absolute transitive secret erasure or total RSS bounds are not claimed.
- Whole-scope retry resolves independent dependencies first, then ordinary children, lifecycle owner and directory. A newly discovered dependency stops further retirement before protecting owners are released. Live-token retry acts only on already-requested resources; selected live objects, lifecycle and root protection remain unrequested. Cycles and indeterminate native states fail closed.
- Linux detaches and consumes its os.File lease once, preserves returned diagnostics and distinguishes a consumed error from unresolved ownership. Actual directory pins survive until children retire. No repeated numeric descriptor close is present.
- Darwin releases represented logical byte holds under the shared process-inode gate. Safe idle regular leases can transfer to the native registry while their actual parent pin remains. Delayed close failure publishes atomic terminal/error state to the surviving scope. Directory, unreleased-lock and terminal-uncertainty states are incomplete. Native registry attempted-state prevents repeated raw close. Reuse/parent-pin transfer and callbacks were source-reviewed; no Darwin runtime was executed.
- Windows requires explicit retired state, empty raw handle, no unknown-close tombstone, no remaining exact byte holds and no parent/child pins. A nil deferred close under children is not physical retirement. Failed represented unlock may retry; terminal Close uncertainty never retries the numeric HANDLE. Native owners and pins remain strongly retained. No Windows runtime was executed.
- Registry admission/snapshot locks do not span native calls. Admission reaping uses group TryLock plus already-published completion bits, avoiding both blocked group and blocked file/status work. Native callbacks avoid the common group mutex. Retry serialization and deduplication passed race/memory laws.
- Error discovery is bounded to 4096 visited nodes before effects and deduplicates scopes. Oversized/cyclic caller trees return the limit error without retry effects or rewrapping the unsafe input. Immutable historical tokens, fresh snapshot slices, bounded first/current error history, capacity-before-acquisition and 5000 retries are covered by memory laws. Terminal obligations may consume capacity indefinitely.
- Public failed-open/close routes retain strong retry ownership independently of DB objects. database/sql may discard close errors; PendingEncryptedCleanups preserves the represented scope. This is distinct from the existing plaintext cleanup APIs. Ordinary real driver histories and descriptor-free standard-library pool disposal both pass.

## Exact validation

Commands and output are preserved in the named logs. Environment and per-test/helper behavioral audit are in `test-scope-audit.md`; tool binary SHA-256 is in `toolchain-sha256.txt`. Shared `../go-cache` and `../go-path` were used throughout, with GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off and GOFLAGS=-buildvcs=false.

- Final v3 `go run ./internal/testscope` immediately before **each** executed suite: PASS.
- Final v3 anchored focused suites in internal/encstore, internal/pager, sqlite and driver, `CGO_ENABLED=0 go test -count=1`: PASS. See `v3-focused-cgo0-race.txt`.
- The same final v3 selection under `CGO_ENABLED=1 go test -race -count=1`: PASS. CGO is used only for Go's race tooling; production remains native Go.
- Final v3 `CGO_ENABLED=0 go build ./...` and `go vet ./...`: PASS.
- Final v3 production dependency listing reported no CgoFiles.
- Final v3 whole-tree builds for linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64: PASS. For every target, encstore, pager, sqlite and driver test binaries compiled successfully. See `v3-build-vet-cross-compile.txt`; binaries remain in `../encrypted-retirement-review-builds`. None of those target binaries was executed.
- Frozen snapshot/WAL/shared layout vectors, intact correct-key snapshot/WAL/shared close/reopen, exact provider counts, ordinary rekey/copies, password reopen, Rows/Tx closure, optional checkpoint BUSY, and real driver pool histories all pass in the selected suites.

The production encryption format files and primitive package, KDF policy, lock-byte protocol, module dependencies and frozen layout vectors remain unchanged from the baseline. New public cleanup APIs do not change ciphertext framing or persisted coordination bytes.

## Negative findings and remaining gaps

Native I/O/lock/unmap/close/Sync/short-write/disk-full/permission failures, file/database corruption or tamper, wrong-key/missing-selected-file cases, namespace replacement/retarget/race injection, crash/kill/process exit/hot recovery, and interrupted publication/rekey are **NOT_RUN**, not moved to another environment. Descriptor-free callbacks establish bookkeeping laws only. Native Darwin/Windows execution and native Linux failure qualification remain absent.

Existing limitations remain: plaintext rollback Reader/Tx and exclusive-WAL ownership are outside this slice; explicit plaintext import retains that older source contract. Windows encrypted support is restricted to the implemented amd64/fixed-local-NTFS contract, has no Unix-style directory-sync promise and no atomic shared-lifecycle rekey conversion. Darwin basename/Unicode policy and fsync-versus-F_FULLFSYNC limitations remain. Other encrypted OS backends fail closed. This does not establish full SQLite compatibility, historical-key revocation, external identity/freshness, crash durability or security/legal acceptance.

No further blocking defect was found in the exact reviewed v3 slice. Applying it to a newer durable SQL base still requires the lead's exact integration/aggregate verification; this review does not certify the mixed working tree.
