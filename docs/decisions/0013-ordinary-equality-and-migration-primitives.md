# ADR 0013: Bounded equality access and migration prerequisites

Status: implemented scoped increment; broader SQL and M9 remain incomplete.

## Ordinary access

Reuse persistent native SQLite B-trees. Prefer existing ANN/grams paths, then
prove a physical rowid or nonunique BINARY ASC leading equality prefix. Apply
comparison affinity exactly, read a bounded prefix range, sort candidate signed
rowids and seek source rows before the unchanged residual predicate. Keeping
source order preserves current LIMIT/stable-sort semantics. Error-capable WHERE
expressions retain scans because current AND evaluation is eager. NOT INDEXED
is the exact reference; unsupported read-only peer index definitions do not hide
a usable supported index. No range/order/covering/general planner claim.

Logical page-reader calls, matching keys and source seeks are separate counters.
Attempted work survives budget fallback. No statistics means no fabricated
candidate estimate. See ordinary-index.md for bounds and official comparisons.

## Migration identity

Begin with a bounded local, exact-schema catalog and real-array inspector.
Names alone cannot identify extension semantics: bind membership, method handler,
opclass, operator strategy, support function and types. Reports distinguish
supplied profiles from observed facts, and canonical current-content hashes from
unchanged original-byte provenance. Unsupported/review-required objects cannot
emit an apparently usable target. Legacy HNSW defaults are not silently retuned.

This checkpoint neither executes remote SQL nor substitutes string replacement
for the required future typed converter. Local import/resume/verification and
PostgreSQL transition templates remain separate required implementations. Exact
upstream notices and public references are retained; independent legal review
and live PostgreSQL qualification have not occurred.

## Extended values

Introduce immutable, representation-specific half/bit/sparse Go values and
strict distinct version-1 codecs while preserving dense GSVv1. Explicit loss
reports prevent silent half quantization. Sparse work scales with nonzeros;
bit packing/padding and all binary16 rounding boundaries have direct tests.
Metrics are representation-typed and use bounded cancellable finite binary64
arithmetic. Published upstream differences are disclosed rather than weakened
into passing tolerances.

These primitives and Scanner/Valuer adapters do not enable extended SQL storage,
operators, typed exact ranking or ANN. Those integrations must persist matching
identities and preserve existing dense data in subsequent checkpoints. Neither
memory tests nor ordinary database lifecycle tests establish physical recovery,
security or full M9 acceptance.
