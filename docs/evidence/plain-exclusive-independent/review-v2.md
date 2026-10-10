# Independent review: WALExclusive managed retirement V2

2026-10-10 UTC. Decision: **accept this scoped exclusive-WAL source increment for integration**, subject to the lead's exact combined-source gate. No remaining blocker was found in the reviewed scope. This is not native failure, physical durability/security, native Darwin/Windows runtime, or milestone/release acceptance.

## Exact source and ancestry

- Candidate: sibling `plain-exclusive-freeze-v2`, based on independently accepted TxV2 and ReaderV5.
- Full manifest `plain-exclusive-v2-all-sha256.txt`: SHA256 `6180d33214507b003d4bf78f4d0da45477b946307ec00a0665fbe3d24fa6a11d`; all 1083 files verified before and after independent checks.
- Fourteen-path owned manifest: SHA256 `027fc39d445ed2c363272218525c3b6a7627d0dd60476c2ba0ad925fb4dd677c`.
- TxV2-to-ExclusiveV2 delta: SHA256 `c5c398e60c038e257382c3a4f32971e26ba596edcd46db8044719bf3c90d9b3b`.
- ExclusiveV1-to-V2 delta: SHA256 `a73341b649026892c578b13212e4c0031c7dd98b7e0d3f10f00fedb43fec7213`.
- TxV2 full manifest: SHA256 `bf450dcdde4293bbd303750d9e14d65d969b8a63fad66249f7c6ba1f8d6eaaa2`; ReaderV5 full manifest: SHA256 `12bb816edaf4b88af31c243bd409fbf2262d7ef1a5f0c1c7a3b5d46eeb725f50`. All 1062 and 1024 respective base files were independently verified.

Each delta was independently applied to a fresh copy of its declared base; all fourteen owned source/document files match V2 byte-for-byte. A separate complete chain from integration tree `508732922205872b7a8738b471b236fcf8ecb096` through ReaderV5, TxV2, ExclusiveV1 and V2 reproduces all 529 candidate Go/go.mod/go.sum files exactly. The integration base is a tree object, not a commit; its historical commit parent 883999c is an ancestor of the pinned main ed4a38e4738a6906027a87cf42dc9f9ac84bc32b. The current-main differences from that original tree include separate oracle test infrastructure and documentation/evidence, not this ownership delta.

Records: `v1-identity.json`, `v1-chain-replay.json`, `v2-identity.json`, `v2-chain-replay.json`, and `v2-final-identity.json`. No unowned Go/module changes exist relative to TxV2. The independent V2 copy adds exactly `internal/pager/plain_exclusive_independent_test.go`, preserved as `v2-probes.go.txt`. Owner freezes and main production sources were not edited; no staging, commit, upload or publication was performed.

## Finding and correction

V1's direct pager scope was bounded before pager metadata, as originally intended. Independent source review found that public WALExclusive canonicalization happened earlier: a Windows managed metadata failure could remain retained by the underlying VFS yet have no plaintext pending-snapshot route when the public failed-open error was discarded. The lead approved extending this increment to that public boundary. The precise source-only finding and prior-scope qualification are preserved in `v1-findings.md`. No negative native path/failure probe was executed.

V2 fixes this exclusive boundary. OpenContext reserves `ExclusiveOpenAdmission` before canonicalOpenPath. Every early return reaches its deferred Finish; Finish adopts the error into the same bounded root and returns a replacement result, so a bounded opaque error is not rejoined to its unsafe original. `OpenSessionAdmitted` claims that exact root once; a transferred admission's later public Finish cannot retire the live Session. A direct OpenSession retains its own pre-pager-acquisition reservation. Cancellation/options rejection before claim leaves the public admission available for the outer Finish. The atomic pointer contains only the root, and the root has no application callback or Session/DB/Rows/Tx/page/undo field.

Independent descriptor-free probes confirmed 100 competing claim-versus-Finish races have exactly one owner and one retirement. A cyclic error is adopted opaquely, remains discoverable, and can be retried through a bounded safe token without re-exposing the original cycle. Owner tests cover early no-DB pending snapshots, one-shot Finish/transfer, and the public full-capacity rejection before invalid input could reach canonicalization.

## Ownership/source assessment

- The common fixed 1024 opening/live/pending root registry charges fixed main/current-WAL/temporary slots before exclusive public or direct pager acquisition. Child reservation precedes acquisition; returned owners are adopted immediately. The temporary slot serves the borrowed transition journal, normalization candidate, or superseded WAL. Nested discovery remains bounded at 4096 error nodes.
- Nested obligations precede temporary retirement, then current WAL, then protecting main. Explicit RetirementComplete is authoritative. A consumed close diagnostic may advance; nil incomplete cleanup cannot. Native terminal close state is retained without retrying a possibly reused numeric handle: Windows closeUnknown and Darwin attempted-record checks were source-inspected. These native exceptional branches were not executed.
- A live retry cannot retire active main/current WAL or an unrequested live temporary owner. Session serialization and poison checks prevent active normalization/candidate acquisition from crossing checkpoint. Successful candidate replacement relabels already-owned slots before old-WAL retirement; pending child cleanup prevents unlink or parent retirement. Source was checked through normal and exceptional branches.
- Logical closure runs once, clears Session files/options/snapshot, and retires only managed ownership afterward. Optional Close checkpoint runs once, is skipped after prior disposal error/poison/KeepWALOnClose, and an active lease returns BUSY without clearing options. A child close that just failed inside Close is not immediately repeated by that same Close. Retry does not reopen paths, unlink, synchronize, checkpoint, republish, replay, or recover.
- The private exclusive mode-transition Tx borrows its Session root and preserves main protection. Closed undo state is cleared; child journal errors and conservative recovery diagnostics survive the outer constructor. Shared WAL's private nil-scope KeepFile branch and encrypted protocols/format bytes are unchanged by this slice.
- DB Close invokes the new exclusive CloseWithError bridge after one-time Rows/Tx invalidation, and repeated Close reaches managed retirement only. Public wrapping preserves error identity and poison/CodeIOErr precedence. Inherited driver disposal may discard cleanup diagnostics, but independent pending root ownership remains reachable. Public Options/DB/Rows/Tx/Plan/Stats layouts are unchanged.
- Root enumeration/admission does not wait on a native/status call. Same-root retries serialize. Historical tokens stay immutable while current snapshots are replaced, and first retained diagnostic history does not grow with retries. Managed resources and diagnostics are retained; arbitrary payload inside opaque error implementations retains the prior documented caveat, so these bounds are not a universal process-RSS bound.

## Independent verification

`v2-checks.sh` and `v2-checks.txt` preserve exact commands and results. Final exit is 0 at `2026-10-10T19:56:48Z`. `v2-test-audit.txt` records selected test/helper behavior. Each test batch was preceded by `go run ./internal/testscope`.

Environment: pinned Go 1.27.2, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false, designated shared cache/path, pinned official SQLite 3.54.0. Passed:

- Selected normal and race pager/VFS/public/driver retirement, admission, provenance, exclusive ordinary read/write/rollback/BUSY/cancellation/close/reopen/checkpoint and uninterrupted normalization tests.
- Independent 256 four-stage dependency combinations, consumed/nil-incomplete/terminal outcomes, first-cause retention, immutable historical tokens, live-owner protection, checkpoint-once/concurrent Close, plus the new admission race and cyclic-error laws. Exceptional outcomes are exclusively descriptor-free callbacks/state.
- Unchanged selected shared-WAL, correct-key encrypted lifecycle/copies/successful uninterrupted rekey, and driver BUSY/pool-reset/WAL/encrypted connector regressions, normal and race.
- Both selected official normal WAL interoperability cases: TestOracleExclusiveWALNormalInterop and TestOracleImportedWALCheckpointWithoutNativeWrite. Official subprocesses finish normally through .quit/EOF.
- CGO_ENABLED=0 `go build ./...`, scoped vet, and empty production CGO dependency output (`v2-production-cgo.txt`). CGO was enabled only for the Go race harness.
- Darwin/Windows amd64 pager/sqlite/driver test compilation. Target binaries were not run.

The V1 independent passing results remain immutable historical evidence. V2 reran the full focused selection; no full default/oracle aggregate was taken from the lead.

## Remaining scope and excluded work

Public rollback Reader canonicalization and shared public canonicalization remain separately identified discovery gaps in this exact V2 freeze. The owner reports a separately authorized later delta; it is not accepted by this review. Legacy shared/recovery absence checks still have the previously documented joined-error/provenance coupling. Integration documentation should retain these boundaries until their own reviewed fixes land.

No physical corruption/tamper/wrong-key/missing selected file, injected namespace replacement/retarget/race, crash/kill/process-exit/hot recovery, native I/O/lock/close/unmap/Sync failure, short-write/disk-full/permissions, or interrupted publication/rekey test ran here, in CI, or another environment. Physical durability/security qualification, native target exceptional execution, broader compatibility/soak and full milestone/release acceptance remain NOT_RUN.
