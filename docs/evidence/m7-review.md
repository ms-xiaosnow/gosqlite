# M7 first HNSW scoped engineering review

2026-10-10, native Go1.27.2 Linux/amd64. This is engineering review, not independent
security/legal/cryptographic certification or full milestone acceptance.

Source inspection confirmed a genuine deterministic multilayer graph, lazy
per-node B-tree access, traversable immutable tombstones and no whole-graph
Open/query preload. Model tests use memory stores; persisted tests use normal
transactional SQLite files. Unknown config/format/identity refuses. Existing
grams schema bytes and canonical identity remain preserved.

One confirmed defect: automatic SQL ANN reserved requested LIMIT/OFFSET capacity
even on a two-live-row table. LIMIT10000 under16KiB failed where NOT INDEXED
succeeded. The query owner fixed effective retention to min(requested,validated
live cardinality), without relaxing quotas or changing requested diagnostics.
Independent unchanged regression now passes, including huge OFFSET, zero LIMIT,
empty/all-tombstone, all-filtered, bad/NULL-query lazy behavior.

Four independent tests in sqlite/ann_review_test.go also cover same-transaction
update/delete/rowid reuse/reindex/rollback, mixed grams/ANN and signed int64 extrema
and ties. Focused ANN query/storage/review tests and race passed. Memory-only
adjacency fuzz407170 executions passed. GOARCH386 package compilation succeeded,
but attempting execution returned exec format error; no32bit runtime evidence.
SQL int64-to-int controls check round-trip bounds before narrowing.

Lead independently exercised official SQLite six-page/mode combinations plus an
actual pre-ANN fixture upgrade; ordinary tests pass. The first real persistent
benchmark met fixed-data recall@10 target but was slower than exact scan.
Evidence is retained without hiding that performance result. No physical fault,
damaged on-disk image, forced process exit or recovery workflow was run.

No remaining actionable source finding was reported at review close. This is
scoped evidence, not proof of absence of defects. IVFFlat remains required code;
restricted durability and independent release acceptance remain open.

Lead driver regression initially failed because its assertion expected a method=hnsw
substring absent from the existing human-readable plan format. Actual plan
correctly showed USING items_hnsw hnsw v1 and exact=false. The test assertion was
corrected without production changes; final rollback/shared prepared bindings,
read-own-update/rollback/reindex race count3 passed11.294s.
