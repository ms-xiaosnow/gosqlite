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

### M2–M10
- Status: NOT_STARTED
- Deliverables/tests/oracle evidence: none in this fresh repository.
- Limits and risks: all acceptance criteria remain open; restricted crash/fault/process-exit tests require manual evidence and may not be executed here or redirected elsewhere.
- Next: implement and verify M2 ordinary transaction/data-format acceptance; retain any restricted gates as BLOCKED.
