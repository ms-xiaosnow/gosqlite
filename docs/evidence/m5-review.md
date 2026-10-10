# M5 independent scoped review

Date: 2026-10-10 (UTC). Toolchain: repository-local Go 1.27.2.

This review covers `extension/similarity`, `internal/sql/similarity.go`,
`sqlite/similarity_search.go`, their new tests, and the SQL evaluation-context
changes. It checks the text-similarity portion of `goal.md` section 6 and M5.
It is a scoped code/test review, not a security audit, durability certification,
production-readiness claim, or evidence for later vector/encryption milestones.

## Finding and resolution

One reproducible defect was reported to the implementation owner and fixed:

- `SimilarityOptions.Where` accepted a standalone expression with a trailing
  `--` comment during its first parse, but the synthetic SELECT appended `)`
  on the same line. The closing parenthesis became comment text, so native
  search rejected valid filters such as `active=1 -- trailing comment` and
  `active=? -- trailing parameter comment`. Ordinary SELECT accepted them.
- The owner changed the wrapper to place a newline before its closing
  parenthesis. Both regressions now pass for indexed and forced-scan paths,
  including positional binding.

A suspected context omission in schema-default validation was investigated and
withdrawn: the parser rejects function defaults before the context-free schema
evaluation is reachable. A regression preserves that current unsupported scope.
No production code was edited by this reviewer.

## Independent tests added

`sqlite/similarity_review_test.go` adds:

- `TestSimilarityReviewIndependentRecall`: 504 searches against an independent
  rune-window/map/cardinality/full-sort reference. Covers all six contiguous
  gram ranges within 1..3, Dice and Jaccard, zero and smallest-positive
  thresholds, inclusive boundary and nextafter boundary, all-score and empty
  matches, SQL NULL exclusion, business filtering, Unicode and embedded NUL,
  and minimum/maximum signed rowids. The reference shares neither production
  gram tokenization nor the ranking heap. Input text is already NFC. It checks
  each result's complete score cardinalities and Top-K ordering, actual
  SEARCH-versus-SCAN choice, and every selected query-gram range being visited
  when SEARCH is used.
- `TestSimilarityReviewWhereMatchesSQL`: 11 filter forms compared with ordinary
  SQL on both paths, including truth conversion, TRUE/FALSE, NULL, TEXT affinity,
  numbered and repeated named parameters, rowid aliases, and trailing comments.
- `TestSimilarityReviewRuntimeFailuresDiscardHits`: an error in a later row's
  filter and an aggregate comparison quota exhausted after an earlier hit both
  return a zero result, with no retained hits or misleading plan/statistics.
  Also checks cancellation inside a substantial native-filter scalar and that
  the connection remains usable afterward.
- `TestSimilarityReviewScalarDefaultVersusPersistedConfiguration`: a persisted
  punctuation-removing/case-folding index matches both `ab` and `A-B`, while
  the scalar keeps its documented default case-sensitive, punctuation-preserving
  semantics. Indexed and forced-scan results agree using the persisted config.
- `TestSimilarityReviewRejectsFunctionDefaults`: the currently unsupported
  function-default path is rejected before schema-default evaluation.

## Code-review conclusions

- The positive-threshold, nonempty-query fast path uses the union of every
  selected query gram. A positive Dice/Jaccard score implies at least one shared
  gram, so that complete union is a valid superset. Source rows are fetched and
  exact scores recomputed before ranking. There is no selectivity probe or
  Top-K candidate-prefix shortcut in this path.
- Threshold zero and empty selected query sets require SCAN. Unusable/dirty
  indexes choose SCAN; an unfinished union caused by posting/candidate work
  limits is discarded and followed by SCAN or an explicit quota error. The
  abandoned posting work remains in statistics, and incomplete candidates are
  never advertised as complete.
- The source snapshot supplies the persisted MATCH normalization configuration;
  ordinary scalar calls intentionally use their separate documented default.
  Plan identity contains tokenizer/config hashes and index generation. Ordinary
  SQL scalar predicates remain truthfully reported as SCAN rather than claiming
  an unimplemented SQL optimizer rewrite.
- Native search counts fetched rows and field text even when the business
  predicate rejects them. The mathematical scan's merge-comparison quota is
  cumulative across scored rows. Errors and cancellation return no partial
  native or mathematical scan result. SQL streaming consumers must still check
  `Rows.Err()`; no claim of atomic streaming SELECT delivery is made.
- Ranking is score descending then signed rowid ascending; NULL fields are
  excluded. Prepared token sets expose copies and carry configuration identity.
  Engine/config data is immutable during operations; operation state is local.
- RRF uses the supplied ordered lists, positive finite weights and offset,
  one-based rank, input-list accumulation order, explicit duplicate/overflow
  errors, bounded total rank entries, and explainable per-list contributions.
  Exact fusion of supplied lists does not claim upstream candidate recall.
- Decimal control text is parsed completely using bounded exact decimal digit
  arithmetic, without floating-point truncation of fractions. Integer/float
  controls reject out-of-range and nonintegral values; blobs are not controls.
  Metric text is length-checked before name folding. Existing tests include
  5,000 decimal cases against `math/big.Rat`, long zeros/exponents, fractional
  rounding traps, and Unicode/NULL/coercion goldens.

## Commands run by this reviewer

Commands ran from the repository directory with:

```sh
export GOTOOLCHAIN=local
export GOCACHE="$PWD/../go-cache"
export GOPATH="$PWD/../go-path"
GO=../toolchains/go1.27.2/bin/go
```

Final focused ordinary and race selections passed:

```sh
CGO_ENABLED=0 "$GO" test ./sqlite -run '^TestSimilarity' -count=1
CGO_ENABLED=1 "$GO" test -race ./sqlite -run '^TestSimilarity' -count=1
CGO_ENABLED=0 "$GO" test ./extension/similarity ./internal/sql \
  -run 'Similarity|Levenshtein|MinimumIntersection|CandidateBounds|Scan|Fusion|Example' -count=1
CGO_ENABLED=1 "$GO" test -race ./extension/similarity ./internal/sql \
  -run 'Similarity|Levenshtein|MinimumIntersection|CandidateBounds|Scan|Fusion|Example' -count=1
git diff --check
```

The sqlite selections include the owner's ordinary transaction, rollback,
reindex/configuration, reopen and checkpoint cases. The math selections include
independent set/ranking/edit-distance/RRF references, cumulative quotas,
cancellation, and immutable shared-engine concurrent use.

Bounded in-memory fuzzing also passed:

```sh
"$GO" test ./extension/similarity -run '^$' \
  -fuzz '^FuzzSimilarityMath$' -fuzztime=15s -parallel=2
```

Observed: 216,581 executions, 28 new interesting inputs, no failure. This fuzz
target opens no database and does not execute faults, recovery, or process tests.

## Remaining limits

- This reviewer did not rerun the separate official-SQLite external-invalidation
  oracle. Its source was reviewed; any execution evidence for it belongs to
  the separate runner and must not be attributed to this review.
- This is not a full-repository aggregate, cross-platform run, long soak,
  benchmark, or proof of all possible inputs. The finite oracle and short fuzz
  run supplement the recall argument; they do not replace it.
- No physical fault, crash, process-kill, hot-recovery, damaged-disk, or manual
  fault harness was run. Database tests here used ordinary valid data and normal
  transaction/close/reopen/checkpoint APIs.
- No commits, publication, or external writes were made. The reviewer-owned
  test and this evidence document are ready for the parent aggregate run.
