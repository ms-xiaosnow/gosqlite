# Independent review: public plaintext cleanup adoption V1

2026-10-10 UTC. Decision: **accept this scoped public-open source increment for integration**, subject to the lead's exact combined-tree gate. No remaining blocker was found in the reviewed scope. This is not native failure qualification, physical durability/security, native Darwin/Windows runtime, or milestone/release acceptance.

## Exact source and ancestry

- Candidate: sibling `plain-public-adoption-freeze-v1`, based on accepted `plain-exclusive-freeze-v2` → TxV2 → ReaderV5.
- Full manifest `plain-public-adoption-v1-all-sha256.txt`: SHA256 `ad05ae86ce81cd1eeca2603bc535f508ce5c773da4b59bc49c156480e384cc33`; all 1097 files independently verified before and after checks.
- Nine-path owned manifest: SHA256 `9c91e9f4ead5924ee7ff664803001017d91ee621888b5db74a9bd99efa6fe601`.
- ExclusiveV2-to-publicV1 delta: SHA256 `dd3919ba517674d47f395d6ae8ca399a378004c6166068dc4fe796b2da1b17d1`.
- ExclusiveV2 full manifest: `6180d33214507b003d4bf78f4d0da45477b946307ec00a0665fbe3d24fa6a11d`; all 1083 files verified.
- TxV2 full manifest: `bf450dcdde4293bbd303750d9e14d65d969b8a63fad66249f7c6ba1f8d6eaaa2`; all 1062 files verified.
- ReaderV5 full manifest: `12bb816edaf4b88af31c243bd409fbf2262d7ef1a5f0c1c7a3b5d46eeb725f50`; all 1024 files verified.

The new delta was independently applied to a fresh ExclusiveV2 copy and reproduced all nine owned files byte-for-byte. No unowned Go/go.mod/go.sum changes exist. Independently replaying the complete four-patch chain from integration tree `508732922205872b7a8738b471b236fcf8ecb096` reproduces all 532 candidate Go/module files. The base is a tree object, not a commit. Patch hashes and replay commands are preserved in `v1-chain-replay.json` and `replay-chain.py`; other identity checks are in `v1-identity.json`, `v1-final-identity.json`, and their scripts.

The independent copy `public-cleanup-independent-v1` adds exactly two reviewer test files, preserved as `v1-pager-probes.go.txt` and `v1-sqlite-probes.go.txt`. No owner freeze or main production source was changed. No staging, commit, upload, or publication was performed.

## Ownership assessment

1. Public plaintext rollback and exclusive opens reserve the same fixed 1024-root registry before `canonicalOpenPath`. The admission is retained through method/options preparation and transferred once into `OpenAdmitted` or `OpenSessionAdmitted`. Direct pager Reader/Session callers retain their prior before-acquisition reservation. No extra native probe or platform call was added.
2. Every early return after reservation reaches deferred `Finish`. An unused plaintext root adopts the error through the existing bounded plaintext state machine. The handled result replaces the original rather than rejoining an unsafe graph. A transferred or already-finished admission cannot close the live Reader/Session. The exclusive-only internal names remain aliases/wrappers to the same one-shot root.
3. `SharedCloseAdmission.Finish` atomically takes the unused group under its admission mutex, then releases that mutex before retirement. The new `beginPublicOpen` bounds VFS owner discovery at 4096 nodes and adopts metadata obligations into the existing close group. The normal shared-session close and failed-pager-open state machines are byte-unchanged. The separate 64-group `RetryOpenCleanup` route remains responsible after pager acquisition; public prelude failures instead use `PendingCloseCleanups`/`RetryCloseCleanup` and the already-reserved 1024 close-group limit.
4. Error-carried VFS obligations are discovered before effects and retain their established dependency order. The same explicit `RetirementComplete` law applies: consumed diagnostics can advance, nil without completion cannot, and indeterminate native ownership remains terminal. The new path does not reopen, resolve, unlink, checkpoint, recover, publish, or retry a numeric handle. The Windows metadata tombstone producer and returned cleanup-error chain were source-inspected; public retry discovery is now supplied in addition to the VFS's strong retention.
5. Both registries publish immutable current tokens independently of DB/Reader/Session reachability. Healthy live/opening groups remain excluded from snapshots but continue to count toward admission. Registry enumeration/admission does no owner/native work or group-mutex wait. Retries serialize per group. First-cause history remains fixed; current snapshots replace old tokens. Completion drops retained owners/history and releases admission.
6. The roots retain managed resources, bounded discovery bookkeeping and diagnostic causes, with no DB, Reader, Session, Tx, Rows, Options, page buffer or application cleanup closure field. Opaque custom errors can retain arbitrary payload and execute their own methods; the structural bound is not a universal transitive-memory/RSS bound.
7. Cancellation/invalid-option preflight keeps its earlier precedence. The Reader's private initial-no-owner absence provenance is unchanged. Ordinary initial absence completes the transferred Reader root before a distinct create-Tx root is acquired. Additional operation/retirement diagnostics or pending ownership still cannot authorize create. Independent ordinary work with 1023 empty holds leaves exactly one acquisition slot and passes creation, write, close, existing-file writable/read-only reopening and query. This confirms transfer avoids double reservation and that healthy idle rollback DBs do not pin the root.
8. Encrypted source and format algorithms are unchanged. Windows `CanonicalWindowsPath` was traced through `openDirectory`'s encrypted root/lease reservation, scoped metadata children and `closeResult`/group retention. It already has its independent encrypted pending/retry route. No new encrypted ownership guarantee is inferred from this plaintext increment.

## Independent checks

`v1-checks.sh`, `v1-checks.txt`, and `v1-checks.exit` contain the exact focused commands/results. Final exit is **0**, completed `2026-10-10T20:03:50Z`. `v1-test-audit.txt` records behavior/helper review; `internal/testscope` ran before each test batch.

Environment: pinned Go 1.27.2; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false, designated shared Go cache/path, and pinned official SQLite 3.54.0. Passed:

- Selected normal and race plaintext/shared admission, Reader/Tx/exclusive cleanup, absence, VFS discovery/status, public wrapping, and intact ordinary rollback/shared/exclusive lifecycles.
- Independent 200 plaintext plus 200 shared competing transfer-versus-Finish races require exactly one owner and one retirement. Independent consumed/nil-incomplete/terminal callback laws verify explicit completion, stable history through 5000 retries, no terminal numeric-operation replay, current snapshot removal and immutable/idempotent old tokens.
- Independent cyclic-error law bounds initial discovery and prevents both Error formatting and Unwrap traversal of the retained unsafe original during subsequent retries. Owner and inherited laws cover no-DB pending discovery, admission exhaustion before path handling, all stage barriers, snapshot/mutex independence, and database/sql discarded-close behavior.
- The new single-slot ordinary public lifecycle and canceled/invalid-option preflight checks. No physical metadata failure was induced; exceptional adoption evidence is compositional source plus descriptor-free VFS/group laws.
- Existing shared snapshot/checkpoint, correct-key encrypted read/write/rollback/copies/uninterrupted successful rekey/reopen, and driver savepoint/deferred/BUSY/pool/WAL/encrypted regressions, normal and race. The ordinary 65-live-shared-session case and failed-open-capacity separation law also pass.
- All four selected `TestTxOracle` cases and both normal exclusive-WAL interoperability/imported-checkpoint cases. Official processes complete through ordinary EOF/.quit; no interruption helper is used.
- CGO_ENABLED=0 `go build ./...`, scoped `go vet`, and empty production CGO dependency output. CGO is enabled only for Go's race harness.
- Darwin and Windows amd64 pager/sqlite/driver test compilation. These target binaries were not run.

This is a focused independent gate, not a default/full-suite or full-oracle run. There were no negative test results in this review; earlier Reader/Tx/exclusive negative findings and their historical fixes remain preserved in their prior reports.

## Combined Rows source interaction

At the lead's request, separately checked combined tree `2d57bc8f9fa728c737bb88d470d4f91db87791d2` over SQL base commit `1e413b466d10db5c7304ba231fd5d4536dfc5fa5`. All 45 declared owned hashes match. Forty-four paths match publicV1 exactly. `sqlite/rows.go` differs only because it retains the separately reviewed Boolean/rank startup implementation from the SQL base.

The exact base-to-combined Rows delta is one line in `Rows.fail`: the previous discarded `r.close()` result is now assigned through `wrapCleanup("next", r.err, r.close())`. Boolean predicate fields, preparation, once-only startup caching, EOF behavior and close-time clearing are byte-preserved. Startup errors and EOF both flow through this existing failure/close boundary; close is still once-only, and a newly pending cleanup retains CodeIOErr precedence. No source interaction blocker found. `combined-rows-review.json` and `combined-rows-source-diff.patch` preserve this audit. No tests ran on the lead's combined tree in this review; its exact combined gate remains lead-owned.

## Remaining scope

Legacy direct shared/recovery `os.IsNotExist` guards retain their documented coupling to joined-error shape instead of explicit provenance. That separate source issue is not silently repaired here; no current Windows post-inspection create bug was established for those callers. Historical earlier docs have narrower increment scopes; current integration docs should point to this public adoption extension.

No physical corruption/tamper/wrong-key/missing selected file, namespace replacement/retarget/race injection, crash/kill/process-exit/hot recovery, native I/O/lock/close/unmap/Sync failure, short-write/disk-full/permissions, or interrupted publication/rekey ran here, in CI, or elsewhere. Native target runtime/exceptional-path execution, physical durability/security, broad compatibility/soak and M10/release acceptance remain **NOT_RUN**.
