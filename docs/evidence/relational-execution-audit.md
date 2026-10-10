# Relational JOIN increment: ordinary execution audit

Date: 2026-10-10 UTC. Native toolchain: Go 1.27.2 linux/amd64.
Pinned test-only SQLite: 3.54.0, source ID
`be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.

## Reproducible source

The immutable final source is `../relational-sql-final-4dd07287`, copied and
hash-verified from the tested candidate `../relational-sql-freeze-v2`, made by `git archive`
of durable `69008bf658611318154a112178913c693b679a2b` plus the 27 explicit paths
in `relational-owned-paths.txt`. Its exact owned bytes are listed in
`relational-source-sha256.txt`; final manifest SHA256:
`4dd07287c3687ee466b51b7d6c343261aeb997ff81c01f75bdfe0188b85a9397`.

The shared `sqlite/sqlite.go` file was reconstructed from that durable baseline
with only the DB.Query binding line changed. `relational-query-adapter.patch`
preserves that hunk. No concurrent encrypted/native lifetime or migration
working-tree changes entered the isolated source.

Environment for every recorded run:

```
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-buildvcs=false
GOCACHE=$PWD/../go-cache GOPATH=$PWD/../go-path
SQLITE_ORACLE=$PWD/../toolchains/sqlite-build-3540000/sqlite3
GO=$PWD/../toolchains/go1.27.2/bin/go
```

Production/tests are CGO_ENABLED=0 except the explicitly separate race run.
`go list -deps` with CGO_ENABLED=1 reports no production CgoFiles for sqlite,
driver, or cmd/gosqlite. Official SQLite is used only by tagged tests/probes.

## Behavioral test-source audit

Before each executed suite, `go run ./internal/testscope` passed. The source
reference guard is supplemental; the following tests and their helpers were
read for behavior before execution:

- New `internal/sql/from_test.go`, `eval_budget_test.go`,
  `internal/btree/cursor_budget_test.go`: pure in-memory parsing, expression
  evaluation, allocation/work/cancel admission, valid dense page/freeblock
  layouts. No physical database alteration or injected native failures.
- New `sqlite/relational_test.go`, `relational_bind_budget_test.go`: ordinary
  SQL fixtures through `reviewVectorDB`, `reviewVectorExec`, `reviewVectorQuery`
  and `gramResults`; normal shared-WAL commits/read snapshots, Tx leases,
  rollback/BUSY, close/reopen/checkpoint and operation cancellation. Memory
  quota, synthetic source callbacks, BLOB/Valuer copy admission and arithmetic
  tests are descriptor-free. Overflow-record quota uses an intact inserted
  BLOB and counts ordinary page reads, never damages a page.
- New tagged `internal/testkit/relational_conformance_test.go`: ordinary SQL
  CLI/native fixtures, read-only quote-mode comparisons and integrity checks,
  normal CLI completion, ordinary external ALTER/default fixture. Helpers
  `confOracle`, `confCLI`, `confRun`, `confExpected`, `confNative`, `confCompare`,
  `confUnchanged`, `writeExec`, `writeSame`, `writeIntegrity` were inspected.
  There are 137 value/class/label comparisons (49 scenarios in each of two
  journal modes, 3 default scenarios, 24 Boolean-name/order scenarios and
  12 Boolean-alias scenarios), plus 15 matching error-category scenarios.
- New `driver/relational_test.go`: normal prepared queries/binding/types,
  transaction lifetime, early close, cancel/work/bytes quotas, shared snapshot
  behavior and explicit unsupported forward-ON forms. No crash subprocesses.
- Selected legacy internal/sql scalar/parser/expression tests: only pure
  evaluation/parsing/math and existing static test data. Selected sqlite
  expression features/review, ordinary-index storage/rowid/prefix/budget/
  transaction/concurrency/slot tests, grams/ANN precedence, vector rank/heap,
  typed HNSW all-metrics persistence tests: ordinary fixture SQL, scans/indexes,
  native math, normal rollback/close/reopen and cancellation. The typed ANN
  and IVF/HNSW fixture helpers were inspected; no failure injection was run.

No broad mixed all-test aggregate, fuzz campaign or CI job was executed.
Forbidden corruption/tamper/wrong-key/missing-file, namespace replacement,
crash/kill/process-exit/hot recovery, failed native I/O/lock/unmap/close/Sync,
disk-full/permissions or publication/rekey failure workflows were not run.
Existing manual-gated tests remain excluded and unchanged.

## Commands and evidence

Focused new suite (guard immediately before command):

```
CGO_ENABLED=0 $GO test -tags=oracle ./internal/btree ./internal/sql ./sqlite ./internal/testkit ./driver -run '^Test(Relational|DriverRelational)' -count=1
```

Final-byte result: `relational-focused-cgo0-final.txt`.
Separate final-byte race and production analysis:

```
CGO_ENABLED=1 $GO test -race ./internal/btree ./internal/sql ./sqlite ./driver -run '^Test(Relational|DriverRelational)' -count=1
CGO_ENABLED=0 $GO build ./...
CGO_ENABLED=0 $GO vet ./...
```

Results: `relational-race-build-final.txt`. Build/vet compile/analyze code; they
do not execute arbitrary tests. Legacy exact regex and result are preserved
in `relational-legacy-cgo0.txt`; it ran at manifest 9a797377, before the
subsequent guide-only gap documentation and canceled-reservation check.
`relational-build-vet.txt` additionally records toolchain/source ID and empty
production CgoFiles output. Earlier focused/race logs retain their actual hash;
they are not relabeled as final-byte runs.

The inherited eager AND/OR error divergence is explicitly documented in the
new guide, with ordinary pinned/native output in `relational-eager-boundary.txt`
and its complete small probe source in `relational-eager-probe.go.txt`.
This is an open SQL correctness gap, not an oracle pass or unsupported syntax
rejection. No assertion was weakened to hide it.

Independent review uses its own preserved source copies and evidence under
`../relational-independent-review`. The lead owns integration/publication and
will record the review's final exact manifest and outcome separately.
