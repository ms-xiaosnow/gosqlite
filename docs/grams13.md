# Persistent grams13 text indices

Experimental native implementation. Ordinary correctness is tested; restricted crash/fault recovery, independent audits and production acceptance remain open. This is not SQLite FTS5 or pg_bigm binary/SQL compatibility.

## Executable interfaces

```sql
CREATE TABLE docs(id INTEGER PRIMARY KEY, title TEXT, body TEXT);
CREATE INDEX docs_grams ON docs(title,body) USING grams13;
SELECT id FROM docs WHERE title LIKE '%库%';
SELECT id FROM docs WHERE title LIKE '%数据%';
SELECT id FROM docs WHERE title LIKE '%数据库%';
EXPLAIN QUERY PLAN SELECT id FROM docs WHERE title LIKE '%数据库%';
SELECT id FROM docs NOT INDEXED WHERE title LIKE '%数据库%';
SELECT id FROM docs WHERE grams13_match('docs_grams',title,'数据库');
REINDEX docs_grams;
```

USING grams13, its WITH options and grams13_match are project extension syntax/functions. Standard CREATE INDEX remains ordinary SQLite. The extended CREATE statement is never stored in sqlite_schema. There is no virtual-table/JOIN/FTS query language claim.

WITH accepts min_gram=1, max_gram=3, and exact text policies case_fold ('none' or 'unicode-full-default'), whitespace ('preserve' or 'unicode-space-collapse'), punctuation ('preserve' or 'unicode-punct-remove'). Other values fail rather than being guessed. Example:

```sql
CREATE INDEX docs_folded ON docs(title) USING grams13
  WITH (case_fold='unicode-full-default', punctuation='unicode-punct-remove');
```

Native DB and Tx provide CreateGramsIndex(ctx,name,table,columns,...GramsIndexOptions), Reindex(ctx,name,...GramsIndexOptions), and CheckExtensionIndex(ctx,name). Passing MatchConfig to Reindex atomically changes configuration. CheckExtensionIndex reconciles source values, expected postings, physical covering-index/table correspondence and exact frequencies; it is a bounded read-only check, not repair or general SQLite integrity_check.

Options.IndexMethods accepts an immutable registry snapshot. Nil installs grams13. CreateMethodIndex invokes genuinely registered Configure/Keys implementations with canonical versioned metadata; the shared transactional storage framework persists their inverted keys. Trusted custom Go handlers must obey the immutable/context/budget contract. This registry does not load code and does not yet provide ANN storage or arbitrary planner hooks.

The database/sql adapter executes the same statements and transactions. The CLI supports query, EXPLAIN through query, and check [--wal-exclusive|--wal-shared] <db> <index>, which explicitly reports extension-only consistency scope. See examples/grams for a compiled example.

## Two distinct namespaces

LIKE_RAW_V1 uses exactly the SQL evaluator's conversion, validates the entire UTF-8 value, truncates at the first NUL, folds ASCII A–Z, and stores distinct contiguous rune1/2/3-grams. No NFC, punctuation deletion, whitespace collapsing or non-ASCII folding is applied. Normalizing LIKE text would create false negatives for substrings inside decomposed characters.

MATCH_CONFIG_V1 defaults to case-sensitive NFC. Optional punctuation removal and whitespace collapse precede NFC; optional full Unicode folding is followed by NFC again. It preserves NUL. grams13_match reads the named index's persisted configuration in the SELECT snapshot and performs exact normalized substring containment. An empty normalized needle matches every non-NULL value, so candidate filtering is disabled. NULL arguments yield NULL. It has no positions, phrase operators or FTS syntax. The function is currently supported in SELECT, not DML expressions.

Characters mean Unicode code points, not grapheme clusters. Pinned x/text and actual Unicode versions, all policies, acceptance budgets, and method/storage versions form canonical metadata identity. Inputs whose normalization would silently insert a stream-safe CGJ are explicitly rejected; existing CGJ is preserved. Invalid UTF-8 in an indexed field is an extension constraint error, with statement rollback. The default tokenizer accepts1MiB input/output,262,144 runes/distinct tokens and2MiB distinct-token bytes per namespace. These are rejection limits, never truncation.

## Persistent and transactional design

Legal ordinary __gosqlite_ext_* tables store catalog, fields, posting rows and document frequencies, with real BINARY covering indices. A posting's own INTEGER PRIMARY KEY is different from the source content_rowid. Every changed posting, frequency, source row and metadata row uses the same pager.Statement and outer transaction. Updates preserve unchanged posting identities and apply only old/new deltas. Rowid changes remove old identities and add new ones. Cancelled/quota-failed rebuilds restore the prior statement state without erasing earlier transaction work.

Native writes refuse direct shadow mutation. Prefixes are not ownership evidence: exact schema, method version, canonical metadata and generated trigger definitions are checked. Three legal AFTER INSERT/UPDATE/DELETE guards make ordinary official SQLite source writes set needs_reindex in their own transaction. Source SQL fingerprints and persisted schema cookies also detect schema changes, including ordinary VACUUM that can renumber implicit rowids without DML triggers. Native schema changes refresh only already-clean or explicitly rebuilt indices, never dirty peers.

Dirty/missing-guard/schema/config-hash mismatch yields an explained SCAN and prevents native source maintenance until REINDEX. A missing exact guard can be recreated. An unexpectedly changed guard must be removed explicitly before repair; unknown general trigger code is never executed by this engine. Unknown storage/method/tokenizer versions, invalid canonical configuration and unrecognized shadow schemas fail closed. Direct external edits to shadow data or malicious restoration of matching metadata are outside cooperative-writer guarantees; the integrity checker is available for reconciliation.

## Actual plans and work

SEARCH descends the covering posting B-tree to necessary literal grams, intersects/unions bounded sorted content rowids, seeks only those source rows, then rechecks the original full WHERE. One/two-rune literals use corresponding grams; longer literals use trigrams. Frequency probes choose selective grams. AND can use any safe child; OR requires a safe plan for every branch. NOT, dynamic/literal-free patterns, unindexed expressions, high-frequency grams and exhausted candidate budgets truthfully fall back to SCAN. NOT INDEXED explicitly forces the reference path.

Rows.Plan returns access, index/method/version, field/namespace/gram lengths, estimated candidates, recheck requirement and fallback reason. Rows.Stats counts frequency lookups, actual posting records, candidates, base-row fetches and returned rows. EXPLAIN returns id,parent,notused,detail without reading postings or source rows; frequency probes are allowed. ORDER BY remains bounded materialization; no unbounded plaintext spill occurs.

Planning is bounded to128 frequency probes,64 distinct selected literal grams, at most4 postings per simple predicate and at most100,000 candidate/posting records within MaxSortBytes. Index construction also obeys MaxTransactionPages; a large all-at-once rebuild can legitimately return a quota error. No unlimited build or benchmark-scale performance claim is made.

## Evidence and limits

The native suite compares100 seeded patterns against128 Unicode texts, totaling12,800 query/text memberships, using real SEARCH versus NOT INDEXED result equality. Additional escaped-LIKE proof tests cover12,000 in-memory trials. Dedicated tests prove one/two/three-rune selective seeks and zero content/posting execution by EXPLAIN, plus NULL, empty, NUL, BLOB/numeric coercion, Unicode/ESCAPE, boolean plans, configured MATCH and normal reopen.

Ordinary official SQLite tests cover512/4096/65536-byte pages in rollback and shared-WAL modes, external writes, dirty fallback, rebuild, native deltas/rowid changes, transaction rollback, checkpoint, missing guards, VACUUM epoch invalidation and integrity_check. CheckExtensionIndex verifies reconciliation after maintenance. See evidence/m4-aggregate.txt and m4-review.md for actual final commands/results. These tests do not execute or prove crash recovery. General triggers, ALTER/DROP via native SQL, arbitrary collation, positional search and M5 similarity ranking are separate capabilities.
