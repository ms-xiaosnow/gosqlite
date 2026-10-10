# M6 independent scoped engineering review

Date: 2026-10-10 (UTC). Toolchain: repository-local Go 1.27.2.

This review covers the dense-vector primitive package, typed SQL interpretation
and writes, lexer/parser/evaluation changes, native exact search, SQL bounded
Top-K ranking, and Rows integration. It checks the implemented subset of
`goal.md` sections 7 and 10 (M6). It is an engineering code/test review, not an
independent M10 security/legal audit, crash-durability certification, full
SQLite/PostgreSQL compatibility claim, or production-readiness approval.

## Findings and resolution

- A typed-interpretation inconsistency was reported to the implementation owner:
  SQL heap scoring returned NULL immediately for a NULL query operand, bypassing
  validation of a non-NULL VECTOR(n) candidate. The general SQL sorter resolves
  that typed column first and rejects invalid text, an invalid BLOB, a wrong
  declared dimension, or an unsupported storage class. The owner moved candidate
  decoding before the query-NULL return. The independent memory-source regression
  now confirms identical error behavior on both paths. A NULL candidate still
  suppresses decoding a malformed query, as ordinary scalar NULL propagation
  requires. No malformed database file was used.
- Rows close needed to release the new rank cache in addition to its existing
  parameter/result storage. The owner had independently added the cleanup while
  the reviewer raised this concern. The regression verifies that bound query
  bytes and decoded rank storage are no longer retained by a closed Rows, while
  Plan and Stats remain available. This is a lifecycle regression, not a claim
  that a measured process-memory leak was reproduced.
- The owner also corrected typed vector input resource errors so ErrLimit remains
  a resource error rather than being classified as a type mismatch. That fix
  and its SQL tests were included in the final focused ordinary/race runs.

No production files were edited by this reviewer. No unresolved substantive
correctness finding remains in the reviewed scope.

## Independent tests added

`sqlite/vector_review_test.go` contains six review tests:

- `TestVectorReviewHeapAndNativeIndependentReference`: 216 parameterized SQL
  bounded-heap queries are compared with a deliberately unoptimized secondary-
  key full sorter and a separate arithmetic/full-sort reference. Another 216
  native exact queries are checked against that independent reference with NULL
  candidates removed. The fixed-seed dataset has 76 rows, four dimensions,
  negative/positive coordinates, repeated vectors, NULL values, filtering and
  minimum/maximum signed rowids. It covers all four dense metrics, three query
  vectors, literal operator/function shapes with bound query values, reversed
  operands, result aliases, six LIMIT/OFFSET windows, windows beyond the source,
  and truthful source/rank/result diagnostics. Quarter-integer components make
  the reference's products and sums exact in float64; final sqrt/cosine results
  use relative tolerance 1e-13. Neither the reference metric nor its full sorter
  calls a production distance or ranking implementation.
- `TestVectorReviewTypedNullOperandParity`: entirely in-memory candidate rows
  compare heap and scalar/full-sort typed validation for wrong dimensions,
  invalid text, invalid textual BLOBs, unsupported scalar storage, valid text,
  valid binary vectors, and each NULL-operand arrangement.
- `TestVectorReviewSQLHeapBudgetsCountNullAndFilteredRows`: memory-source tests
  verify that NULL and business-filtered candidates still consume the source
  quota; a later candidate or cumulative component-budget failure produces no
  materialized partial result.
- `TestVectorReviewHeapCancellationReleasesSnapshot`: cancellation during a
  ranked query returns no Rows result; a second connection can perform an
  ordinary update and checkpoint, and the first connection remains usable.
- `TestVectorReviewHeapSnapshotRowsRemainStable`: a ranked result materialized
  before another connection's ordinary committed update retains its old values;
  a subsequent query sees the new ordering. Normal checkpoint succeeds after
  the original Rows is consumed.
- `TestVectorReviewCloseReleasesRankStorage`: explicit close releases the new
  query/parsed rank cache plus parameters and retained rows, without discarding
  stable plan/statistics diagnostics.

The independent reference test exercises both SQL and native interfaces. No test changes an existing assertion or skips a
failure. Ordinary fixture writes use the native transactional engine.

## Code-review conclusions

- The SQL fast path retains only LIMIT+OFFSET ranked rows, using a worst-first
  heap and draining it in ascending order. It does not stop at a candidate
  prefix. Its result order is finite distance ascending, signed rowid ascending,
  with SQL NULL first. Native exact search excludes NULL candidates. The two
  policies are deliberately different and both are tested.
- A heap is a ranking stage, not an ANN index. SQL source access remains SCAN or
  a separately proven business-predicate index path, with original WHERE
  re-evaluation. Native VectorSearch reports an exact full scan. HNSW/IVFFlat,
  their persistence, metric operator classes, and ANN recall are not supplied.
- Only a single ascending distance key over one VECTOR(n) column and a literal
  or bound operand is optimized. DESC, secondary keys, unlimited ordering and
  unsupported expression shapes remain on the general SQL path. Aliases and
  result ordinals are bound before recognition. LIMIT+OFFSET is checked before
  conversion to heap capacity; LIMIT zero does not score source rows.
- Eligible zero-norm cosine candidates fail the whole operation rather than
  disappearing from recall. Business predicates run before distance scoring.
  Native queries are validated even when their limit is zero; SQL preserves
  scalar laziness for an empty/filtered source or NULL candidate. This is a
  documented interface distinction, not evidence of identical lazy evaluation.
- Dense values are immutable finite float32 vectors with explicit dimensional
  bounds. Text and binary input are separate; a BLOB is never guessed to be
  textual vector data. The versioned binary header, dimension, exact length,
  flags, endianness and component finiteness are checked before exposing a
  vector. Signed zero and subnormal storage remain bit-preserving.
- Metrics widen before arithmetic and use float64 compensated sums. The
  documented float32 range/dimension bound keeps supported distances finite,
  aside from explicitly rejected zero-norm cosine. This intentionally differs
  from upstream float32/FMA rounding and overflow; semantic compatibility is
  not bitwise pgvector arithmetic equivalence.
- Source candidates and ranking component work are bounded. SQL retained-row
  costs include source/projection/key storage; native duplicate tracking and
  result storage have separate conservative logical charges. These limits do
  not promise an exact process-RSS bound or charge arbitrary scalar/filter work
  as vector scoring. Errors/cancellation do not return falsely exact partial
  native results or partially materialized ranked SQL Rows.
- Native exact search and SQL Rows use the existing reader/transaction boundary;
  there is no separate vector sidecar or nontransactional index to publish.
  Writes validate VECTOR(n) before canonical BLOB storage. Typed reads preserve
  ordinary legal existing text/BLOB storage classes while validating values
  that are actually interpreted. Unused/filtered physical vector values need
  not be decoded merely because another column is read.

## Commands actually run

From the repository root:

```sh
export GOTOOLCHAIN=local
export GOCACHE="$PWD/../go-cache"
export GOPATH="$PWD/../go-path"
GO=../toolchains/go1.27.2/bin/go
CGO_ENABLED=0 "$GO" test ./sqlite -run '^TestVectorReview' -count=1
CGO_ENABLED=0 "$GO" test ./sqlite -run '^TestVector' -count=1
CGO_ENABLED=0 "$GO" test ./extension/vector ./internal/sql -run 'Vector' -count=1
CGO_ENABLED=1 "$GO" test -race ./sqlite ./extension/vector ./internal/sql -run 'Vector' -count=1
CGO_ENABLED=0 "$GO" test ./extension/vector -run '^$' \
  -fuzz '^FuzzVectorTopK$' -fuzztime=10s -parallel=2
git diff --check
```

All passed. Final ordinary package timings: sqlite 0.749s, extension/vector
0.151s, internal/sql 0.009s. Race timings: sqlite 5.723s, extension/vector 2.080s,
internal/sql 1.060s. Race instrumentation used CGO_ENABLED=1 only for the Go race
runtime; ordinary production-path tests used CGO_ENABLED=0.

The bounded memory-only TopK fuzz run completed 24,072 executions, with 43 new
interesting inputs and no failure (10.628s). Its target creates no database and
performs no physical fault, process-exit or recovery workflow. Vector package
runs also exercise pinned published pgvector examples, independent high-
precision references, codec bounds, extrema, adapters and immutable shared use.
No PostgreSQL/pgvector server was executed by this reviewer.

## Remaining limits

- The reviewer did not execute the official-SQLite interoperability tests or the
  whole repository aggregate. Those separate runs belong to their runner and
  should not be attributed to this review.
- The finite reference matrix, source inspection, race run and short fuzz test
  are not exhaustive proofs, broad-platform evidence, long-soak measurements,
  performance benchmarks, or ANN recall measurements.
- No crash, process kill, fault injection, hot recovery, deliberately damaged
  disk or forced process-exit database workflow was run locally or remotely.
  Database tests used ordinary valid writes, cancellation, transactional commit,
  normal snapshot reads, close and checkpoint only. Invalid typed input in this
  review stayed inside a memory source.
- No commits, staging, publication, remote changes or external communications
  were performed by this reviewer. These two reviewer-owned files are ready
  for the parent aggregate and checkpoint workflow.
