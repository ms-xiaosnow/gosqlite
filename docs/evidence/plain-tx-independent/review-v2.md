# Independent review: standalone rollback Tx/journal retirement V2

2026-10-10 UTC. Decision: **accept this scoped TxV2 source increment for integration**, subject to the lead's exact combined-tree gate. No remaining blocker was found in the reviewed scope. This is not full database durability/security, native failure qualification, native Darwin/Windows runtime acceptance, or milestone/release acceptance.

## Exact source

- Candidate: sibling `plain-tx-freeze-v2`, based on independently accepted `plain-reader-freeze-v5`.
- Full TxV2 manifest: `plain-tx-v2-all-sha256.txt`, SHA256 `bf450dcdde4293bbd303750d9e14d65d969b8a63fad66249f7c6ba1f8d6eaaa2`; all 1062 files verified before and after checks.
- ReaderV5 full manifest SHA256 `12bb816edaf4b88af31c243bd409fbf2262d7ef1a5f0c1c7a3b5d46eeb725f50`; all 1024 base files independently verified.
- TxV2 13-path owned manifest SHA256 `9d95728c0541ee69f1d3f37878a1287c347baf98b982e594964bcd3241977dab`.
- ReaderV5-to-TxV2 source delta SHA256 `688836b91343cb1c2c95c78adb2358e6485f7f72611254ede02ea2777df69e5a`. Independent application to a fresh V5 copy reproduced all 13 owned source/document files byte-for-byte.
- TxV1-to-TxV2 source delta SHA256 `07427570172fca61387a7a06a5fb91b9003bd5fae3c68f7b29e94d99d94a2432`.

`v2-identity.json`, `v2-base-replay.json` and `v2-final-identity.json` contain the checks. No unowned Go/go.mod/go.sum differences exist between the V5 base and TxV2. Independent copy `plain-tx-independent-v2` differs from the full manifest only by the two declared independent `_test.go` files, preserved here as `.go.txt`. Candidate freezes and main production sources were not changed. No stage, commit, upload or publication was performed.

## Review findings and corrections

The original TxV1 negative evidence is retained in `v1-findings.txt`, `v1-probes.txt` and `v1-public-probe.txt`, with its exact failing source and manifests.

1. First journal-retirement diagnostic loss is fixed. `closeTemporary` now preserves both the first operation cause and first requested child-retirement attempt before whole closure. Deferring whole retirement neither repeats the child cleanup nor discards its diagnostic. Further retries do not append history.
2. Closed undo retention through direct Exclusive poison is fixed. `retirePlainLocked` clears closed undo centrally before attempting potentially pending retirement. This covers the poison call lacking Commit/Rollback's outer defer.
3. Automatic `DB.Exec` rollback code precedence is fixed. The actual production `Tx.rollbackAfterExecLocked` helper classifies the full join through `wrapCleanup`; a preceding CodeBusy cannot mask current pending ownership's CodeIOErr. `DB.Exec` calls this helper. The independent public probe retains all original assertions and now invokes that exact helper. Driver ExecContext's new `executionError` helper invalidates poison connections, joins disposal diagnostics and preserves CodeIOErr precedence while leaving ordinary BUSY/execution errors reusable.
4. Pending journal-constructor ownership now carries conservative RecoveryRequired. `openJournal` failure passes the journal-acquisition phase into preflight; earlier pre-I/O failures do not. This preserves child/main barriers without replaying any publication or cleanup.
5. Direct OpenTx now consumes the VFS initial-no-handle absence boolean inherited from ReaderV5. Its create fallback no longer depends on legacy os.IsNotExist error shape. The prior V4 post-acquisition example was not a demonstrated direct-OpenTx bug and is explicitly withdrawn in the historical report.

The first two independent pager repros are byte-identical across V1/V2; they fail on V1 and pass on V2. The owner also added production-helper memory regressions for the same findings. No native exceptional operation was injected to demonstrate any correction.

## Ownership/source assessment

- The same fixed 1024 opening/live/pending plaintext bound is reserved before standalone path resolution/main acquisition. Main and journal are fixed primary slots; journal admission is marked before acquisition and unused nil-result slots are relinquished. Failed constructors adopt error-carried owners before the main protector can retire.
- Error-carried nested obligations precede journal retirement, and an incomplete journal precedes main retirement. Explicit RetirementComplete is authoritative: consumed diagnostics may advance, nil/noncomplete cannot, and terminal owners remain pending without numeric-handle retries.
- Live retry cannot retire the active main or an unrequested live journal. Whole logical closure removes the Tx's usable file/pages/free/undo state; retries perform managed retirement only. No retry reopens a path, unlinks a journal, syncs, publishes, replays a transaction, checkpoints or runs recovery.
- Registry admission/snapshot enumeration never holds a resource/native/status mutex. Same-group retries serialize. Discovery is bounded before effects; historical tokens remain immutable, current snapshots replace prior tokens, and fixed first-cause history does not grow per retry.
- The retained group has no Tx, DB, Rows, Options, page/undo buffer or application cleanup callback field. Managed resources and diagnostic causes remain; opaque error implementations retain their documented arbitrary-payload caveat. Retaining only a numeric descriptor would not satisfy this assessment and is not the implementation used here.
- Public closed transactions and their Rows are invalidated once. Native pager Rollback can retry its group; public ended Tx cleanup uses the independent retry/snapshot route. Driver disposal may discard an error, but independent scope ownership persists.
- KeepFile remains the private WAL transition branch with no standalone cleanup group. Shared-WAL/encrypted algorithms and format/layout bytes are unchanged by this delta. Their previously reviewed independent ownership contracts are not generalized to WALExclusive.

## Independent verification

All commands and outcomes are in `v2-checks.sh` and `v2-checks.txt`; final exit is `0`, completed `2026-10-10T19:43:16Z`.

Environment: pinned Go 1.27.2; `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS=-buildvcs=false`, designated sibling Go cache/path and pinned official SQLite 3.54.0 oracle. Each selected suite is preceded by `go run ./internal/testscope`. Behavioral selection/helper audits are in `v2-test-audit.txt` and the prior audit.

Passed:

- Scoped normal and race tests for pager/VFS/public/driver retirement, absence and ordinary rollback lifecycles.
- All three independent corrected laws plus 64 complete/consumed/nil-noncomplete/terminal combinations across nested child, journal and main. The stage law checks ordering, explicit completion, no terminal numeric replay, stable history, current completion and idempotent historical-token retry.
- Intact shared/exclusive-WAL lease/checkpoint/close regressions and correct-key encrypted read/write/rollback/rekey/copy/reopen plus driver BUSY/pool-reset/connector lifecycles, both normal and race.
- All four selected official `TestTxOracle` cases: standard file/freelist/row/index interoperability and cooperating ordinary contention/managed-probe lifetime.
- `CGO_ENABLED=0 go build ./...` and scoped `go vet`.
- Production dependency CGO listing is empty (`v2-production-cgo.txt`). CGO was enabled only for the Go race harness, never as a production SQLite backend.
- Darwin and Windows amd64 pager/sqlite/driver test compilation. Target binaries were not run.

This is a focused independent selection, not a new full-suite or forbidden failure campaign. No corruption/tamper/wrong-key/missing selected file, namespace mutation, crash/kill/process-exit/hot recovery, failed native I/O/lock/unmap/close/Sync, short write/disk full/permissions or interrupted publication/rekey was run here, in CI, or moved to another environment by this review.

## Remaining real scope gaps

WALExclusive acquisition, later WAL/candidate acquisition/replacement, checkpoint and Session.Close ownership remain deferred source work, including the private KeepFile transition's separate lifetime contract. This increment does not repair those paths. Native OS exceptional-path execution, physical durability/security acceptance, soak, broader compatibility and independent release qualification remain NOT_RUN. Earlier Reader/shared/encrypted documents retain their historical increment boundaries; integration documentation should refer readers to this Tx extension when describing current standalone rollback coverage.
