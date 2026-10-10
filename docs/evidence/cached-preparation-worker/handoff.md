# Cached-schema preparation: final owner handoff

## Result and identity

Implemented a bounded metadata-only repair for ordinary rollback-mode cached
schema preparation at first-snapshot BUSY. Invalid prepared DML preserves counters
and reports its native preparation error; valid DML reports BUSY and resets
changes to zero. The cached metadata cannot authorize row access or mutation.
Independent review of the exact final freeze is assigned separately; this handoff
does not substitute an owner result for its verdict.

- Required immutable baseline: sql-connection-counters-freeze-v1,
  tree `7c8f7a266a63a713b22709268fdd4d25f1b875d1`.
- Final: sql-cached-preparation-freeze-v3,
  tree `18cab5976106fc7a794066d50c9d8a0c199efe49`.
- Complete 1,264-file inventory: all-sha256.txt, SHA-256
  `ebc6d47b2de38c6ea97559ff33559034cbd666f9e2c9f175900ad6df8131e07a`.
- Exact checked patch: source.patch, SHA-256
  `c96429d2427037bfc6c2286ff2a744300484f8f0834c419e4ace6c1bafcd50e0`.
- manifest.json records all 14 owned baseline/final hashes and isolated Git
  tree-object store; baseline-owned-sha256.txt and owned-sha256.txt are separate
  inventories. patch-apply.txt verifies fresh baseline + exact patch reproduces
  the complete path/byte/mode identity. final-full-verification.txt verifies the
  final worktree, freeze and replay after tests, with original baseline intact.

## Design

`sqlite/preparation_cache.go` retains immutable copies of table/column names and
INTEGER PRIMARY KEY ordinal, with no roots, defaults, indexes, row data, pager
handle or execution plan. Budget is min(MaxSortBytes, 16 MiB) with fixed/per-entry
charges and an additional pre-parsing declaration charge for read-warmed schemas.
This is a separately bounded metadata allowance, not a joint active-query sort cap.
Oversized/unsupported cache candidates are discarded without changing ordinary
query success or introducing a new query error.

Existing INSERT/UPDATE/DELETE validation moved into shared preparation routines;
ordinary execution and cached classification call the same logic. Runtime values
are not evaluated while blocked. Syntax/parameters, read-only and active Rows
checks retain their existing order. Snapshot acquisition is still attempted first.
Only an error tree consisting solely of ordinary pager/VFS BUSY sentinels permits
cached classification; accompanying native errors retain precedence. No cleanup,
recovery, pager or VFS production implementation is changed.

Cache population follows full ordinary catalog validation, with normal ownership
checking on writes. User-table planning and native Schema can warm it. Successful
snapshot observes cookie changes and uses the fresh catalog; successful native
CREATE updates metadata; DDL-discarding rollback/savepoint rollback and Close
invalidate it. Shared/WAL/encrypted sessions and extension-owned/unsupported
catalogs retain conservative first-snapshot admission. Connection counters,
source-row insert/ABORT accounting, public Options/Result fields and existing
statement-specific Result values remain unchanged.

## Deliberate remaining boundaries

- No cache means conservative admission. Quota rejection, sessions and extension
  schemas are documented no-cache cases.
- Constant SELECT and SQL sqlite_schema projection do not initialize this native
  optional cache, while official SQLite may initialize schema there. Native DB
  constants still acquire their usual reader; Tx constants remain lazy.
- Successful snapshot paths preserve current native fresh-catalog/error ordering.
  This does not emulate every SQLite stale-schema retry rule. The retained oracle
  negative shows official old-column-count INSERT preparation can fail after
  DROP/recreate until another suitable statement refreshes schema. Final passing
  histories deliberately refresh using fixed-label SELECT count(*).
- Driver Prepare stores reusable SQL text, parsing for validity; no new public
  prepared-plan API is introduced. Full SQLite preparation parity is not claimed.

`docs/sql-cached-preparation.md` is the dedicated scope document.
`docs/sql-connection-counters.md` replaces only its formerly documented gap with
this bounded behavior and links the new scope. No central goal/progress/README or
compatibility file changed.

## Evidence

`run-selected.sh` fixes Go 1.27.2, local offline caches, GOTOOLCHAIN=local,
GOPROXY/GOSUMDB=off, CGO0 production and CGO1 race only, and the pinned official
SQLite3.54 CLI. Every execution batch has a behavior audit and `internal/testscope`
first. Exact final results:

- selected-checks-v2.txt/.exit: PASS exit0.
- frozen-checks.txt/.exit: v2 frozen code PASS exit0.
- frozen-v3-checks.txt/.exit: exact final v3 PASS exit0, completed
  2026-10-10 20:41:30 UTC.
- Final normal tests select new native/driver cases and affected existing counter,
  SQL-expression, transaction, savepoint, Rows, cancellation/read-only/quotas cases.
- New typed oracle histories cover 512/4096/65536-page rollback-mode schemas,
  fresh/read-warm states, invalid names/arity/counts, runtime error behind BUSY,
  ordinary peer SQL ALTER/DROP/recreate, stale positive/negative schemas, DDL
  rollback/savepoints and explicit deferred transactions. Whole labels/values and
  storage classes are compared; primary native errors compare official categories.
- Existing six rollback/shared-WAL counter histories and live promotion/commit
  BUSY tests remain selected, including partial INSERT/ABORT and Result values.
- Focused native/driver/scalar race and oracle-race; affected vet and production
  build all passed. No broad test suite or extra build tags were used.
- source-audit.txt records the production invariants and pinned public source
  functions/locations. oracle/probe.py/.out retain standalone typed/error histories.

`negative-controls.md` explains retained failures. In particular:

- New strict oracle tests fail against unchanged old production source, proving
  the actual prior gap; baseline-strict-negative.txt/.exit retains that result.
- Independent review caught an empty-schema fixed-charge edge below 256 bytes.
  It was fixed before final freeze, with explicit 1/64/255-byte tests.
- Initial full batch hit official CLI startup SHARED-lock contention under race;
  a `.print`-only startup handshake fixed the harness without warming schema.
  Earlier failing log and exact v1 source identity are retained.
- Stale INSERT cardinality and SELECT-star shell-header negatives are retained,
  not counted as parity. The two historical new-oracle source reconstructions
  are explicitly labeled as reconstructions of the one-line continuation changes.

No physical corruption/tamper/wrong-key/missing-selected-file, namespace mutation,
crash/kill/hot recovery, failed native I/O/lock/unmap/close/Sync/shortwrite/diskfull/
permissions, interrupted publication or rekey workflow was run or qualified.
Ordinary cooperative BUSY contention and pure error-tree classification are the
only lock-related negative checks. No commits, uploads or remote writes occurred.
