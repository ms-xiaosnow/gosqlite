# Architecture

The goal is a native Go engine, with SQLite 3 pages as its plaintext compatibility boundary. M2 adds native transaction overlays, local B-tree mutation, rollback-journal publication and SQL writes. Recovery/WAL and extension/security stages remain unimplemented.

Target data path:

    database/sql driver → public sqlite API → SQL parser/binder/planner → executor
                              ↓ transactions and schema                  ↓
                 registry / grams13 / vector index → B-tree / record → pager
                                                                         ↓
                                                             journal / codec / VFS

The engine will use bounded page access and query context cancellation. Connections and transaction snapshots, not a process-wide interpreter mutex, own execution. Writers must use native file locks before publishing SQLite pages; readers must never ignore hot journals or WAL.

Persisted extension indices share the table transaction. Encryption sits below logical pages, with an independent authenticated physical container. Unsupported formats and semantics fail closed. Go structs are not disk formats.

Implemented M1 path: cmd/gosqlite query/inspect or database/sql driver → sqlite pull executor → internal/sql expressions/parser + schema interpretation → streaming internal/btree cursor + internal/record decoder → read-only internal/pager and native Linux shared locks. The public sqlite package currently owns the small pull executor rather than creating a placeholder VM package. ORDER BY is separately bounded; ordinary scans stream. The PageReader abstraction decouples traversal from the physical reader for future transaction overlays. See ADR0002 for lock lifetimes, quotas and rejected modes.

Implemented M2 write path: parsed CREATE/INSERT/UPDATE/DELETE → full supported-catalog and page-ownership validation → statement undo snapshot → local table/index B-tree operations → bounded pager dirty overlay and SQLite freelist → native RESERVED/PENDING/EXCLUSIVE locks → standard DELETE rollback journal and database publication. Successful statements remain in an explicit transaction; a rejected statement restores its own overlay without erasing earlier statements. No page spill is used. Index pages are maintained, but SELECT still scans.

Idle connections release snapshots; each query or transaction owns its file handle and locks. Native Commit returning BUSY before physical I/O can be retried after readers close. database/sql invalidates a Tx after any Commit result, so its adapter rolls back after a failed commit. Once commit physical I/O begins, cancellation is deferred until the operation resolves. Any ambiguous physical-I/O result poisons the handle; it cannot be used as a successful commit.

Recovery of an existing journal is not yet implemented. All writable opens refuse any journal sidecar. Source-ordering review and normal commit tests do not prove crash atomicity. See ADR0003 and manual-verification.md. Planned packages are created only with real implementation, not placeholder interfaces.
