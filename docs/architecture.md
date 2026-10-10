# Architecture

The goal is a native Go engine, with SQLite 3 pages as its plaintext compatibility boundary. M1 implements the read-only path below; write/extension/security stages remain unimplemented.

Target data path:

    database/sql driver → public sqlite API → SQL parser/binder/planner → executor
                              ↓ transactions and schema                  ↓
                 registry / grams13 / vector index → B-tree / record → pager
                                                                         ↓
                                                             journal / codec / VFS

The engine will use bounded page access and query context cancellation. Connections and transaction snapshots, not a process-wide interpreter mutex, own execution. Writers must use native file locks before publishing SQLite pages; readers must never ignore hot journals or WAL.

Persisted extension indices share the table transaction. Encryption sits below logical pages, with an independent authenticated physical container. Unsupported formats and semantics fail closed. Go structs are not disk formats.

Implemented M1 path: cmd/gosqlite query/inspect or database/sql driver → sqlite pull executor → internal/sql expressions/parser + schema interpretation → streaming internal/btree cursor + internal/record decoder → read-only internal/pager and native Linux shared locks. The public sqlite package currently owns the small pull executor rather than creating a placeholder VM package. ORDER BY is separately bounded; ordinary scans stream. The PageReader abstraction decouples traversal from the physical reader for future transaction overlays. See ADR0002 for lock lifetimes, quotas and rejected modes.

Planned packages are created only with real implementation, not placeholder interfaces. Encryption, journal/WAL, extension registry and mutable B-trees remain future work.
