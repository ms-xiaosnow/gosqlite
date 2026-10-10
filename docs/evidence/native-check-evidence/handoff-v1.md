# Native supported-profile integrity check: frozen v1 handoff

## Result

Goal §12.2 now has a genuine native `gosqlite check <db>` outcome, backed by new
`DB.Check` and `Tx.Check`. It checks a complete declared profile rather than
mistaking an official SQLite oracle or structural traversal for ordinary-index
consistency. All APIs use one pinned logical reader throughout; ordinary indexes
are reconciled bidirectionally against source rowid and values. Full checking
also reconciles the supported grams/ANN families and rejects ownerless shadow
rows that a single named-index check intentionally does not examine.

- Frozen source: `native-check-freeze-v1`
- Frozen exact Git tree: `384f19e2e79cca04a9482ce4d8c78b298c07dd09`
- Composed baseline: `native-check-base-v1`, tree
  `0374382bdef1c61a7f5b36092a42109046b100dd`
- Baseline composition: requested rollback-reader-acquisition freeze tree
  `ad066d04d676b8368ca7d02d83664e50dfa47216`, checked-apply reviewed
  `cli-backup-rekey-evidence/source-v2.patch`, SHA-256
  `c7893f8a1c6f686cf137cca15dbfa4cdf670e986625f2bf863e01caf80e8c359`
- Minimal owned reconstruction patch: `native-check-evidence/source-v1.patch`
- Patch SHA-256:
  `408afede0a32bebc28aee9089e9bf9eeebdfe7d118338f927980257ef2f0d503`
- Complete specification: frozen `docs/native-check.md`

No main/remote write, commit, push, upload or dependency download occurred.
The patch assumes the recorded composed baseline; the reviewed adapter dependency
is not silently included in this eight-path delta.

## Ownership and exact replay

1,330 baseline files become 1,337 frozen files. Seven additions and one narrow
existing CLI dispatcher/help change. Eight owned paths:

1. `sqlite/check.go`: additive public check API/types and native full-profile logic
2. `sqlite/check_internal_test.go`: detached memory negatives and native ownership/counter tests
3. `sqlite/check_test.go`: intact ordinary/API/encryption/official fixture checks
4. `cmd/gosqlite/check.go`: conservative CLI parser, native call and result handling
5. `cmd/gosqlite/check_test.go`: full/named/inspect/provider/readonly boundary tests
6. `cmd/gosqlite/main.go`: help and check dispatcher only
7. `docs/native-check.md`: actual scope, limits, examples and remaining gates
8. `examples/native-check/main.go`: real executable versus native API proof

Every other path's bytes and modes are unchanged. In particular, all pager files,
public Options/Result, connection counters, native storage/owner contracts,
reviewed maintenance/provider adapter, original goal, compatibility/progress
and central test-scope guard remain untouched.

`identity-v1.json`, `base-inventory.json`, `freeze-v1-inventory.json`, portable
SHA lists, `owned-paths.txt` and `owned-inventory.json` record full path/mode/hash
identity. `source-v1.patch` passed whitespace/error and clean apply checks.
`native-check-patch-verify-v1` reproduces the exact full frozen inventory/modes.
`freeze.py` records the reconstruction steps. Post-run source readback is recorded
in `final-verification-v1.json`.

## Semantics and boundaries

- New `CheckOptions` caps logical page ownership (default/max 1,048,576) and shared
  cumulative PageReader calls (default 16,777,216; max 2^32). Existing separate
  record/schema/cursor/MaxSortBytes/extension limits remain; no aggregate heap
  limit or universal wall-time bound is claimed.
- Zero CheckResult on every failure, including owned-reader close errors.
  Unsupported features fail closed rather than returning a success with exclusions.
- Stable versioned scope and counts cover all logical pages, freelist trunks,
  reserved lock-byte page, ordinary/shadow tables and indexes, logical extension
  definitions, rows, index entries and actual logical read calls. Table counts
  exclude sqlite_schema; it is still fully checked.
- Full catalog safety profile is rowid/unreserved/schema4/UTF-8, supported column
  defaults/NOT NULL/IPK/types, nonunique BINARY ASC ordinary indexes and supported
  grams13/dense/typed HNSW/IVF. Unsupported SQLite grammar/constraints/objects,
  custom index methods and invalid UTF-8 TEXT fail explicitly. This is narrower
  than permissive read planning around unsupported peer indexes.
- Full check reuses source/table-value and strict typed interpretation rules.
  Source-to-index SeekIndex and index-to-source SeekTable/value comparison are
  both mandatory. Page ownership, overflow, encoding/record/key validation,
  ordering and freelist accounting are additional independent passes.
- Existing per-extension same-reader helpers check grams source/postings/frequency,
  HNSW state/live/vector/edge/count invariants and IVF source/state/memberships.
  The new global shadow-owner guard prevents orphan/cross-method rows escaping
  these per-index checks. No ANN recall/performance/topology optimality claim.
- No writer promotion, journal, repair, checkpoint, raw file copy or external SQL
  delegation is introduced by Check. Active transaction snapshots/Rows and SQL
  counters retain ownership; deferred Tx first access follows the existing read
  path. Cancellation/quota failure does not roll back a caller-owned transaction.
- Full CLI rollback and encrypted snapshot/exclusive-WAL/shared-WAL opens are
  genuinely read-only/no-create and emit ok JSON only after DB.Close. Encryption
  uses the reviewed explicit env/stdin adapter; no raw keys/DSN/argv/log values.
- Full plaintext WAL CLI remains **an implementation gap**: leading WAL flags
  without a named extension reject before opening, because existing native
  openers require writable coordination and can convert/init files. Native
  DB/Tx.Check works on already configured pinned WAL sessions without acquiring
  or promoting a writer. Parent explicitly approved this conservative boundary.
- Existing named extension syntax and scope, including historical writable WAL
  coordination flags, remain. Inspect remains metadata inspection.

## Exact frozen verification

The executable command record is `checks-v1.sh`, with a `.command`, `.txt` and
`.exit` file for each gate. Every exit is zero:

- Pinned Go1.27.2 and official SQLite3.54.0 source-ID verification
- `go run ./internal/testscope` before both normal and race selections
- New native/API/CLI tests plus existing version, ordinary query/inspect and named
  grams checks: sqlite 2.285s; cmd/gosqlite 0.056s
- Same focused `-race` selection: sqlite 10.500s; cmd/gosqlite 1.534s
- Vet: sqlite, cmd/gosqlite and examples/native-check
- CGO0 native executable build with -trimpath
- Real binary proof: plaintext and correct-key encrypted snapshot/WAL/shared;
  complete native API/CLI results equal, family byte/name hashes unchanged
- Production `go list -deps` has no CGO package
- `go mod verify`: all modules verified

All commands used the pinned sibling toolchain, offline module settings and the
existing local caches, with CGO0 except the explicit CGO1 race gate. Broad
old-baseline aggregate tests and manual tags were not run. The initial Go-cache
cleanup was performed separately by the lead after all workers confirmed idle;
no source/evidence/fixture/DB was removed by this worker.

The real executable proof reports identical counts in all four modes:
10 pages, 5 tables, 4 physical indexes, 1 logical extension index, 232 table rows,
231 index entries and 950 logical page reads. Those rows include shadow data.
The CLI proof prints only mode and counts, never credentials/decrypted values.

`test-audit-v1.md` records pre-run review of all new bodies/helpers and selected
legacy helpers. `development-history.md` and development outputs preserve the
initial test-only compile/counter mistakes and corrected runs; no assertion was
weakened to hide a storage failure.

## Unexecuted gates and integration notes

No physical corruption/tamper/wrong-key/missing-selected-file, selected-file
removal/namespace mutation, crash/kill/abrupt DB exit/hot recovery,
native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permission failure,
interrupted publication/rekey or manual-tag case ran or was moved into CI.
Normal context cancellation/quota failures and in-memory malformed fixtures are
separate. No Darwin/Windows native runtime, crash/durability/security/release
acceptance follows from these Linux ordinary checks.

Independent review is still the next gate. Parent owns any integration, central
README/compatibility/progress updates and aggregate acceptance for the eventual
combined tree. This worker deliberately kept those central documents unchanged
to maintain a minimal owned delta. The general SQLite integrity_check parity
and native read-only plaintext-WAL opener gaps must remain explicit.
