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
