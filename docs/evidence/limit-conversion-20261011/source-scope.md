# Pre-execution source and behavior audit

2026-10-11. Base main HEAD 63c2ebb10e3a2c4ff2378ee9f33c46133134c70f,
tree d735409ebd1c43d90ee1650c8a7c2a084e270674. New independent conversion-only work.
Read restriction-provenance-planner-backends-20261011.md, V4c independent REVIEW,
current acceptance/interruption/closeout records, and the old V4c conversion hunks.
No AGENTS.md is present in applicable ancestor paths or the tracked repository.

Production scope: only sqlite/query.go limitValue and unused strconv import.
All six single/relational/aggregate call sites, SQL Eval/normalize/numericText/
ApplyAffinity, eval_budget.go, public wrap mapping, DB.Query, Tx.Query/queryLocked,
and Rows close/release ownership inspected. Current grouping/counters/prepared
code is not modified. Full old V4c tests and source are not transplanted.

Before the initial baseline run, both NEW regression files were fully read after
gofmt. The complete helper chain is local to those files:
- limitConversionCases: 63 fixed typed values, no random generation.
- limitConversionBudget: scalar quota-only errors/observations, no I/O hooks.
- limitConversionFixture: fresh default t.TempDir, normal plaintext Open and
  successful CREATE/INSERT of a three-row single table. No missing-file test,
  recovery, WAL, encryption, tamper, namespace interference or fault fixture.
- limitConversionShapes/Expected: fixed supported single, LEFT JOIN, global
  aggregate and grouped queries with independent expected row slicing.
- limitConversionCollect: public Next/Values/Err and normal Rows.Close.
- public controls use DB.Query and read-only Tx.Query, then ordinary Rollback
  and Close. Valid zero bounds are deliberately excluded from public cases.
- limitConversionOracle: ordinary official CLI exec.Command/Run, in-memory
  fixture, fixed SQL only; no CommandContext, timeout, signal, kill, observer,
  custom extension or external helper dependency. Errors require natural exit 1
  and explicit datatype mismatch (20), no output; success JSON stays exact
  integer-valued, ordered by named result columns. Pinned version/source ID is
  mandatory. SQLITE_ORACLE is the sole configuration, existing oracle build tag.
- No sqlite/internal-sql TestMain or test init functions exist.

Exact permitted selectors:
1. go test -timeout=0 -count=1 -v ./sqlite -run
   '^(TestLimitConversionTypes|TestLimitConversionErrorOwnership|TestLimitConversionPublicSQL)$'
2. Same with CGO_ENABLED=1 and -race.
3. go test -timeout=0 -count=1 -v -tags oracle ./sqlite -run
   '^TestLimitConversionPinnedOracle$'; same with CGO_ENABLED=1 and -race.
4. go run ./internal/testscope (entire read-only AST source guard body inspected).
5. go vet ./sqlite ./internal/sql; go build ./sqlite ./internal/sql, CGO0.

No full/global tests, fuzz, native OS/CI, restricted manual tags, physical fault,
crash/process interruption/recovery, namespace mutation, missing-selected-file,
wrong-key/tamper, rekey, backup, ANN or prior interrupted chain is selected.
Default /tmp remains unchanged. All commands use the pinned offline environment.
Baseline errors are retained, not normalized or weakened; regression source is
frozen before running and remains identical across original and corrected runs.

## Oracle harness version correction (before v2 runs)

Initial stdin CLI output omitted the numeric result code, making the initial
literal "datatype mismatch (20)" transport assumption false. Initial oracle
source/logs/inventories are retained under initial-harness; baseline source stays
in limit-conversion-baseline. These results are HARNESS_FAIL, with production
NUL failures visible separately, not 344 valid production comparisons.

The official source shell.c.in was inspected: shellLog prints the actual numeric
SQLite log code; .log stderr routes that existing callback to stderr without
opening a file or extension. A normal fixed NULL query verified `(20) statement
aborts ... datatype mismatch`. Argument SQL mode was also inspected/probed and
likewise omitted the numeric code; it is not adopted.

V2 changes exactly two helper lines: add CLI `-cmd '.log stderr'`; require stderr
start `(20) ` plus its datatype-mismatch text instead of the false adjacent-text
format. Exit 1, empty stdout, public CodeMismatch(20)/ErrMismatch/nil Rows, exact
successful rows and closed leases remain required. The observer is the standard
CLI log, with no custom C helper/build/environment dependency. Full helper chain
otherwise unchanged. V2 is run on fresh unchanged original production and fixed
production using byte-identical new tests. The two-line delta is preserved.
