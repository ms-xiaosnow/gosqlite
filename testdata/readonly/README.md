# Read-only SQLite oracle fixtures

Regenerate with the pinned official SQLite 3.54.0 CLI, strictly as a test oracle:

    python3 testdata/readonly/generate.py ../toolchains/sqlite-build-3540000/sqlite3

`fixture.sql` is identical across 512, 4096 and 65536-byte page sizes. Each
fixture has ordinary table and index B-trees, a 1,028-row `samples` table,
multiple B-tree/overflow pages, signed rowid and integer extremes, every
integer serial width, NULL/empty/nonempty BLOB and text, floating-point values,
and CJK/emoji/combining Unicode. `samples.json` is the official CLI's full
ordered result with BLOBs encoded as hex and an explicit NULL-BLOB marker.
`numbers` has 25 serial-width boundary values. `quotes` tests quoted names.
The generator verifies `integrity_check = ok` and equal oracle result sets
for all three page sizes, recording SHA-256 digests in `manifest.json`.
Ordinary Go tests read committed bytes and never run the external CLI.

`schema512.db` contains 80 small tables and one 600-column table. Its
`sqlite_schema` root is an interior B-tree page, and the wide CREATE TABLE
SQL itself uses overflow pages. `schema.json` is the complete official
schema result, compared field-for-field by Go tests.
