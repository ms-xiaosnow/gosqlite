# Exact text similarity (M5 ordinary implementation)

This layer shares the pinned grams13 Unicode tokenizer. It does not claim pg_bigm compatibility. Same-snapshot vector/database fusion is now demonstrated by examples/vector; restricted recovery acceptance remains open, so full M5 acceptance is conditional.

## SQL

```sql
SELECT ngram_similarity('数据库查询','数据检索',2,'dice');
SELECT ngram_similarity('数据库查询','数据检索',3,'jaccard');
SELECT bigram_similarity('same','same');
SELECT levenshtein('kitten','sitting'), levenshtein('kitten','sitting',1);
SELECT id,ngram_similarity(title,?,2,'dice') AS score
FROM docs WHERE score>=? ORDER BY score DESC,id ASC LIMIT ?;
```

SQL score expressions currently scan; they do not silently use a persistent index. Add explicit secondary ordering when deterministic SQL ties matter. SQL functions use default MATCH normalization: Unicode17.0.0 NFC, case-sensitive, punctuation/whitespace/NUL preserved. Invalid UTF-8 and normalization/size limits fail explicitly. Native indexed search instead uses its named index's persisted configuration.

- Grams are distinct contiguous Unicode-code-point sequences, without padding or word boundaries. Repetition has no multiplicity. n is1,2 or3.
- Dice is 2×intersection/(left cardinality+right cardinality). Jaccard is intersection/union. Two empty sets score1, including nonempty strings shorter than n; exactly one empty set scores0.
- bigram_similarity is unpadded bigram Dice, not a pg_bigm-compatible function alias.
- Any NULL argument returns NULL. Text/BLOB/numeric operands follow the same tested text conversion as LIKE. Embedded NUL remains part of MATCH text, unlike the independent LIKE_RAW namespace.
- Metric is text dice or jaccard, ASCII-case-insensitive. Integer controls require exact nonnegative whole numbers or complete decimal numeric text, never truncated fractions or numeric prefixes. n must be1..3; Levenshtein maximum must be below the platform's maximum int.
- Levenshtein measures normalized runes, not UTF-8 bytes. A supplied maximum returns maximum+1 when the true distance exceeds it. That sentinel is not an exact larger distance. Without maximum the result is exact within quotas.
- Actual operation contexts reach scalar work in SELECT and supported DML. Errors never return an approximate score.

## Native persistent search

```go
result, err := db.SimilaritySearch(ctx, "docs_grams", "title", "数据库",
    sqlite.SimilarityOptions{
        MinGram: 2, MaxGram: 2, Metric: similarity.Dice,
        Threshold: 0.1, Limit: 10,
        Where: "category=? AND id>=?", Args: []any{"reference", 1},
    })
```

DB and Tx expose the same method. A named registered grams13 index and one physical covered field are required, even with ForceScan. One read/transaction snapshot binds source rows, metadata, configuration and generation throughout the operation. An active transaction sees its own writes. Changing normalization requires atomic Reindex; query options cannot reinterpret persisted postings.

Both gram bounds zero mean2..2; otherwise select an inclusive range within1..3. Empty Metric means Dice. Threshold is inclusive in[0,1]. Limit is explicit: zero validates but returns EMPTY without source reads; negative or over-budget values error. A NULL query returns EMPTY. Query accepts normal SQL-convertible Go values, including driver.Valuer; cancellation after coercion is checked. Where is exactly one supported SQL expression, with ordinary named/positional binding; comments are accepted, appended statements/clauses rejected. Where executes before ranking, and NULL predicate results exclude a row.

Returned Hits contain rowid and exact float64 score plus left/right/intersection cardinalities. Ordering is score DESC, signed rowid ASC. NULL fields do not match. Any error or cancellation discards the whole result; there is no partial-success flag to overlook.

## Complete candidate proof and fallback

For a nonempty query set and positive Dice/Jaccard threshold, every qualifying document has at least one query gram. SEARCH therefore reads the complete union of every selected query-gram posting, seeks those rowids and recomputes exact scores. It never selects only rare grams, caps a preliminary Top-K, stops after an early perfect score, or reuses LIKE's necessary-subset planner. Large-query tests explicitly exercise more than128 grams.

Threshold0 can include disjoint documents. An empty query set can match another empty set with score1. Both cases require SCAN. Dirty indices, explicit ForceScan, and inability to finish the entire posting union also require SCAN. A posting/candidate budget failure discards its partial union before scanning, preserving attempted work counters. If the exact scan itself exceeds a quota, the operation errors rather than silently truncating recall. Unknown metadata/method versions fail closed rather than becoming unverified candidates.

This proof relies on cooperative ordinary source/index consistency and the same tokenizer configuration. Official external DML invalidates via exact guards; schema-cookie checks detect VACUUM/schema changes. Reindex restores usable postings. Restricted physical recovery and hostile on-disk tampering are not qualified by this semantic proof.

## Plans, work and limits

SimilarityPlan embeds QueryPlan and adds CandidateMode, QueryTokens, ConfigHash, TokenizerHash, Generation, MinGram/MaxGram, Metric, Threshold, Limit and Exact. Its actual access is SEARCH, SCAN or EMPTY, with a reason. Detail describes the completed operation; it is not a separate SQL optimizer rewrite.

SimilarityStats embeds QueryStats and adds PostingRangesRead, FilteredRows, ScoredRows and MatchedRows. PostingKeysRead counts records, not B-tree pages; PostingRangesRead counts opened gram ranges. BaseRowFetches counts actual source rows, including subsequently rejected filters. MatchedRows precedes Top-K. Abandoned posting reads remain visible on a scan fallback. Candidates is populated only for a completed union.

Zero Limits selects similarity.DefaultLimits; a nonzero struct must be fully valid. Defaults:4096 edit-distance runes,2^24 DP cells,2^24 cumulative set comparisons,100,000 source candidates,10,000 result rows,64MiB cumulative input bytes,32 fusion lists. Tokenizer per-text quotas also apply. Native MaxCandidates and cumulative input bytes charge fetched/filtered rows, preventing a filter from bypassing the work bound. MaxSortBytes additionally bounds candidate/result memory. These are logical work limits, not exact allocator or wall-clock guarantees; trusted caller callbacks/Valuer implementations are outside them.

## Pure Go mathematics and RRF

extension/similarity.New accepts an immutable versioned Config; prepared sets carry its canonical hash and cannot be compared under another configuration. Compare/ComparePrepared, bounded Levenshtein, exact Scan, cardinality bounds and candidate-condition helpers have independent reference/property tests.

Engine.Fuse takes supplied best-first RankedList values with positive finite weights. It sums weight/(offset+one-based-rank), requiring positive finite offset; missing ranks contribute zero. Output sorts score DESC,rowid ASC and retains per-list name, rank, weight and contribution. Duplicate rowids within one list are errors. This is exact for the supplied lists, not evidence of upstream approximate recall. examples/similarity combines title/body text rankings; examples/vector adds same-snapshot database vector fusion with original text scores/cardinalities, vector distances and contribution details.

Evidence: evidence/m5-review.md and m5-aggregate.txt; public driver/native/example and ordinary live-SQLite invalidation tests. No pg_bigm/PostgreSQL execution or full-production acceptance is claimed.
