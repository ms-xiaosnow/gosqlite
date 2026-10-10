# Legacy extension schema fixture

`legacy-grams.db` is an unmodified, normally closed SQLite format3 database created
by the project's **M6 commit14865c9798b62583323c4285fd064ecfea67c62c** from the
verified Library-restored source. It deliberately predates the optional ANN table
family. It contains the original seven-object grams extension family and three
ordinary source rows. The M7 compatibility test copies it into its own temporary
directory before opening for write; never modify this tracked fixture in place.

Creation with that revision's native `gosqlite` CLI:

```sql
CREATE TABLE legacy_docs(id INTEGER PRIMARY KEY,title TEXT);
INSERT INTO legacy_docs VALUES(1,'数据库旧索引'),(2,'普通文字'),(3,'向量与数据库');
CREATE INDEX legacy_grams ON legacy_docs(title) USING grams13;
```

The native LIKE plan searched two candidates; native extension consistency check
and official SQLite3.54.0 read-only `PRAGMA integrity_check` both returned `ok`.
Creation used ordinary SQL and normal close, with no corruption/fault/recovery.

- Size:36,864bytes
- SHA256:`5dd3237ecaeddb4b94c0019e4a21c7e851996c4438e3dfbe0bae8ec7cd3f92a8`
- Format:SQLite3, page4096, DELETEjournal

## Pre-IVF HNSW v1 fixture

`hnsw-v1.db` was generated on2026-10-10 by `go run ./examples/ann` from the
Library-restored, exact committed HNSW revision
`5a794e5d9fd6e04a3e488c1ad4fd82de74641d6f`. The normal example creates128
items with VECTOR(3) NOT NULL and a default L2 `items_hnsw` graph, runs native
exact/approximate and SQL queries, then closes normally. Official SQLite3.54.0
read-only integrity_check returned `ok`, row count128. This fixture predates
IVFFlat support and preserves exact canonical HNSW-v1 metadata for compatibility.

- Size:167,936bytes
- SHA256:`5e11c357723a134217b2f3f110c1e80770d758dad1ab2efc8cf4a9d503817401`
- Page4096, ordinary DELETEjournal, complete optional ANN family
- Copy into a temporary directory before every write test; never edit in place
