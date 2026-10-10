# File formats

M2 reads and writes the tested rowid-table and BINARY ASC secondary-index subset of SQLite format 3, according to the [official specification](https://www.sqlite.org/fileformat2.html), with tested subsets recorded in compatibility.md. Future extension data must occupy legal transactional SQLite structures. Unknown feature bits, container versions, encodings, and index versions must be rejected instead of guessed.

The experimental encrypted snapshot format is an independent authenticated root/base container, distinct from SQLite plaintext and SEE. It now supports snapshot storage; encrypted WAL and a stable encrypted-format promise remain open. See [encryption-storage.md](encryption-storage.md). Every format change requires an ADR, preserved snapshot, explicit migration and round-trip verification.

## M2 plaintext writes

Table and index pages use native SQLite leaf/interior formats, payload-locality rules and overflow chains. Index interior records are real keys, not duplicate separators. Root page numbers remain stable across root split/collapse. Freed B-tree/overflow pages return to legal SQLite freelist trunk/leaf pages and are reused before file extension. Schema entries reside in the page-one sqlite_schema B-tree; page-one header updates belong to the pager. User/application IDs and unrelated supported tables are preserved.

Writes require UTF-8, rollback mode, no reserved bytes, schema format 4 (or new empty format 0), no autovacuum, no unknown schema object/constraint, and a single-link regular file. A whole-file ownership preflight rejects overlapping live/free pages, cross-root reuse, duplicate roots and unowned physical pages before the first mutation. This is structural preflight, not a complete logical integrity checker.

The journal uses SQLite's documented magic, sector-aligned header, big-endian page numbers and nonce-seeded page checksums. Original pages and touched-sector neighbors precede physical database changes. Sector size defaults to an explicit 4096-byte assumption, configurable through SectorSize; hardware-specific atomicity is not certified. Existing journals are refused pending implemented recovery. Default rollback opens reject WAL; the explicit M3 exclusive mode is described below. Commit ordering and error outcomes are specified in ADR0003; interrupted/failed-I/O execution remains NOT_RUN.


## M3 WAL and recovery source

WAL uses SQLite's32-byte header and24-byte frame header plus a complete page. Stored integers are big-endian; magic selects little/big-endian checksum input words. Frame salts bind the WAL generation, while checksum recurrence follows the public format. They do not authenticate the owning database. File pairing and cooperative native locks remain required.

The committed overlay selects the newest page image through the last complete commit marker, with logical extent from that marker and strict bounded scanning. Invalid, incomplete, uncommitted and salt-mismatching tails are classified and refused; this does not implement SQLite's full recovery policy. Checkpoint retains synced WAL images for every extant main-file sector neighbor before overwriting pages. Header mode2 remains after normal checkpoint/close, even when the WAL is removed.

Rollback replay plans support one finite-count header/record segment, page geometry, nonce-seeded sparse checksums, exact original extent and complete missing-original coverage if the current extent is shorter. Physical replay is deliberately isolated and unqualified. See ADR0004 and the source-only manual harness. The separate [experimental authenticated snapshot format](encryption-storage.md) does not implement encrypted WAL.

## M3 shared WAL-index

The optional shared mode implements SQLite's transient native-endian SHM format: two checked48-byte headers,32KiB hash/index regions, reader marks/backfill counters and the standard lock bytes. These sidecars are not portable interchange artifacts independent of their database/WAL and locks. Every reader pins a generation and maximum frame; hash lookup returns the newest page image within its frame bounds. Publication follows WAL Sync and checked SHM header ordering. Shared mode retains all three inodes and resets salts only with complete backfill and exclusive ownership of every WAL-reader slot.

Unlike the earlier exclusive strict-tail policy, shared cold reconstruction accepts a fully checksum-valid uncommitted ordinary rollback suffix but indexes only the last complete commit; normal old-salt reset suffixes are also accepted. Incomplete/checksum-invalid current-generation input is still refused. A subsequent native commit trims only obsolete bytes beyond its new committed end under the writer lock. See ADR0006 for limits and unverified interruption assumptions.

## M4 inverted-index storage version1

The exact legal __gosqlite_ext_* catalog, fields, postings and frequency schemas and covering indices are defined by extensionSchemas in sqlite/extension_catalog.go. Catalog rows bind logical name/source SQL hash, method/storage versions, canonical dual-namespace tokenizer configuration/hash, generation, dirty state, source-row count and schema cookie. Posting keys include index id, field id, namespace, gram length, UTF-8 token, content rowid and a separate physical posting rowid. No private page types or unsupported CREATE syntax are written.

Exact source-table guard triggers mark indices dirty under official SQLite DML; schema-cookie validation catches schema-level operations such as VACUUM. Native transactions maintain postings and frequencies directly, recognizing only their exact generated guards. Config changes require one atomic REINDEX. Unknown versions or modified owned schemas fail closed. This first format is experimental; no backward format migration or crash-recovery qualification is claimed. Definitions and bounds are specified in grams13.md.

## M6 dense-vector value codec and SQL storage

The pure mathematical vector package encodes one value as12+4×dimension bytes: GSV\x00 magic; version1; float-format1; byte-order1 (big-endian); zero flags; uint32 dimension; big-endian IEEE754 float32 bit patterns. Dimensions are1..16,000 and every component must be finite. Signed zero/subnormal bits are preserved. Unknown encoding versions/flags, incorrect exact length and nonfinite components reject. There is no checksum/authentication and no PostgreSQL wire compatibility. Native VECTOR(n) INSERT/UPDATE now validates declared dimensions and canonicalizes to this BLOB inside the same statement/transaction as ordinary rows and extension indices. The legal VECTOR(n) declaration is stored in sqlite_schema. Valid externally written SQLite TEXT/BLOB is interpreted with strict dimension checks; raw read storage class is preserved until native rewrite. See vector-primitives.md and the pinned fixture manifest.

## M7 optional ANN family version1

The original seven grams schemas are unchanged. The optional complete eleven-
object ANN family consists of state/nodes/live/centroids/members tables and six
lookup B-trees. Absent family is legal for older grams-only files; partial or
unrecognized owned objects reject. First HNSW or IVFFlat creation initializes the family
transactionally. Catalog config binds method/version, dimensions, metric/opclass,
seed, construction policy and limits with strict canonical bytes. State tracks
entry, top level, next immutable identity, live/tombstone counts. Each node stores
original GSV bytes and strict versioned adjacency; live source-rowid mapping is
separate. Physical table rowids are not content rowids or graph identities.
Centroids/members remain empty in HNSW and are checked accordingly.

No custom SQLite page type is introduced. Ordinary official SQLite can preserve
these structures and execute their source invalidation guards, but cannot execute
the native ANN methods itself. Normal creation/mutation/rebuild/rollback/reopen/checkpoint and actual
pre-ANN file upgrade are tested. Interrupted publication/recovery and future
format migration are not qualified. Reindex keeps the immutable configuration.
IVFFlat now uses the centroid/member tables already reserved in this family and
preserves HNSW canonical serialization exactly.

### M7 IVFFlat version1 state, centroids and membership

All rows are keyed by logical index ID within the same optional ANN family.
The exact schema definitions remain in `sqlite/ann_storage.go`; no private page
types or PostgreSQL binary-index structures are introduced.

- `__gosqlite_ext_ann_state` contains one row per index. IVF stores entry 0,
  maximum level -1, next node ID 1, tombstones 0, a bounded non-NULL eligible
  source count, and training state exactly `untrained` or `ready`. The ordinary
  catalog separately counts all source rows, including NULL vectors, and binds
  generation, schema cookie, source SQL hash and configuration hash.
- `__gosqlite_ext_ann_centroids` stores physical ID, index ID, list ID, canonical
  GSV-encoded float32 centroid and member count. A ready index has exactly one
  centroid for every list ID 1..lists. The `(index_id,list_id)` lookup locates its
  bounded centroid range. Vector dimensions/finite values and spherical unit
  norms are checked; per-list counts are reconciled by the full checker.
- `__gosqlite_ext_ann_members` stores physical ID, index ID, list ID and signed
  content rowid. It does not duplicate the source vector. The covering
  `(index_id,list_id,content_rowid)` B-tree supplies each selected list's range;
  `(index_id,content_rowid)` supplies the reverse identity lookup. Search checks
  both against the physical membership row before fetching the source record.
  Physical rowids are neither source rowids nor list IDs.
- An untrained index has no centroid or member rows. Inserts/updates/deletes
  maintain eligible/source counts but never implicitly train it. Ready IVF
  indexes have one membership for every non-NULL source vector and no graph
  nodes, live mappings or tombstones. Cosine zeros reject; IP zeros are valid
  members assigned to list 1, but excluded from spherical training.

Creation/REINDEX first performs bounded deterministic bottom-hash sampling and
Lloyd training, then assigns every eligible source vector to the nearest stored
centroid. Readiness is published only with the complete centroid/member/count
state. Insufficient trainable samples persist `untrained` instead. Source rows,
all shadow records and catalog metadata share statement/transaction rollback.
Ordinary trained DML maintains membership under fixed centroids; only explicit
REINDEX retrains and atomically replaces that state with an advanced generation.
The full checker validates source-to-nearest-centroid assignment, both lookup
directions, list counts and absence of orphan/unexpected owned records.

IVF configuration has its own closed canonical JSON envelope, separate from
HNSW-v1's byte-for-byte preserved encoding. Its ordered fields are method,
version, dimensions, metric, opclass, seed, lists, training_samples,
training_iterations, max_training_work, max_nodes, max_visited,
max_distance_evaluations, max_memory_bytes, build_policy, sample_policy,
zero_policy and empty_policy. The version-1 policy strings are:

- `build_policy`: `farthest-first-lloyd-f64-spherical-v1`
- `sample_policy`: `splitmix64-bottom-hash-rowid-v1`
- `zero_policy`: `reject-cosine-zero-skip-ip-zero-training`
- `empty_policy`: `retain-prior-empty-or-zero-sum`

Parser acceptance requires exact equality to canonical serialization, not merely
equivalent JSON. Unknown/missing/duplicate fields, alternate ordering, altered
policy strings, noncanonical bytes, trailing data or unsupported versions reject.
No IVF field is added to the persisted HNSW envelope, so existing HNSW hashes do
not change. See [ann.md](ann.md) and the
[algorithm contract](../extension/vector/ann/README.md) for deterministic
training, resource bounds, probes, fallback and compatibility limits.

## M8 authenticated records and experimental snapshots

The encryption package defines an experimental 512-byte GSQLENC1 bootstrap and
24-byte authenticated-record header plus AES256GCM ciphertext/tag. The snapshot
storage layer now uses authenticated root/base objects for database publication,
rekey, encrypted backup/restore and explicit plaintext import/export. Existing
plaintext files are not silently transformed. Encrypted WAL, a stable format
promise and physical crash/security qualification remain open. See
[encryption-storage.md](encryption-storage.md) for the root/base snapshot format
and [encryption-primitives.md](encryption-primitives.md) for record-codec offsets
and cryptographic scope.
