# ADR 0005: Persistent grams13 design

Status: proposed design; range cursor and tokenizer prerequisites are implemented and tested. Persistent registry/postings/planner are NOT implemented by this document.

## Legal SQLite storage and ownership

Extension definitions must not be stored as unknown CREATE syntax in sqlite_schema. Use owned legal ordinary tables and ascending BINARY covering indices under __gosqlite_ext_*. Catalog rows contain logical index name/id, source table/schema hash, method/storage/tokenizer versions, canonical config/hash, actual Unicode/dependency versions, generation and needs_reindex. Field rows resolve column names/ordinals from the current schema. Posting rows contain index_id, field_id, namespace, gram_length, gram and content_rowid; their INTEGER PRIMARY KEY is a distinct posting id. The covering lookup index ends with content_rowid followed by SQLite's implicit posting rowid. Frequency rows maintain document counts in the same transaction. Current unsupported UNIQUE syntax must not be introduced into stored schemas; native maintenance enforces logical uniqueness.

Physical roots are resolved from the current sqlite_schema snapshot. Metadata does not authorize dynamic code loading or SQL execution. Unknown versions/configs fail closed. Users cannot directly mutate owned shadow data through the native SQL API.

## Two semantic namespaces

LIKE_RAW_V1 validates the entire coerced input UTF-8, then truncates at the first NUL, folds only ASCII A–Z and emits distinct contiguous one/two/three-rune grams. Punctuation, combining marks and non-ASCII case remain untouched. SQL conversion must be exactly the evaluator's LIKE conversion, not a second implementation.

MATCH_CONFIG_V1 uses actual NFC and optional full Unicode folding, with NFC reapplied afterward. Whitespace/punctuation policies, strict handling of stream-safe CGJ insertion, data versions and all resource limits are persisted. Initially MATCH means configured normalized substring containment, not FTS syntax or positional phrase search. Invalid UTF-8 indexed fields are rejected as an explicit extension-index constraint during build and DML, with statement rollback.

Proof for indexed LIKE: a successful LIKE match contains each escaped literal run contiguously under the same ASCII fold and NUL semantics. Each selected gram from such a run is therefore necessary. Any chosen necessary gram or intersection yields a superset, and the original WHERE/LIKE removes false positives. NFC-only filtering is unsound: decomposed e+mark contains e but normalizes to é. The separate raw namespace avoids this false negative.

## External writers invalidate

Generated legal AFTER INSERT/UPDATE/DELETE source-table triggers set the corresponding catalog needs_reindex=1. Official SQLite executes these guards; native writes maintain postings directly and recognize only exact owned generated guard SQL, source table, name and rootpage0. A prefix alone never proves ownership. General triggers remain unsupported. Guard roots are not B-tree ownership roots.

Dirty, missing/changed guard or schema-fingerprint mismatch disables index plans and yields an explicit scan fallback. Native source writes require successful REINDEX first. REINDEX needs a narrow repair path for exact owned guards without executing unexpected trigger code; unrelated triggers remain a blocker. Header change counters alone are not a safe external WAL invalidation mechanism.

## Real index ranges and plans

The B-tree range cursor descends directly to a partial-key lower bound, includes interior index records and stops at the prefix boundary. It must not scan and discard earlier keys. Same-cell overflow rereads need ownership-aware handling without permitting cross-cell shared chains.

Candidate lookup uses covering posting index ranges and content_rowid directly, then btree.SeekTable for only candidate source rows. No posting-table/source-table full scan can be labeled SEARCH. One-rune literals use unigrams, two use bigrams, longer runs use trigrams. Select persisted low-frequency grams under a bounded query budget. Common grams may correctly choose SCAN.

AND may use any safe indexed child candidate then recheck the full WHERE. OR needs safe candidates for every branch or falls back to SCAN. NOT LIKE, dynamic patterns, unsupported collations and literal-free patterns scan. Sorted posting lists support bounded intersection/union/deduplication. LIMIT/OFFSET apply after recheck; existing ORDER BY remains bounded.

EXPLAIN QUERY PLAN and Rows.Plan/Stats should expose SCAN/SEARCH, logical index/method/version, field/namespace/gram lengths, estimate/fallback reason, original-LIKE recheck and actual posting keys, candidates, base-row fetches and returned rows. Selective tests must prove fewer base-row reads, not merely inspect a label.

## Registry and transaction boundaries

Registry is immutable per connection, with registered method/version handlers; no mutable package-global registry and no metadata-directed downloads. Storage adapters expose bounded typed table/key operations sharing the existing Tx.reader/Tx.storage, never independent commits or filesystem paths.

Build/DML/REINDEX run inside pager.Statement. INSERT/UPDATE/DELETE compute bounded old/new token sets and apply only deltas plus frequencies; rowid changes remove old/add new. Missing/duplicate logical postings are errors. Rebuild is one cancellable statement, clears dirty state only on success and preserves the previous index/earlier transaction statements on failure. Do not mutate via a retained cursor while deleting the same tree.

Proposed actual extension grammar: CREATE INDEX name ON table(columns) USING grams13 WITH (...), REINDEX name, EXPLAIN QUERY PLAN SELECT. Standard CREATE INDEX remains unchanged. Configured matching can initially use an explicitly documented grams13_match function/native Match API rather than pretending virtual-table JOIN/FTS semantics exist.

## Ordinary evidence required

At least10,000 deterministic randomized indexed-versus-forced-scan query/text combinations, plus decomposed/combining/Hangul/kana/Han/emoji/ZWJ, cases, whitespace/punctuation, escaped wildcards/Unicode ESCAPE, NUL/NULL/empty/repeated grams. Real one/two/three-rune SEARCH plans and fewer source fetches. Incremental DML/rowid changes/read-own-writes/statement and transaction rollback/normal reopen/checkpoint. REINDEX/config changes/cancellation/quota rollback and exact posting/frequency reconciliation. Official SQLite integrity and ordinary source writes must trigger safe invalidation, scan fallback and native rebuild. Crash/recovery consistency stays BLOCKED/NOT_RUN until required M3 gates pass.

Primary references: [SQLite LIKE](https://www.sqlite.org/lang_expr.html#the_like_glob_regexp_match_and_extract_operators), [stored schema](https://www.sqlite.org/fileformat2.html#storage_of_the_sql_database_schema), [trigger grammar](https://www.sqlite.org/lang_createtrigger.html), pinned x/text sources/checksums in upstream.md.
