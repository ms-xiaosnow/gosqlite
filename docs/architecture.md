# Architecture

The goal is a native Go engine, with SQLite 3 pages as its plaintext compatibility boundary. M2 adds native transaction overlays, local B-tree mutation, rollback-journal publication and SQL writes. M3 adds an explicit connection-owned exclusive-WAL path and isolated recovery source; full concurrent WAL, qualified recovery and extension/security stages remain unfinished.

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


M3 WAL route: DB(WALExclusive) → pager.Session retaining one EXCLUSIVE descriptor → Reader/Tx lease → dirty overlay → native WAL codec/committed-page index → synced WAL frames. Query, Schema and Begin all use the owning session; they never open an independent file descriptor against its lock. Checkpoint extends sector protection for imported logs, syncs WAL, copies committed images to main, syncs main, then removes sidecars and syncs the directory. Default Close checkpoints; explicit KeepWALOnClose preserves normally committed frames. Shared-memory WAL is not implemented.

Rollback recovery is a separate explicit internal operation. Its parser/planner is tested only in memory; physical replay is source-reviewed and compile-checked but not executed. Normal Open retains the fail-closed journal policy. This boundary prevents an ordinary test or user query from implicitly entering an unqualified recovery workflow.
