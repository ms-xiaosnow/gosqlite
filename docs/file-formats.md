# File formats

M2 reads and writes the tested rowid-table and BINARY ASC secondary-index subset of SQLite format 3, according to the [official specification](https://www.sqlite.org/fileformat2.html), with tested subsets recorded in compatibility.md. Future extension data must occupy legal transactional SQLite structures. Unknown feature bits, container versions, encodings, and index versions must be rejected instead of guessed.

Encrypted target: independent authenticated container, distinct from SQLite plaintext and SEE. No encrypted format is currently frozen or supported. Every format change requires an ADR, preserved snapshot, explicit migration and round-trip verification.

## M2 plaintext writes

Table and index pages use native SQLite leaf/interior formats, payload-locality rules and overflow chains. Index interior records are real keys, not duplicate separators. Root page numbers remain stable across root split/collapse. Freed B-tree/overflow pages return to legal SQLite freelist trunk/leaf pages and are reused before file extension. Schema entries reside in the page-one sqlite_schema B-tree; page-one header updates belong to the pager. User/application IDs and unrelated supported tables are preserved.

Writes require UTF-8, rollback mode, no reserved bytes, schema format 4 (or new empty format 0), no autovacuum, no unknown schema object/constraint, and a single-link regular file. A whole-file ownership preflight rejects overlapping live/free pages, cross-root reuse, duplicate roots and unowned physical pages before the first mutation. This is structural preflight, not a complete logical integrity checker.

The journal uses SQLite's documented magic, sector-aligned header, big-endian page numbers and nonce-seeded page checksums. Original pages and touched-sector neighbors precede physical database changes. Sector size defaults to an explicit 4096-byte assumption, configurable through SectorSize; hardware-specific atomicity is not certified. Existing journals are refused pending implemented recovery. No WAL format is supported yet. Commit ordering and error outcomes are specified in ADR0003; interrupted/failed-I/O execution remains NOT_RUN.
