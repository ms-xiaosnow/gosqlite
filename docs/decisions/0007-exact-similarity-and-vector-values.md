# ADR0007: exact similarity candidates and independent dense values

Date:2026-10-10. Status: accepted for implemented ordinary paths; release acceptance conditional.

## Decision

M5 exposes SQL scalar scores and an explicit native DB/Tx.SimilaritySearch rather than claiming a SQL optimization that is absent. Native search names one persisted grams13 index/field, holds one snapshot, uses its canonical MATCH configuration and recomputes exact scores after fetching source rows.

A positive Dice/Jaccard score with a nonempty query set implies at least one shared gram. Therefore only a COMPLETE union of ALL selected query grams is a sufficient posting candidate superset. LIKE's rare necessary-gram subset is unsound for similarity and is not reused. Threshold0 and empty query sets scan. Work limits discard an unfinished union and fall back to full scan or explicit error; no partial union may become an exact result. Ranking is score DESC,signed rowid ASC; filters precede ranking. Configuration/generation, chosen path, fallback reason and actual work are observable.

SQL scalars have documented default NFC/case-sensitive semantics and do not inherit whichever index happens to exist. Native queries inherit only their explicit index's snapshot configuration. No pg_bigm compatibility label is attached to unpadded bigram Dice. Rune Levenshtein has bounded DP and maximum+1 exceedance sentinel. Context and cumulative work budgets reach actual execution. RRF preserves supplied rank/weight contributions and claims exactness only over the supplied lists.

M6 begins with immutable validated dense float32 values, an independent versioned GSV BLOB codec, four exact metrics and bounded pure-Go Top-K. Metrics use float64 compensated sums, explicitly differing from pgvector float32/FMA extremes; zero-norm cosine errors. Public pinned fixtures and independent mathematical references support these semantics, without claiming PostgreSQL execution. Database VECTOR(n) enforcement, SQL KNN and vector/text fusion remain subsequent required code, not acceptance-only gaps.

## Consequences

- Explicit APIs make index use and recall guarantees inspectable without pretending the limited SQL optimizer supports arbitrary rewrites.
- Complete posting unions can be costly; exact fallback/errors are preferable to false-negative truncation.
- Native result errors are atomic; ordinary streaming SQL consumers must still check Rows.Err().
- Legal SQLite shadow tables are reused unchanged; this checkpoint creates no new similarity on-disk schema.
- Vector value format is independently versioned; PostgreSQL binary COPY and SEE compatibility are not implied.
- Ordinary tests, scoped engineering review and memory fuzz support only their stated paths. Restricted physical recovery/crash/fault, independent security/legal/native-OS and soak gates remain open. Future code must preserve these distinctions.
