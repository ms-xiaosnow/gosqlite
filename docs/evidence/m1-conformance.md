# M1 differential verification

Date: 2026-10-10 UTC. Scope: implemented read-only SQLite subset on Linux/amd64. This is an engineering conformance check, not a security/legal audit or complete SQLite certification.

## Reference and commands

`SQLITE_ORACLE` must identify the separately built official SQLite 3.54.0 CLI. Both exact version and source ID are checked before oracle-tagged tests. Missing configuration fails explicitly. Production packages never execute this oracle.

- `CGO_ENABLED=0 go test ./... -count=1`: ordinary tests.
- `CGO_ENABLED=0 go vet ./...` and `CGO_ENABLED=0 go build -trimpath ./...`.
- `CGO_ENABLED=1 go test -race ./... -count=1`: separate race-tooling check.
- `SQLITE_ORACLE=<verified path> CGO_ENABLED=0 go test -tags oracle ./... -count=1`: differential acceptance.
- `SQLITE_ORACLE=<verified path> CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -count=1`: differential/concurrency check with race instrumentation.

Final outputs are retained in m1-aggregate-final.txt. Cross-build outputs are in m1-aggregate.txt; memory-only fuzz outputs are in m1-memory-fuzz.txt.

## Coverage

The expression suite compares5,160 cases with actual official results, including scalar/LIKE/affinity cases and random binary64 formatting boundaries. The separate end-to-end suite has 9 top-level tests and272 test/subtest events, including 80 seeded predicates and300 scalar expressions. Counts are per suite, not claims of disjoint inputs or exhaustive coverage.

Committed fixtures cover page sizes 512/4096/65536,1,028 rows per samples table,200,000-byte TEXT and180,000-byte BLOB overflow, all signed-integer serial widths, rowid extremes, Unicode, NULL/empty values, and schema overflow. Official integrity_check is ok for those valid fixtures. Whole source-file SHA-256 values remain unchanged after reads.

Queries cover schema reads, projections and exact labels, rowid/IPK, affinity, literals/binds, WHERE/LIKE/ESCAPE, ORDER BY alias/source precedence, LIMIT/OFFSET, quoted identifiers, defaults after ALTER ADD COLUMN, empty results, error occurrence, cancellation, sort quotas, retained BLOB ownership, descriptor closure and concurrent prepared queries.

The quote-mode oracle transport preserves INTEGER/REAL/BLOB/TEXT distinctions and duplicate column labels. Embedded-NUL text is checked through hex/length/LIKE because the CLI quote transport truncates it. Empty-result metadata is asserted explicitly because the CLI emits no header then. No normalization conceals a discrepancy.

## Regressions found and corrected

The retained tests caught incorrect declared-name/rowid labels, nested ORDER BY resolution, TEXT/BLOB abs result types, a rejected-by-SQLite negative hexadecimal boundary, FIRST alias parsing, and scientific-notation coercion for bitwise operations. Additional numeric tests fixed float-to-TEXT rounding/subnormal formatting against the pinned baseline. Assertions follow observed official behavior.

One initial integrated run failed on `'1e2' | 0` (100 instead of1). The conversion was corrected, regression cases retained, and final full runs passed. These final successes do not erase the failed discovery run.

## Assurance limits

All read-only gates for the documented M1 subset passed. No write/durability, crash/fault/process-exit, full integrity checker, indexed execution, WAL, encryption or native non-Linux runtime claim follows. No deliberately corrupted database files or intentional subprocess termination tests were run. Ordinary CLI invocations use a protective timeout that was not intentionally triggered.

The four short fuzz sessions exercise only in-memory varint, record and header parsing. They are bounded smoke coverage, not a long-duration fuzz qualification. Parser fuzz separately completed 48,614 executions without a panic; its generated inputs never mutated a database.
