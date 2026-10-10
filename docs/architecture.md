# Architecture

The goal is a native Go engine, with SQLite 3 pages as its plaintext compatibility boundary. The current M0 scaffold has no database execution implementation.

Planned data path:

    database/sql driver → public sqlite API → SQL parser/binder/planner → executor
                              ↓ transactions and schema                  ↓
                 registry / grams13 / vector index → B-tree / record → pager
                                                                         ↓
                                                             journal / codec / VFS

The engine will use bounded page access and query context cancellation. Connections and transaction snapshots, not a process-wide interpreter mutex, own execution. Writers must use native file locks before publishing SQLite pages; readers must never ignore hot journals or WAL.

Persisted extension indices share the table transaction. Encryption sits below logical pages, with an independent authenticated physical container. Unsupported formats and semantics fail closed. Go structs are not disk formats.

At M0 the implemented packages are cmd/gosqlite (version/help) and internal/buildinfo (pinned provenance). Planned packages are created only with real implementation, not placeholder interfaces.
