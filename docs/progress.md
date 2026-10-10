# Implementation and acceptance progress

Fresh restart, 2026-10-10 UTC. Historical work and test results are not acceptance evidence. Original goal SHA-256: 9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81.

### M0: Baseline and scaffold
- Status: VERIFIED
- Deliverables: pinned Go module, native CLI version/help, unit tests, ordinary-check script/CI, architecture/compatibility/security/format ADRs, durable original checkpoint.
- Key ADR: 0001-native-core-and-evidence; official SQLite is test-only; exact remote history and independent persistent bundle backups.
- Code: cmd/gosqlite, internal/buildinfo, scripts/check.sh, go.mod, .github/workflows/check.yml.
- Actual checks (Linux/amd64, Go 1.27.2, 2026-10-10): CGO_ENABLED=0 go test ./... PASSED (CLI package and buildinfo compiled); CGO_ENABLED=0 go vet ./... PASSED; two CGO_ENABLED=0 go build -trimpath -o <distinct path> ./cmd/gosqlite builds byte-identical; binary version output matched all baselines; go test -race ./... PASSED (race tooling with CGO enabled). Default home cache attempt FAILED because home is read-only; rerun with writable GOCACHE/GOPATH succeeded. Initial go.mod needed go mod tidy to remove redundant equal toolchain directive; fixed, go line remains exact 1.27.2.
- Oracle: official SQLite 3.54.0 source zip published SHA3-256 and generated sqlite3.c SHA3-256 independently matched; CLI source ID and in-memory integrity_check matched. No database-engine differential test exists yet.
- Limits: scaffold only. All M1–M10 database features unsupported. Ordinary-only remote CI PASSED for checkpoint 7742337875980eaa1fb78a7a6e6f2bd4c00d9114: https://github.com/ms-xiaosnow/gosqlite/actions/runs/38040601986.
- Risk/safety/migration: no database writes implemented, no production readiness claim; source/license review is informational and not an independent legal audit.
- Next: M1 read-only SQLite header/record/B-tree/schema and limited SELECT against official fixtures.

### M1: Native Go read-only SQLite
- Status: VERIFIED (documented Linux read-only subset)
- Deliverables: validated header/record/varint codecs, streaming table B-trees and overflow/schema reads, SELECT executor and parser, database/sql driver, query/inspect CLI and official fixtures.
- Key ADR: 0002-readonly-locks-and-limits; shared-lock handle lifetime, bounded resource usage and fail-closed unsupported formats.
- Code: internal/record, internal/pager, internal/btree, internal/sql, sqlite, driver, examples/readonly, cmd/gosqlite; oracle-tagged internal/testkit and committed testdata.
- Actual checks: final CGO_ENABLED=0 unit/vet/build and separate CGO_ENABLED=1 race gates passed; complete oracle-tagged suite passed with exact3.54.0 version/source verification; race-enabled oracle conformance passed. Four-target CGO-disabled builds passed (Linux amd64/arm64, Darwin arm64, Windows amd64). Four memory-only fuzz targets each ran5 seconds and passed; parser fuzz completed 48,614 executions. See docs/evidence/m1-aggregate-final.txt, m1-aggregate.txt and m1-memory-fuzz.txt.
- Official comparison:5,160 expression/affinity cases in one suite; end-to-end suite 9 top-level tests/272 events with80 seeded predicates and300 seeded scalars. Real512/4096/65536-byte fixtures each contain1,028 records, plus separate schema/ALTER fixtures. Oracle integrity_check=ok; Go reads preserve source SHA-256. See docs/evidence/m1-conformance.md and fixture manifests.
- Limits: writes/transactions/recovery/WAL, index traversal/planning and unlisted SQL are unsupported; native non-Linux Open fails; full integrity checking is absent; ORDER BY is bounded in-memory. Cross-builds are not native-OS verification.
- Risk/safety/migration: no database modification path; existing nonempty journals rejected rather than recovered. Read-only tests do not qualify durability, release security or legal review. Initial SQL discrepancies were corrected with retained oracle regressions.
- Next: M2 native transaction overlay/allocator, mutable table and index B-trees, journal-backed publishing and SQL CRUD, while keeping prohibited fault workflows manual/unverified.

### M2: Native writes and SQL MVP
- Status: VERIFIED (documented ordinary Linux SQL/write MVP; crash/fault durability excluded)
- Deliverables: SQLite freelist allocator, bounded transaction/statement overlays, stable-root local table/index split/merge and overflow mutation, native Linux writer locks, standard DELETE journal normal-success publication, CREATE TABLE/INDEX and CRUD, explicit native/database/sql transactions, writable CLI and examples.
- Key ADR: 0003-write-journal-and-fail-closed. Existing journals refuse recovery; ambiguous physical-I/O errors poison the transaction. No crash safety claim.
- Code: internal/pager/{tx,freelist,commit,write_lock_*}.go, internal/journal, internal/btree/{mutable,index_read,ownership}.go, sqlite/{transaction,write_rows,write_schema}.go, driver, cmd/gosqlite, examples/write.
- Checks and official comparison: final CGO0 unit/vet/build, separate CGO1 race and full official-oracle suite PASSED; four-target CGO0 cross-builds PASSED, runtime remains Linux only. Six fixture manifest hashes and original goal hash unchanged. See docs/evidence/m2-aggregate.txt and m2-review.md. Ordinary-success oracle tests passed at512/4096/65536 bytes; official integrity_check and forced index reads matched. Valid-memory randomized table/index models, statement rollback, normal cancellation, constraints, BUSY retry, concurrency, normal cross-process lock contention and race checks passed in component runs.
- Review: canonical connection paths prevent cwd/symlink retargeting; uncertain PENDING cleanup poisons; journal refusal classification unified; whole-file page ownership and equal-depth checks prevent unsafe mutation of unsupported structure. The review is engineering source review, not independent security/legal certification. A real legacy-test failure mutated a generated fixture; explicit read-only options and exact verified fixture restoration fixed the cause. Final aggregate passed with unchanged fixture hashes.
- Limits: ordinary nonunique BINARY ASC indices maintained but SELECT still scans; unknown schema/constraints fail closed; only Linux runtime; no WAL/recovery, no savepoints, no full integrity_check. Existing zero-length files rejected. Public SectorSize is an explicit4096-default assumption, not device certification. Dirty-page and materialization budgets may reject large transactions safely.
- Risk/safety/migration: standard SQLite plaintext file format, no custom data encoding or hidden SQL delegation; source-only physical-error branches and all restricted crash/fault gates remain BLOCKED/NOT_RUN. No acceptance or production claim beyond ordinary M2 evidence.
- Next: persist verified M2 checkpoint, then M3 native recovery/WAL implementation and allowed concurrency/interop evidence; restricted gates remain manual and later milestone acceptance conditional.

### M3: Durability and concurrency
- Status: PARTIAL / BLOCKED; ordinary exclusive and shared-WAL paths implemented. M3 is not complete.
- Implemented: native WAL codec/committed-prefix scanner; optional connection-owned exclusive session; real shared-memory WAL-index dual headers, hash blocks, read marks, DMS and reader/writer/checkpoint/reconstruction locks; independent concurrent connection snapshots, deferred single writer and BUSY_SNAPSHOT; conservative append, partial/full checkpoint and safe reset. Native/driver/CLI support both explicit modes. Shared sessions never rename/unlink the WAL or SHM inode.
- ADRs: 0004-exclusive-wal-and-recovery-gates and 0006-shared-wal-protocol. Exclusive remains an intermediate/restricted alternative; the shared implementation now supplies actual single-writer/multiple-reader coordination rather than substituting an exclusive lock.
- Evidence: native valid-data page-size suites, all-four-readmark saturation, multiple hash blocks, pooled writes, ordinary rollback/reopen and live official SQLite 3.54.0 concurrent reader/writer/checkpoint/reset tests pass. Public/driver/CLI and live oracle race runs pass. Full aggregate and review evidence: docs/evidence/m3-shared-aggregate.txt and m3-shared-review.md. No failure-path execution is implied.
- Required implementation still open: broader rollback-journal recovery segments/modes and automatic safe Open recovery; incomplete/checksum-invalid current-generation WAL tail recovery; native non-Linux file locking. These remain code gaps, separately from unavailable execution gates.
- Required execution evidence blocked: physical hot rollback replay, failed/short/wrong-offset I/O, disk-full/permission faults, forced exit/power loss and interrupted checkpoint. Source-only recovery compilation is not runtime recovery verification. These gates remain BLOCKED/NOT_RUN and cannot be moved elsewhere to evade the restriction.
- Limits: Linux native runtime, writable DB/WAL/SHM required, one local lease per connection, bounded WAL growth/work, no native shrink, conservative imported-sector append/checkpoint gates unless caller explicitly asserts PSOW. Cold reconstruction accepts only committed data, valid discarded rollback suffixes or normal old-salt reset suffixes; other ambiguous tails refuse. No production acceptance claim.
- Next: preserve the reviewed shared implementation durably; implement remaining recovery/platform code under the same execution restrictions while advancing independent M4 features.

### M4: Persistent grams13 and indexed LIKE
- Status: PREREQUISITES VERIFIED / REQUIRED INDEX IMPLEMENTATION OPEN.
- Implemented prerequisites: direct lower-bound/prefix B-tree range cursor with real bounded descent/interior-key traversal, and immutable versioned Unicode rune 1/2/3-gram tokenizer. LIKE_RAW_V1 preserves SQL LIKE semantics without NFC false negatives; MATCH_CONFIG_V1 applies actual pinned NFC/optional full folding. Canonical configuration persists dependency/Unicode versions and resource limits.
- Evidence: all three official page-size fixtures, valid-memory range models and exact page-read assertions; tokenizer Unicode goldens, exhaustive scalar CGJ behavior, 4,096-word containment proof, concurrency and memory fuzz (reference91,549; LIKE71,448; config155,283 executions). Tokenizer coverage92.7%, race/vet pass. Pinned pure-Go x/text v0.43.0 checksums/license documented.
- Not implemented: persistent registry/postings/frequencies, atomic incremental maintenance, external-writer dirty guards/reindex, SQL method grammar, actual indexed LIKE plans/diagnostics and the required ≥10,000 indexed-versus-scan differential cases. Current SELECT still scans. ADR0005 is proposed design, not implementation evidence.
- Next: build registry/storage adapter, persistent posting lifecycle and truthful planner integration; retain all crash-consistency gates as conditional.

### M5–M10
- Status: NOT_STARTED.
- Deliverables/tests/oracle evidence: no fresh implementation/acceptance evidence yet.
- Limits and risks: all remaining required code and acceptance gates stay open. Independent native-OS, security, crypto/legal and long-soak acceptance cannot be fabricated.
- Every meaningful checkpoint stays remotely committed and independently backed up.
