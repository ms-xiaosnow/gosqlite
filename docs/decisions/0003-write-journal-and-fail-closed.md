# ADR 0003: Native bounded writes and fail-closed journal publication

Status: accepted for M2 implementation; crash/fault durability acceptance remains blocked.

The SQL MVP uses statement snapshots of a bounded transaction dirty-page map. No dirty-page spill or temporary SQL files occur. Ordinary failure before publication restores the statement overlay, including allocation/freelist and schema-cookie changes. Explicit rollback discards the full overlay. Multirow constraint failures cannot leave earlier rows from that statement in the transaction.

Local table/index B-tree mutations split, redistribute/merge and collapse stable roots. They do not serialize a replacement database or rebuild every table. Real secondary-index keys include the trailing rowid. Only ascending BINARY, nonunique ordinary indices are supported. Before mutation, the whole supported catalog and page ownership graph are validated; unknown objects, unowned pages and live/free aliasing fail closed.

Linux native OFD byte-range locks cover SQLite's shared/reserved/pending regions. Readers coexist; one writer obtains RESERVED before changing the overlay. EXCLUSIVE is acquired before journal creation. A contention-only failure releases temporary PENDING and remains retryable. Cleanup failure poisons and closes the transaction. Unsupported OSes fail explicitly.

Publication order: EXCLUSIVE; create journal exclusively; write invalid header and original-page records including sector neighbors; journal Sync; publish valid first12 header bytes and Sync; directory Sync; database page writes and Sync; unlink journal; directory Sync; close. The commit path no longer observes cancellation after physical I/O starts. WriteAt short counts are errors. If database I/O may have begun, preserve the synced rollback journal and poison the transaction. A final unlink/directory-sync ambiguity returns ErrCommitUncertain rather than success. Source review does not qualify any of these failure paths.

Default sector size4096 is a stated assumption, not hardware discovery. The public options and internal pager permit an explicit supported sector size. New-file initialization and existing-file update are covered by normal-success tests only. Writable opens refuse any existing journal pending native recovery; read-only opens apply the same any-journal rejection. WAL is unsupported. No rename-based replacement of existing databases is used.

The public native API permits a BUSY Commit retry. database/sql's transaction lifecycle does not, so its adapter rolls back on Commit failure and returns the original error. Writable zero-value options permit creation; NoCreate and ReadOnly are explicit alternatives. CLI SELECT is read-only and one DML/DDL statement is one autocommit transaction. Explicit multi-statement transactions use the Go API.

Consequences: M2's ordinary data-format/SQL tests can be verified independently, but M3 and dependent durability gates stay BLOCKED/NOT_RUN until allowed independent crash/fault evidence exists. Production use, full SQLite compatibility, logical integrity completeness and non-Linux runtime support are not claimed.
