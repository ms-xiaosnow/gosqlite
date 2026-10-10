# Selected test behavioral audit

Every suite command is preceded by `go run ./internal/testscope`; no manual tags.
The reference guard alone is not considered a behavioral audit.

New internal/sql tests only construct ASTs, values, mock scalar functions and
memory-only quota/canceled-context objects. Existing internal/sql tests and
oracle differential helpers were reviewed: parser/codec/math/affinity tests are
in memory; `runDiff` launches only the pinned CLI on `:memory:` and waits normally.
No process-termination/corruption controls are used.

New sqlite tests use reviewVectorDB/reviewVectorExec (owned TempDir, normal Open,
ordinary SQL and Close), direct descriptor-free typed-cell evaluator inputs,
normal read cancellation and rollback. New driver test owns one ordinary temp
file through NewConnector/database/sql, prepared parameters, rollback and a
canceled query context. Existing selected Relational/OrdinaryIndex/Expression/
SQLExpression tests use the same intact normal fixtures. The ordinary-index
internal PageReader/cursor budgets use valid in-memory encoded pages and canceled
read contexts; no physical descriptor fault is injected.

The new tagged Boolean oracle test uses confOracle's exact version/source-ID
check; normal SQL creates all fixtures, then the CLI uses readonly quote-mode
reads and temporary parameter bindings. Error comparisons use ordinary SQL
expression errors only. Helpers confQuoted/writeExec are parsing and normal
transaction SQL, respectively. No CLI interrupt or timeout kill is used.

Selected existing relational/driver oracle tests are normal writes, reads,
rollback/snapshot/cancel/close or unsupported syntax, using the same pinned
ordinary CLI. Existing vector-rank and grams planner checks, if run, are intact
normal SQL/index fixtures and conservative memory/cancellation budgets. Ordinary
index/row source ordering limitations are reported explicitly, not concealed
by omitting the source-order regression.
