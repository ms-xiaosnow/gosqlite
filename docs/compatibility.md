# Compatibility matrix

Baseline: official SQLite 3.54.0; compatibility is established per tested feature. Version labels never imply full SQLite compatibility.

| Domain | Status | Evidence / boundary |
|---|---|---|
| Native Go scaffold | partial | version/help command; no database operations |
| SQLite header, records, B-tree | unsupported | M1 not started |
| SELECT / schema reads | unsupported | M1 not started |
| CREATE / CRUD / database/sql | unsupported | M2 not started |
| rollback journal / WAL / recovery | unsupported | M3 not started |
| grams13 / indexed LIKE | unsupported | M4 not started |
| similarity / threshold / top-K | unsupported | M5 not started |
| dense vectors / exact KNN | unsupported | M6 not started |
| HNSW / IVFFlat | unsupported | M7 not started |
| authenticated encryption / rekey | unsupported | M8 not started |
| Neon migration / extended vectors | unsupported | M9 not started |
| release audits / native OS evidence | unsupported | M10 not started |

All unspecified SQLite SQL, PRAGMAs, collations, encodings, and extension features are unsupported until listed with actual test evidence. PostgreSQL is not a production SQL backend. No SEE ciphertext interoperability is claimed.
