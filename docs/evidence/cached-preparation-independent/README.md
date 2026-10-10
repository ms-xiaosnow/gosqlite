# Independent cached-schema preparation review

## Scoped verdict

Conditionally accept the exact v3 source tree `18cab5976106fc7a794066d50c9d8a0c199efe49` for the documented ordinary rollback-mode early-BUSY preparation slice. The selected independent normal, race, official-oracle, oracle-race, vet and production build checks passed, ending 2026-10-10 20:41:58 UTC. No unresolved implementation defect was found in that profile after the fixed-charge quota repair.

Acceptance is conditional on retaining the explicit documented boundaries: optional bounded ordinary table/index metadata only, normal snapshot/catalog/ownership authority on every successful admission, conservative no-cache behavior, and no claim of general SQLite preparation or stale-schema reprepare equivalence. This is not a broad-suite pass, physical-safety or native-platform qualification, full SQLite compatibility, or M10 acceptance.

## Exact identities and reproducibility

- Baseline: `sql-connection-counters-freeze-v1`, tree `7c8f7a266a63a713b22709268fdd4d25f1b875d1`, 1,259 files.
- Owner v3 freeze: `sql-cached-preparation-freeze-v3`, tree `18cab5976106fc7a794066d50c9d8a0c199efe49`, 1,264 files.
- Owner full inventory SHA-256: `ebc6d47b2de38c6ea97559ff33559034cbd666f9e2c9f175900ad6df8131e07a`.
- Owner exact source patch SHA-256: `c96429d2427037bfc6c2286ff2a744300484f8f0834c419e4ace6c1bafcd50e0`.
- Independent four-file test patch SHA-256: `3c37c3e236eaba5565066e592974414b2209e7b6c412a3c9a14931d2c23f5d11`.
- Independent review: `sql-preparation-independent-final-v3`; reconstruction: `sql-preparation-independent-replay-v3`.

All 1,264 owner files were checked before and after execution. The declared 14-path delta exactly matches the baseline-to-freeze source inventory. Applying the supplied owner patch to a fresh byte-verified baseline reproduces the complete owner freeze. Applying the independent test patch reproduces all 1,268 review input files. No owner production/test/document source was edited in the review copy. The four additions are `sqlite/preparation_independent_test.go`, `sqlite/preparation_negative_test.go`, `driver/preparation_independent_test.go`, and `internal/testkit/preparation_independent_test.go`.

Evidence: `owner-v3/` snapshots the immutable manifest/patch inputs; `identity-v3-before.json`, `identity-v3-after.json`, `freeze-v3-manifest.json`, `review-input-final-manifest.json`, `freeze-v3-full-before.txt`, `freeze-v3-full-after.txt`, `review-v3-owner-after.txt`, and `patch-v3-replay-identity.txt` establish full path/byte identity. `independent-tests.patch` is ready for optional adoption.

`run-selected-v3.sh` records the exact offline Go 1.27.2 toolchain/cache environment and anchored test selections. `selected-final-v3.txt` and `.exit` record the final successful run. Production/normal/oracle/vet/build use CGO=0; only the Go race detector batches use CGO=1. Every batch follows an explicit behavioral review and `go run ./internal/testscope`. Pinned SQLite is 3.54.0, source ID `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`; tool hashes are in the run log and reference-source hashes in `reference-source-sha256.txt`.

## Findings and independent coverage

The original implementation's fixed 256-byte charge escaped its quota check for an empty catalog because it was checked only inside nonempty loops. An intact empty DB with MaxSortBytes 1, 64 or 255 and a normal Schema call retained 256 charged bytes. `empty-quota-negative.txt` and the original source preserve this real failure. The owner added pre-allocation fixed-charge guards to both cache-construction entry paths. The unchanged independent quota expectation passes on v3.

Source review found a shared semantic preparation path rather than syntax heuristics or a guessed reset. Cache contents have copied table/column strings and an IPK ordinal, no operative root, default-expression AST, declared type, index/extension, record, reader or transaction. It cannot execute anything. Successful snapshot admission still performs current catalog and write-ownership validation. Normal no-cache, unsupported and session modes remain conservative. The retained metadata budget is separate from active-query sort memory and is bounded by min(MaxSortBytes,16MiB).

Independent tests additionally verify:

- Warm/cold malformed SQL and unsupported grammar, missing/wrong parameters, rejected values, pre-cancellation and cancellation from an argument Valuer preserve counters before admission.
- Read-only DB and read-only Tx errors retain their precedence; active transaction Rows return BUSY without resetting counters, even with a cache.
- Copied metadata strings do not retain parser-source allocations; copied columns cannot be mutated through the source catalog; no operational schema state is retained; Close clears cache.
- Automatic CREATE commit BUSY rolls back and invalidates uncommitted metadata. Data-only savepoint rollback after earlier DDL retains current metadata. Driver pool reset rolls back DDL and clears discarded schema state while retaining connection counters.
- Pinned driver prepared invalid DML returns preparation failure; valid DML reaches BUSY/reset; later reuse succeeds with unchanged statement-specific Result contracts.
- Extension-owned schemas and shared-WAL sessions do not gain this cache. Introducing a valid unsupported view through ordinary official SQL cannot make a subsequent native mutation writable through stale metadata. A warm cache never supplies table data under EXCLUSIVE.
- Official typed comparisons cover default-values and rowid/type runtime failures behind BUSY, TRUE/FALSE physical-column shadowing, rowid aliases, invalid dead arms, aggregate misuse and function arity. Values and complete column labels are compared, with native primary codes and official error categories.
- Owner selected histories additionally cover three page sizes, fresh/read-warm states, peer ALTER/DROP/recreate, stale positive/negative binding under EXCLUSIVE, transaction/savepoint DDL invalidation and prior counter histories across rollback/shared WAL.

`source-audit.txt` gives the detailed source and test-scope audit.

## Preserved negatives and harness corrections

- `baseline-repair-negative.txt` fails the strict invalid-DML preparation expectation on the exact predecessor baseline: it returns BUSY rather than the required cached preparation error. Source is retained as `baseline-negative-source.go`.
- `reset-classification-negative.txt` fails after an isolated one-line test-only production perturbation removes the `started` outcome from valid cached-BUSY classification. The strict test observes changes=2 instead of 0. The mutated source and sibling are retained; no owner source was changed.
- `empty-quota-negative.txt` records the real pre-fix budget defect, with source snapshot.
- The first exploratory official script placed ROLLBACK on the same line after an expected failed CREATE; the CLI skipped the remainder. The corrected replay then encountered a transient startup read lock because a newly spawned CLI was not yet synchronized. Both scripts/logs remain. `oracle-boundaries-final.py/.txt` adds a `.print` startup handshake, puts each SQL statement on its own line and confirms the intended histories. A `.print` handshake does not warm schema. The same startup issue independently reproduced in the owner's race-oracle run and its common helper was fixed.
- Initial v2 checksum verification crossed the owner's update of the shared current evidence directory to v3, causing one documentation checksum mismatch. `v2-manifest-race.txt`, `freeze-full-before.txt`, and `patch-replay-identity.txt` preserve that failed identity attempt. The v2 tests finished normally and remain in `selected-final-v2.txt`. All final claims use the locally snapshotted immutable v3 inputs and successful v3 checks.

These failures are not counted as passes. No assertion was weakened to accept an incorrect supported runtime result.

## Genuine remaining implementation boundaries

This cache does not cover session-backed WAL/encrypted or extension/unsupported catalogs, quota-disabled caches, SQL sqlite_schema projections or constant-only SELECT warming. Official SQLite can warm schema on SELECT 1 while the native cache does not. Native explicit deferred constant queries retain their existing snapshot-free behavior.

Full stale-schema reprepare is still different after locks are released: the owner retained a strict oracle failure where SQLite reports a stale INSERT cardinality error until a suitable statement refreshes schema, while native successful admission reads the current catalog. Passing histories explicitly refresh before continuing. This slice qualifies early-BUSY classification, not complete preparation equivalence. General triggers/FK/conflict-policy accounting, INSERT SELECT, RETURNING, views/virtual tables, WITHOUT ROWID and existing broader SQL gaps remain unsupported.

Only intact ordinary cooperative SQL and lock histories were exercised. No physical corruption/tamper/wrong-key/missing-selected-file, namespace mutation, crash/kill/process-exit/hot recovery, injected native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permission failure, interrupted publication/rekey, broad test suite, manual tags, commit, upload or remote action was run.
