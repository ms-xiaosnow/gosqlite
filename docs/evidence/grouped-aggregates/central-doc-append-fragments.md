# Integration append fragments (apply to current central documents)

These are additions for the lead to reconcile against current documents, not
whole-file replacements from the older isolated baseline.

## docs/progress.md

### Native bounded GROUP BY/HAVING increment
- Status: PARTIAL; implemented in an isolated reviewed-base snapshot. Independent
  review and combined lead acceptance are separate gates.
- Added grouped keys, per-group aggregate/representative/error state, HAVING
  binding/promotion/post-finalization filtering, aliases/ordinals, full key and
  general result ordering, and LIMIT/OFFSET in native Go.
- Shared root snapshot/Tx lease, source accounting, work/memory/cancellation and
  atomic grouped result admission are preserved. No on-disk or existing exported
  sqlite/driver struct layout changed. No production CGO/oracle/dependency added.
- See docs/grouped-aggregates.md and docs/evidence/grouped-aggregates for exact
  profile, immutable-source command evidence, corrected probes and limits.
- Unimplemented: ambiguous bare grouped columns/wildcards, DISTINCT/window/FILTER/
  rollup, subqueries/CTEs, grouping/index pushdown or spill. Broad SQL and all
  restricted physical/durability/platform qualification remain open.

## docs/compatibility.md / docs/aggregate-sql.md / docs/relational-sql.md

The subsequent bounded GROUP BY/HAVING slice supersedes earlier statements that
all grouping/HAVING is unsupported. Existing global aggregates remain on their
reviewed path when no GROUP/HAVING is present. Supported keys/output expressions,
HAVING transformations, representative value classes, ordering/bounds and
resource/error contracts are specified in docs/grouped-aggregates.md. This is a
partial profile, not broad SQLite SQL or M4 completion.

## docs/upstream.md / THIRD_PARTY_NOTICES.md

Grouped aggregate behavior additionally references pinned public-domain SQLite
select.c (group output, updateAccumulator, havingToWhere, sqlite3CopySortOrder),
resolve.c (group/HAVING names), expr.c (constant-or-group and aggregate discovery)
and func.c (MIN/MAX representative selection). New Go code is independently
implemented; no C source, external SQL executor or new dependency is introduced.

V2 correction addendum for the grouped entry: finite grouped ORDER/LIMIT now
uses stable LIMIT+OFFSET top-K admission before non-key scalar projection;
resource failures remain immediate. Proved single-table physical-rowid grouping
uses required next-qualified-row lookahead and native forward/reverse cursors,
reported as grouped_rowid. Composite keys containing rowid are supported, while
mismatched ORDER shapes and nonnatural NULL ordering retain result sorting.
These changes are confined to grouping; older general-sort timing is not claimed
fixed. Independent v1 findings and corrective positive/negative controls are
preserved. Secondary-index grouping/spill and broader SQL completion remain open.
