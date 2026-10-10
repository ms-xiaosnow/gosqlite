# Independent bounded GROUP BY / HAVING review

## Verdict

**Accepted for the declared bounded grouped-aggregate profile at exact tree `6774495e7289aa1cf7a652df673ba2610117900e`. No unresolved review finding on that candidate.**

Candidate: sibling `grouped-aggregate-freeze-v2`. Independent execution copy: sibling `grouped-review-copy-v2`. Baseline: `aggregate-combined-lead-verify`, tree `8d7a0fcc5a2e3a83850ea780b64b403047677568`.

V1 required changes. The reviewer found two supported-query error-timing families; the owner corrected both and preserved the original failures. This acceptance is a bounded integration review, not broad SQLite compatibility, milestone completion, a full combined-tree gate, or durability/security/release qualification. The lead's inherited diagnostic and obsolete sort-quota assertion repairs are separate from this grouped patch. The tiny exact-tree sort assertion correction is independently accepted in `SORT-FOUNDATION-REVIEW.md`.

## Corrected findings

1. **P2: general ORDER BY/LIMIT projected groups rejected by SQLite's top-K sorter.** V1 returned an integer-overflow error for a scalar projection in a later worse-ranked group, although official SQLite returned the valid limited result. Initial independent matrix: 508 cases, nine failures in this family. Additional 306 top-K cases exposed 84 more, including equal keys, NULL ordering, aliases/ordinals, OFFSET and int64-bound saturation. V2 evaluates HAVING and sort keys before candidate admission; only accepted candidates evaluate remaining projection expressions. Required alias/ordinal projection values are computed before admission. Equal keys preserve earlier candidates. Aggregate finalization, sort-key errors, cancellation and resource failures remain visible. Original cases now pass unchanged.

2. **P2: unique physical-rowid grouping read predicates beyond SQLite's LIMIT stop.** V1 consumed all input, exposing a third-row WHERE overflow after an officially successful first group. Exact positive mismatches also covered NOT INDEXED, CASE predicates, explicit rowid ordering, OFFSET, duplicate/composite grouping keys, and a rowid key in a later position. V2 proves uniqueness only for a single leaf with an unwrapped physical rowid/IPK grouping term. It preserves the next qualifying row's WHERE/key lookahead before emitting the prior group, while delaying that next group's aggregate arguments. Reverse scans use the existing native bounded rowid cursor. Mismatched ORDER shapes and nonnatural NULL placement retain a real sorter. Required lookahead failures remain failures. Plans distinguish `grouped_rowid` from generic `grouped`, and observed leaf counts confirm real early stopping.

V1 evidence remains in `review-v1.md`, `probes-v1.txt/.exit`, `rowid-stream-v1.txt/.exit`, and `expanded-v1.txt/.exit`. Original and expanded test sources are preserved separately. Official-only control probes that disproved an overly broad ordering hypothesis are explicitly labeled, not counted as product failures.

## Semantic and source review

- Read goal.md, aggregate/relational/Boolean execution documents and the prior independent aggregate review. No applicable AGENTS.md was found in the checkout or checked ancestors. Compared grouping, HAVING promotion, aggregate discovery and representative handling with pinned public SQLite 3.54.0 `src/select.c`, `src/expr.c`, and the reviewed `src/func.c` arithmetic contract.
- Compared complete labels, values and storage classes for successful queries, and ordinary SQL errors for failures. NULL and numeric-equal INTEGER/REAL keys coalesce; TEXT/BLOB and full BINARY bytes remain distinct. NUL data is compared via hex/typeof, avoiding the official CLI quote display's truncation. Large positive/negative int64/REAL boundaries, zero, composite keys, empty values, aliases/ordinals, prepared bindings and source affinity were exercised.
- MIN/MAX retain earliest equal aggregate values and control representative source-cell selection in the pinned discovery order. Computed root grouping expressions retain their saved key; nested expressions use the representative. Deduplicated and repeated MIN/MAX calls, all-NULL cases and HAVING/ORDER-only aggregate discovery passed.
- Key-only/constant HAVING conjunct promotion remains distinct from post-finalization aggregate predicates; tests include typeof-sensitive numeric-equal inputs. LIMIT 0 and EXPLAIN bind without opening source cursors, validate both bounds, and suppress the correct runtime evaluation. Bare columns, wildcard projections, DISTINCT, windows, collations, FILTER and aggregate-local ordering remain fail-closed where unsupported.
- The four promoted exact parser-negative statements have complete positive pinned-oracle comparisons. The grouped wildcard and unrelated negative assertions remain. Unsupported forms remain implementation work under the original goal; this review does not waive them.

## Bounds and ownership

Source inspection verified admission before key/value detachment, per-group state and pointer-array growth, representative replacement, MIN/MAX replacement, deferred SQL-error retention, output buffering and top-K capacity growth. Old candidates remain charged until replacement ownership is established. Comparison and insertion movement consume the shared work budget; resource/cancellation errors are not deferred with SQL errors. The reverse cursor wrapper preserves the root reader's page and record preflight hooks.

Only the root Rows owns the snapshot and transaction activeRows lease. Private input/argument adapters and restarted join children use the same reader, predicate scope, context and function registry. Normal shared snapshots, transaction-own writes, BUSY, rollback, prepared reuse, cancellation and close paths pass. No extra connection, recursive Query or snapshot is introduced.

An additional independent 40-row/1-KiB-BLOB test exercises top-K capacity growth and repeated replacement, reverse-rowid early stopping and general ordered rowid grouping. Reducing either measured work or peak memory by one rejects before any public row; accounting retires, retained public BLOB copies stay independent, and the connection still executes subsequent queries. Normal and race runs pass (`budget-v2.txt/.exit`). Accounting remains conservative rather than an RSS guarantee; no spill or group-index performance claim is made.

## Final verification

PASS on the exact isolated v2 source:

- All **824 unchanged reviewer comparisons**: original 508, 306 top-K details and ten rowid controls, normally and under race.
- Selected grouped parser/native/driver and owner pinned-oracle tests, normally and under race, including the owner's 140 additional rowid/order/error controls.
- Extra independent retention/admission checks, normally and under race.
- Whole-tree `go vet ./...`, CGO-disabled `go build -trimpath ./...`, `go mod verify`, and a CGO-disabled sqlite/driver dependency closure with no CgoFiles.

Every executed test batch was behaviorally audited and preceded by `go run ./internal/testscope`. Tools: pinned Go 1.27.2 and official SQLite 3.54.0 source ID `be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`, offline modules/shared cache and GOFLAGS=-buildvcs=false. Exact commands/output: `run-final-v2.sh`, `final-v2.txt/.exit`, `run-budget-v2.sh`, `budget-v2.txt/.exit`.

No independent full default/oracle suite or cross-platform runtime pass is claimed. The owner records core cross-builds; final combined integration and complete gates belong to the lead.

## Inventory integrity

Verified all **1,265 candidate hashes**, **1,225 baseline hashes**, exactly **52 owned paths**, and **zero removals**. Independently rebuilt both trees and applied the full patch to a fresh baseline copy; every resulting hash matches. Before/after freeze and execution-copy checks show no candidate modification; only the three explicitly listed reviewer test files were added to the execution copy. goal.md, go.mod and go.sum are unchanged. Existing public sqlite/driver struct layouts are unchanged; only internal Select gains grouping fields.

- Final tree: `6774495e7289aa1cf7a652df673ba2610117900e`
- Full patch SHA256: `d456e8caef731c0c3ee897e774e4c98e5d5c59a364111276a45eca925b521577`
- V1-to-v2 patch SHA256: `c1b0ea77824b4b5ef5c7096f5c5c741f06ce5927cd6437399ed5520514352110`
- Full manifest SHA256: `589b97e8d5be0d0565ddcf2b164090b51db155fc88f55a0b5cbc92bd03b4c6f1`

Evidence: `inventory-v2.json`, `after-v2-inventory.json`; original v1 inventory and unchanged-manifest evidence are retained.

All database execution used intact owned fixtures and ordinary authorized SQL/lifetimes. No physical corruption/tamper, wrong-key/missing-file, namespace mutation/retarget/race, crash/kill/process-exit/hot recovery, native I/O/lock/close/unmap/Sync failure, short-write/disk-full/permissions, interrupted publication or rekey workflow was run. No main source edits, staging, commits, uploads, cache clearing or process termination were performed. Main-checkout writes were confined to this review-evidence directory.
