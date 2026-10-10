# ADR 0002: Read-only snapshots and bounded parsing

Accepted 2026-10-10 for M1.

A database reader holds a SQLite-compatible shared lock for its entire lifetime. Linux uses open-file-description fcntl locks on SQLite's pending/shared byte ranges, avoiding the close-one-descriptor-drops-all-process-locks problem of POSIX locks. A writer can therefore be blocked until readers close. M1 does not promise concurrent write progress.

The reader opens existing regular files read-only, resolves symlinks before checking sidecars, rejects nonempty journals/WAL and header WAL mode, and verifies file identity/size/header generation during reads. This intentionally rejects cold persistent journals too. No recovery is attempted. Unsupported OS locking returns an error rather than opening unlocked. Four-target compilation is distinct from native runtime validation.

B-tree cursors retain a bounded traversal path and one row. Limits are explicit: 64 MiB payload, 32,767 record columns, depth 64, 1,048,576 visited pages, and16 MiB retained schema metadata. ORDER BY has a separate configurable retained-memory budget (default 64 MiB), never plaintext spill. SQL parsing has byte/depth/parameter limits. Exhaustion returns errors; no result truncation is disguised as success.

All M1 queries scan a rowid table or the schema. Existing ordinary secondary indexes may coexist, but no query is claimed to use them. Index/WITHOUT ROWID traversal, writes, journaling/recovery and unsupported SQL fail explicitly. The B-tree PageReader interface is ready for a future transaction overlay without connecting the executor to physical storage.

Sources: [file format](https://www.sqlite.org/fileformat2.html), [locking](https://www.sqlite.org/lockingv3.html), and the pinned public SQLite source in docs/upstream.md. This is implementation rationale, not an independent safety audit.
