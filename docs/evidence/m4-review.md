# Persistent grams13 engineering review

Date: 2026-10-10. Scope: the ordinary M4 persistent inverted-index subset in
`goal.md`, ADR 0005, the immutable method registry and grams13 adapter, extension
catalog/storage/checker, SELECT planner and configured MATCH, and the SQL/DML/
transaction integration. This is an internal engineering review, not an
independent release, security, or durability certification.

## Conclusion

No remaining ordinary-operation data-correctness blocker was found in the
reviewed implementation after the fixes below. The result remains conditional on
the existing pager/storage/platform scope and cooperative external writers.
Crash/fault recovery acceptance remains **BLOCKED / NOT_RUN**.

## Findings resolved during review

1. **Persisted method versions could narrow on 32-bit platforms.** The catalog
   read converted `int64` method versions to `int` before registry lookup. A value
   such as 4,294,967,297 could become version 1 on a 32-bit target. The loader now
   rejects nonpositive versions and requires `int64(int(mv)) == mv` before lookup.
   This was a static portability finding; no malformed database was created and
   no 32-bit execution result is claimed.

2. **Creation could accept names its own loader refused.** Empty logical index
   names and quoted empty source-field names could reach extension creation even
   though catalog loading rejects those names. Logical-name validation landed
   concurrently with the review report; field validation was added after review.
   Creation now rejects unsupported names before publishing shadow metadata.
   `TestM4ReviewIndexNameAcceptanceMatchesLoader` verifies rejection, unchanged
   source/schema, subsequent native writes, and a valid index/check afterward.
   The pre-fix mismatch was found by source inspection, not by deliberately
   persisting an unusable catalog.

3. **New metadata could exceed the future catalog-read budget.** Creation charged
   the existing catalog and token sets, but an empty-source index could publish
   metadata larger than `MaxSortBytes`, then fail every subsequent catalog load
   under unchanged connection options. Adding another index had the same
   cumulative-budget risk. Creation now reloads staged catalog/fields inside the
   same statement before building; reconfiguration/reindex validates the final
   staged catalog before success. Any quota error rolls back the entire statement.
   `TestM4ReviewIndexPublicationMetadataBudget` passes for an empty source with a
   512-byte quota and confirms no partial shadow schema or lost writability.
   The storage owner's cumulative metadata regression additionally verifies that
   existing successful indexes remain usable when the next creation exceeds the
   total budget. The original mismatch was a static finding, not a damaged-disk
   experiment.

4. **Implicit-rowid VACUUM needed an invalidation boundary.** The lead identified
   that official SQLite can renumber implicit rowids without source DML triggers.
   The storage owner added persisted schema-cookie checks and native refresh rules.
   This review independently verified that refresh only blesses previously clean
   or explicitly rebuilt indexes. Dirty peers remain dirty through unrelated
   native DDL, external schema changes, rebuilding another index, and missing-
   guard repair. See the oracle regression below. The lead owns the direct normal
   VACUUM regression and its evidence.

5. **MATCH fallback diagnostics were verified on the final snapshot.** An earlier
   inspected snapshot lost the dirty-index reason after filtering unusable
   indexes. The planner owner's existing `p.unavailable` fallback resolves this;
   no further production edit was required. Their ordinary
   `TestOracleGramsMatchExactFallbackReason` verifies exact external-writer and
   schema-epoch SCAN reasons, equality with forced scan, and restored SEARCH after
   rebuild (reported pass: 0.155s). The review note was reconciled against the
   current source rather than treated as an unresolved defect.

## Invariants inspected

- **LIKE safety:** index derivation and exact SQL rechecking share `LikeText`.
  `LIKE_RAW_V1` validates complete UTF-8, then applies first-NUL termination and
  ASCII-only folding. It never uses MATCH's NFC/full-casefold/punctuation policies.
  Necessary escaped literal grams give a candidate superset; all candidates still
  evaluate the original complete WHERE. AND may use a necessary child; OR requires
  a safe candidate source for every branch. Unsupported/dynamic/negative/empty
  patterns and excessive candidate work select a real scan.
- **Configured MATCH:** snapshot-bound `grams13_match` uses the persisted
  `MATCH_CONFIG_V1` tokenizer for both exact containment and candidate extraction.
  MATCH and LIKE lookups retain separate namespaces, including mixed boolean
  predicates. Empty normalized needles scan; NULL remains NULL. SELECT-only
  support is documented, without an FTS/positional-search claim.
- **Physical access and diagnostics:** postings use `NewIndexRangeCursor` with
  direct lower-bound descent and a whole-field prefix boundary. Candidate source
  rows use `SeekTable`. SEARCH does not hide a posting/source full scan. Frequency
  probes, posting reads, candidate cardinality, actual base fetches and returned
  rows are tracked separately; EXPLAIN does not execute posting/source retrieval.
- **Atomic maintenance:** source rows, shadow postings, frequencies, configuration,
  guards and generation updates share the pager statement and outer transaction.
  Old/new deltas preserve unchanged posting identities; rowid changes remove old
  and add new identities. Mutation cursors are closed before deleting their tree.
  Failed reconfiguration and quota-failed multi-row DML preserve earlier
  transaction statements.
- **Ownership/invalidation:** exact ordinary shadow schemas, unique nonzero
  schema roots, exact generated guard SQL/source/name/root zero, versioned
  canonical config, source fingerprints and schema cookies are checked. A prefix
  is not ownership. Missing guards can be repaired; changed/general triggers do
  not gain permission to execute. Unknown method/storage/config versions fail
  closed. Native direct shadow mutation is blocked.
- **Resource/concurrency boundaries:** the registry owns immutable maps/descriptors
  and snapshots per connection; the grams adapter clones configuration and uses
  per-call tokenizer state. Registered custom Go handlers remain trusted code
  responsible for their interface contract. Catalog, token, candidate, sort and
  transaction-page budgets are bounded, with no independent extension commit or
  retained source database copy. Race tests exercise the registry and adapter.
- **Reconciliation:** `CheckExtensionIndex` recomputes source keys, checks every
  expected posting and covering-key/table relationship, checks actual totals for
  extra/orphan entries, and reconciles exact document frequencies/source row count.
  It is an extension-specific bounded check, not general SQLite integrity_check.

## Review-owned regressions

`sqlite/m4_review_test.go` contains five ordinary tests:

1. Three indexes over one source, native table/index DDL, DDL rollback, committed
   unrelated DDL, rowid changes, exact index reconciliation and a real SEARCH.
2. A quota-failed configurable rebuild and cooperative pre-cancellation preserve
   canonical config/hash/generation/count/cookie and earlier transaction writes.
3. Multi-row insertion that exceeds a valid configured input quota rolls back
   source/posting/frequency changes for the entire statement, then commits the
   earlier valid statement and retains a usable empty posting range.
4. Empty-source metadata publication obeys the catalog memory budget atomically.
5. Creation and loader agree on empty logical/field-name rejection.

`internal/testkit/m4_review_conformance_test.go` adds
`TestOracleM4ReviewDirtyPeerSchemaEpochs`, in rollback and shared-WAL modes:

- Official source DML invalidates only its index; unrelated native DDL preserves
  a healthy peer and does not clear the dirty index.
- External schema changes invalidate both; rebuilding one then running more
  native DDL leaves the unreconciled peer unusable.
- Recreating a missing owned guard changes the schema epoch without blessing a
  peer invalidated by external DML.
- Normal rebuild/checkpoint/close/reopen passes extension reconciliation, official
  integrity_check, and native-versus-official source-query equality.

The existing oracle helper verifies the pinned official SQLite version/source ID
and waits for normal EOF/process completion. No timeout-kill was added.

## Exact focused commands and results

Working directory: `/workspace/scratch/5e06a74e5f65/durable-restart/gosqlite`.

```sh
export GOTOOLCHAIN=local
export GOCACHE=/workspace/scratch/5e06a74e5f65/durable-restart/go-cache
export GOPATH=/workspace/scratch/5e06a74e5f65/durable-restart/go-path
export SQLITE_ORACLE=/workspace/scratch/5e06a74e5f65/durable-restart/toolchains/sqlite-build-3540000/sqlite3
GO=../toolchains/go1.27.2/bin/go

"$GO" test ./sqlite -run '^TestM4Review' -count=1 -v

"$GO" test -tags oracle ./internal/testkit \
  -run '^TestOracleM4Review' -count=1 -v

"$GO" test -race ./extension/registry ./sqlite \
  -run '^(TestRegistry|TestGramsMethod|TestM4Review|TestGramsConfiguredMatch|TestLikeLiteralNecessaryGramProof)' \
  -count=1 -v
```

All focused invocations completed successfully, exit 0. The initial native run
contained the first three review tests and passed in 0.496s. The two-mode oracle
run passed in 1.021s. The final race run, after the creation/load fixes, included
all five review tests, configured MATCH and 12,000 pure-memory LIKE literal-proof
trials: registry 1.335s; sqlite 4.987s. These are measured test durations, not
performance benchmarks. The lead separately reports the dirty-peer oracle race
pass in 5.334s; broader aggregate/official-oracle evidence belongs to the lead's
final aggregate logs rather than this focused result.

## Remaining scope and risks

- This is a bounded internal source review plus ordinary tests on the available
  Linux environment, not an independent audit, long soak, or all-platform result.
- The review's cancellation test is pre-cancellation. Atomic partial-work rollback
  was dynamically exercised by quota failures; general mid-build cancellation
  cleanup was inspected in source, not established by that pre-cancellation test.
- Schema-cookie mismatch intentionally invalidates all affected cached metadata
  conservatively, including unrelated external DDL. Arbitrary ALTER/rename/drop
  migration and native general triggers remain unsupported or require explicit
  reconciliation. Direct malicious shadow-table edits/restored matching metadata
  are outside cooperative external-writer guarantees.
- Resource limits can reject large all-at-once builds/rebuilds. No unlimited-input,
  benchmark-scale performance, or general ANN extension framework is implied.
- Physical crash/process-kill, I/O-fault/short-write/Sync-error injection, disk-full
  and permission-fault experiments, corrupted-disk tests, hot recovery and manual
  recovery harnesses were **NOT_RUN**. No durability/recovery certification is
  inferred from normal rollback, reopen, checkpoint, race or integrity success.
