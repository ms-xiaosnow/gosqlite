# Ordinary B-tree SELECT access

The first ordinary-index planner uses the existing native SQLite secondary
B-trees; it does not build a transient replacement index. It supports direct
rowid/INTEGER PRIMARY KEY equality and leading equality prefixes of supported
nonunique BINARY ASC indexes. All matching candidates still evaluate the original
WHERE. This is an incremental SQL capability, not general SQLite planner parity.

## Selection and semantics

Eligible ANN retains first precedence, then grams13. Ordinary access is considered
only when the real source is still an untouched SCAN and no ANN candidate query
was retained. NOT INDEXED keeps the complete scan. Unsupported index definitions
are ignored individually for read-only planning; they do not make an otherwise
supported source table unreadable. Existing writable-catalog safety rules remain.

Column identity is resolved against the physical table and supported direct
column aliases. Equality values receive the same comparison affinity as the
original predicate. NULL/storage-class and mixed INTEGER/REAL comparisons are
preserved, including signed int64 limits and values around2^53. A rowid equality
known impossible after comparison affinity returns no candidate instead of
inventing a nearest rowid.

A secondary prefix range collects bounded matching rowids, then sorts signed
rowids before base seeks. This preserves the existing native scan's unordered
LIMIT/OFFSET selection and stable ORDER BY tie behavior even when index suffixes
have another order. There is no covering-read, reverse-range, inequality endpoint
or ORDER BY elimination claim. Non-leading equality alone does not select that
index. Every returned secondary key must identify a real base row.

The current expression evaluator eagerly evaluates AND operands. Therefore the
whole WHERE and relevant defaults must be proven error-safe before ordinary
candidate pruning: error-capable function calls, LIKE and typed-vector WHERE
interpretation conservatively retain their prior path. The planner must not hide
an error that the original scan would encounter. Projection still occurs for the
same matching rows. Existing grams/ANN fallback reasons are preserved when there
is no ordinary equality candidate.

## Bounds and diagnostics

- At most256 candidate index definitions are considered.
- At most100000 candidate rowids, further bounded by MaxSortBytes/64 and prefix
  memory, are retained.
- At most8192 actual range-page reads, further bounded by available memory divided
  by4*page_size+512, are attempted.
- Prefix/key sizes and underlying B-tree payload/depth/visited limits still apply.
- Candidate/work admission exhaustion keeps the original untouched table source;
  no partial candidate list is returned. Cancellation remains an error.

Plan.Method is rowid or btree and Access is SEARCH only for those executable
paths. B-tree plans identify the selected index and physical equality prefix,
recheck=true, covering=false and order_satisfied=false. With no cardinality
statistics, EstimatedCandidates=-1 is explicit rather than a fabricated estimate.
EXPLAIN does not execute the candidate range.

Stats.IndexKeysRead counts matching index records returned by the range;
IndexPageReads counts actual range PageReader calls, including descent probes;
RowIDSeeks counts base seek attempts. Catalog reads and other access methods are
excluded from these counters. They are logical calls, not OS physical I/O, and
attempted index work is retained after fallback. BaseRowFetches and ReturnedRows
keep their existing meanings.

## Ordinary evidence

Independent valid-file oracle tests cover512/4096/65536 pages in rollback/shared
WAL, native versus NOT INDEXED and official SQLite results, transactional updates
and rollback, external legal SQL changes, normal checkpoint/reopen, unsupported
expression/DESC peer indexes, numeric/text/BLOB/NULL comparisons and NUL/overflow
TEXT prefixes. Unit2.597s and race4.523s passed in the expanded oracle component
run; final-source regressions and aggregate are recorded separately.

A1800-row native fixture reads20 matching index keys,3 range pages and20 source
rows versus1800 forced-scan source rows. Independent PageReader instrumentation
also measures a late two-key prefix reading3 pages versus84 for a complete
1600-key index traversal. These are scoped logical-work observations, not latency
or universal optimization claims. No physical crash/fault/recovery gate is covered.
