# File formats

M2 reads and writes the tested rowid-table and BINARY ASC secondary-index subset of SQLite format 3, according to the [official specification](https://www.sqlite.org/fileformat2.html), with tested subsets recorded in compatibility.md. Future extension data must occupy legal transactional SQLite structures. Unknown feature bits, container versions, encodings, and index versions must be rejected instead of guessed.

Encrypted target: independent authenticated container, distinct from SQLite plaintext and SEE. No encrypted format is currently frozen or supported. Every format change requires an ADR, preserved snapshot, explicit migration and round-trip verification.

## M2 plaintext writes

Table and index pages use native SQLite leaf/interior formats, payload-locality rules and overflow chains. Index interior records are real keys, not duplicate separators. Root page numbers remain stable across root split/collapse. Freed B-tree/overflow pages return to legal SQLite freelist trunk/leaf pages and are reused before file extension. Schema entries reside in the page-one sqlite_schema B-tree; page-one header updates belong to the pager. User/application IDs and unrelated supported tables are preserved.

Writes require UTF-8, rollback mode, no reserved bytes, schema format 4 (or new empty format 0), no autovacuum, no unknown schema object/constraint, and a single-link regular file. A whole-file ownership preflight rejects overlapping live/free pages, cross-root reuse, duplicate roots and unowned physical pages before the first mutation. This is structural preflight, not a complete logical integrity checker.

The journal uses SQLite's documented magic, sector-aligned header, big-endian page numbers and nonce-seeded page checksums. Original pages and touched-sector neighbors precede physical database changes. Sector size defaults to an explicit 4096-byte assumption, configurable through SectorSize; hardware-specific atomicity is not certified. Existing journals are refused pending implemented recovery. Default rollback opens reject WAL; the explicit M3 exclusive mode is described below. Commit ordering and error outcomes are specified in ADR0003; interrupted/failed-I/O execution remains NOT_RUN.


## M3 WAL and recovery source

WAL uses SQLite's32-byte header and24-byte frame header plus a complete page. Stored integers are big-endian; magic selects little/big-endian checksum input words. Frame salts bind the WAL generation, while checksum recurrence follows the public format. They do not authenticate the owning database. File pairing and cooperative native locks remain required.

The committed overlay selects the newest page image through the last complete commit marker, with logical extent from that marker and strict bounded scanning. Invalid, incomplete, uncommitted and salt-mismatching tails are classified and refused; this does not implement SQLite's full recovery policy. Checkpoint retains synced WAL images for every extant main-file sector neighbor before overwriting pages. Header mode2 remains after normal checkpoint/close, even when the WAL is removed.

Rollback replay plans support one finite-count header/record segment, page geometry, nonce-seeded sparse checksums, exact original extent and complete missing-original coverage if the current extent is shorter. Physical replay is deliberately isolated and unqualified. See ADR0004 and the source-only manual harness; no encrypted format exists yet.

## M3 shared WAL-index

The optional shared mode implements SQLite's transient native-endian SHM format: two checked48-byte headers,32KiB hash/index regions, reader marks/backfill counters and the standard lock bytes. These sidecars are not portable interchange artifacts independent of their database/WAL and locks. Every reader pins a generation and maximum frame; hash lookup returns the newest page image within its frame bounds. Publication follows WAL Sync and checked SHM header ordering. Shared mode retains all three inodes and resets salts only with complete backfill and exclusive ownership of every WAL-reader slot.

Unlike the earlier exclusive strict-tail policy, shared cold reconstruction accepts a fully checksum-valid uncommitted ordinary rollback suffix but indexes only the last complete commit; normal old-salt reset suffixes are also accepted. Incomplete/checksum-invalid current-generation input is still refused. A subsequent native commit trims only obsolete bytes beyond its new committed end under the writer lock. See ADR0006 for limits and unverified interruption assumptions.
