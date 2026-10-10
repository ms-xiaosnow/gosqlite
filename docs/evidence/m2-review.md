# M2 engineering review and ordinary test evidence

Date: 2026-10-10 UTC. Native implementation: Go1.27.2. Official test-only oracle: SQLite3.54.0, pinned source ID in upstream.md. This is scoped engineering review, not independent release security/legal certification.

## Independent source and ordinary-oracle pass

Reviewed pager overlay/undo, freelist, native Linux lock transitions, journal publication order, table/index mutation and overflow ownership, SQL statement rollback/catalog/index maintenance and driver lifecycle.

Concrete defects found and fixed before checkpoint:

1. A DB handle kept a caller-relative/unresolved path while reopening snapshots, permitting a cwd/symlink change to redirect later operations. Resolve once at Open; retained ordinary valid-file cwd/symlink tests, plus an active transaction pathname-replacement regression.
2. Failure to release temporary PENDING after failed EXCLUSIVE acquisition could be returned as retryable BUSY. Cleanup failure now poisons/closes. This physical-error branch was source-reviewed only, NOT_RUN.
3. Reader-first public Open classified journals differently from writable Tx. Shared sidecar policy now refuses every existing journal with ErrRecoveryRequired. Existing-journal/recovery workflows remain NOT_RUN.
4. Cursor ownership checks lacked equal leaf depth/nonempty non-root enforcement. Added checks and pure-memory malformed-tree regressions, preserving SQLite's legal unary schema root.
5. Index mutation memory accounting did not consistently cover input/comparison payloads. Capped mutable index records at16MiB, bounded combined accounting and streamed overflow freeing. Retained memory boundary regression.
6. Randomized B-tree mutation exposed a large middle-cell insertion needing a three-way table leaf split. Fixed and retained model test.

A pre-final oracle aggregate failed because a legacy read-only test's driver DSN implicitly became writable and deleted rows from the generated page512 fixture. Corrected all legacy oracle read opens to explicit ReadOnly/mode=ro, tested supported read-only Tx rather than obsolete transaction rejection, restored exact M1 fixture bytes and reran. The failure is not counted as passing evidence.

Final independent command, exit0:

```
CGO_ENABLED=0 SQLITE_ORACLE=$PWD/../toolchains/sqlite-build-3540000/sqlite3 ../toolchains/go1.27.2/bin/go test -count=1 -tags oracle ./...
```

All packages passed; B-tree11.305s, end-to-end oracle testkit2.950s. Separate writable review cache used. Readonly fixture SHA-256 values before/after unchanged. No unresolved concrete defect found within this reviewed subset. Complete logical integrity, external native-OS behavior and interrupted-I/O behavior are outside this finding.

## Component verification retained in source

- Table/index valid-memory models: 5,000 table plus4,000 index operations at512/4096/65536 bytes; signed extrema, mixed SQLite value types, overflow, stable roots and local write-count checks.
- Deletion review:1,800 composite keys at usable480/512/1024/2048/8192, interior-root/deepest-record deletion with full model/equal-depth/ownership assertions;15,000 variable-size schema-root table operations.
- B-tree temporary-file oracle:1,000 rows and208KB indexed overflow, official integrity/index reads, official writes back, native500 deletes/500 updates and complete drain. Three page sizes passed.
- End-to-end public API oracle:320 inserts plus large indexed text/blob, compound and simple indices, updates/deletes/reinsert/freelist reuse, official writes back; rows, SQL types, order and forced-index results compared with reference. Transactional DDL/row rollback preserves complete original bytes; constraint-rejected multirow statements leave no partial rows and preserve earlier successful statements.
- Ordinary locks: native SHARED conflicts with official BEGIN EXCLUSIVE; RESERVED conflicts with official BEGIN IMMEDIATE while official reads succeed; normal close releases locks. No forced process exit.
- Native/driver constraints, default/affinity/rowid semantics, read snapshots, read-own-writes, transaction rollback, cancellation, budget rejection, pooled inserts and BUSY commit behavior tested repeatedly and with race instrumentation.
- TestTxStatementPanicAndCancellationUndo is entirely in-memory before commit, recovering the callback panic in the same goroutine. It is not a process/crash/fault test.

Aggregate command output and fixture hashes: m2-aggregate.txt. There are no crash/fault/recovery test results. Those gates remain BLOCKED/NOT_RUN.
