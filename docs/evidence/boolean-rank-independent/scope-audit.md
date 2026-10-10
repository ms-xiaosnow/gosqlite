# Independent Boolean/ranking repair review: execution scope

The immutable baseline and candidate are read only. New probes exist solely in a
separate candidate copy; the main repository receives only these review artifacts.
The test-scope guard runs immediately before every test suite. Manual tags are
never enabled. Review does not exercise file replacement, corruption, wrong keys,
missing files, native I/O faults, native close/unmap/lock/Sync failure, subprocess
termination, hot recovery or interrupted publication/rekey.

Behavioral review before execution:

- The four new independent test groups use descriptor-free Rows/table/AST/source
  objects for error caching, cancellation, context functions and synthetic logical
  scalar quotas. Invalid vector values stay in memory. Source counters are pure
  memory, and Close always succeeds; they do not wrap a native descriptor.
- Independent SQL/public-search probes call reviewVectorDB/Exec/Query, which own
  fresh TempDir files and use ordinary Open, CREATE, INSERT, SELECT and Close.
  HNSW, typed IVF and grams indexes are ordinary public creations on valid values.
  SQL error cases concern names/arity/arithmetic or invalid query arguments only.
  Empty-source, NULL, LIMIT zero, EXPLAIN and alias checks do not mutate storage
  bytes. Existing helper implementations were inspected.
- The selected three original regressions were inspected: sparse memory tests
  use synthetic sources, malformed typed inputs or literal defaults in memory,
  quota boundaries and aliases. ANN empty/all-filtered tests use an intact
  ordinary SQL-created fixture, normal INSERT/DELETE and public search.
- New owner Boolean/rank/public/IVF test groups were read in full; they use the
  same descriptor-free and intact SQL fixtures, typed query errors and logical
  candidate budgets. No pre-existing test is changed.
- Boolean semantic, affinity and relational scope guidance was read. Existing
  selected Boolean sqlite/evaluator regressions are pure AST/evaluator inputs
  or intact normal query/write/rollback/cancel fixtures.
- Owner's pinned Boolean rank oracle was read in full. confOracle verifies exact
  CLI version/source ID; writeExec writes through native SQL. The CLI is invoked
  readonly, without CommandContext/kill/signal, takes parameters via its ordinary
  .parameter command, and completes at EOF. It compares ordinary scalar filters
  using tied native vector distances, not unsupported SQLite vector semantics.
- The independent race run selects only the four independent groups above. Race
  instrumentation is distinct from physical failure/race injection.

Full candidate test claims below are attributed to the owner and their retained
exact commands/logs. This review's own execution is explicitly selected. The
source-reference guard is a supporting aid rather than a call-graph proof.
