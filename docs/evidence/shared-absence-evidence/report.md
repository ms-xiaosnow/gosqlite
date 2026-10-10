# Shared-WAL initial-absence correction: frozen handoff

## Result

The four scoped absence decisions now use explicit initial no-handle provenance.
Production changes are limited to internal/pager/shared_session.go and
internal/pager/plain_session_cleanup.go. No public struct/API, backend ownership,
retirement protocol, shared WAL inode handling, or SQL change is included.

Frozen source: ../shared-absence-freeze-v1
Base: ../grouped-counters-final-verify
Base Git tree: 13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33
Final Git tree: b983d28c1e217d121bf5cda0ff92c6e85accef41
Patch: source.patch
Patch SHA-256: a4aa67ed7b2764726b059d0f18a6ce6fe2b8f2ea3d0663968b0137f6310c7770

Six owned paths are recorded in owned-paths.txt and owned.json. The progress
change is an appended section; the lead can reconcile that section with parallel
documentation updates without overwriting them. main/commits were not modified.

## Source audit

- Shared creation accepts resolveNativeReaderPath's initialAbsent, never a later
  resolver error merely matching NotExist. Parent resolution still returns its
  full error, and the deferred failedOpen adopts error-carried children.
- Rollback-to-WAL preflight accepts only nativeLstatForCreate's absence output.
  An extant nonempty WAL remains Unsupported; any later observation/retirement
  error returns intact before mode transition.
- Live shared journal checks accept only the same explicit absence output;
  extant journal policy remains RecoveryRequired.
- Session.probePathLocked now forwards nativeLstatForCreate for both exclusive
  and nil-scope shared transition sessions. checkJournalLocked consumes it.
- enterWALLocked uses its descriptor-owning rollback transition. Tx.checkStable
  calls checkSidecars, already using explicit provenance; Commit preserves native
  errors. No exclusive WAL scanner, replacement, or SHM removal path is added.
- Existing failed-open reservation/adoption and successful-close transfer remain
  untouched. Metadata/nested obligations precede index, WAL, and protecting main.
  Windows metadata terminal ownership and Linux/Darwin path contracts are not
  reimplemented or weakened.

## Evidence

checks.sh / checks.log / checks.exit: exit 0 on the exact frozen source.
- Guard: go run ./internal/testscope, after selected bodies/helpers were audited.
- CGO0: new AST/descriptor-free absence laws, unchanged VFS/plain/shared cleanup
  laws, all native pager SharedWAL tests plus 65-session capacity lifecycle.
- CGO0: SQLite shared snapshots, writer contention, option validation, active
  Rows/Tx disposal/reopen; driver pooled writes and cancellation/disposal/reset.
- Scoped vet: pager, VFS, sqlite, driver.
- Separate CGO1 race: the selected memory and ordinary shared lifecycle controls
  across pager, VFS, sqlite, driver. All four packages pass.
- CGO0 build and pager test compile: Linux arm64, Darwin amd64/arm64, Windows amd64.
  These binaries were compiled only, not executed on this Linux host.

negative.sh / negative.log / negative.exit: script exit 0. On an isolated copy
of the immutable base with only the two new test files added, both source laws
FAIL as expected (test exit 1). They expose the three broad shared checks and the
nil-scope transition fallback. The new descriptor-free laws and unchanged memory
negative controls PASS on that same base. No old-base database aggregate ran.

replay.sh / replay.log / replay.exit: exit 0. An isolated clean base plus source.patch
replays to exact full bytes/modes and passes guard, new/unchanged memory laws,
selected native/public/driver shared snapshots, pool writes and ordinary disposal.

base-manifest.json/full-manifest.json record every path's SHA-256, Git blob ID,
Git mode and byte size; base/full-sha256.txt and base/full-modes.txt are convenient
plain lists. The base inventory exactly matches git ls-tree of the declared
immutable tree; independently computed whole tree hashes match. Of 1,322 base
files, 1,319 are byte/mode unchanged. Three files are added, yielding 1,325 final
files. verification.json records exact patch replay. postchecks-verification.json
rechecks frozen, base and replay source after all commands. unchanged-controls.json
records unchanged negative and ordinary control files individually.

## Safety and remaining limits

All exceptional behavior is source-only or descriptor-free memory/error/callback
behavior. Ordinary live files are created and changed solely through normal
SQLite transaction/session lifecycle. No physical missing selected file,
corruption/tamper/wrong key, hostile namespace change, crash/kill/abrupt exit/hot
recovery, native I/O/lock/unmap/close/Sync/short-write/disk-full/permission failure,
interrupted publication/rekey, or manual-tag workflow was executed anywhere.
No native failure qualification, non-Linux runtime acceptance, full aggregate,
new hosted CI pass, or broad M3/release acceptance is claimed.
