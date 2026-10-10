# Shared-WAL failed-open cleanup

Date: 2026-10-10. Status: implemented source and focused ordinary/memory checks;
real native teardown-failure behavior remains unqualified. This is a foundational
M3 lifecycle correction, not full M3 or release acceptance.

## Ownership and ordering

`OpenSharedSession` reserves one of 64 strongly retained groups before its first
native metadata probe or file acquisition. The bound covers in-flight opens and
failed opens still awaiting retirement, not successfully opened sessions. A full
registry returns the existing pager limit classification (public `ErrMemoryLimit`,
`CodeNoMem`) before the pager performs native work. Admission does not attempt to
clean another group, evict an owner, or wait for any database lock.

Each successful acquisition immediately joins the reservation: main database,
WAL, then WAL index. A successful open transfers all three to its session and
releases the reservation. An unsuccessful open attempts synchronous retirement
in dependency order. First it collects all error-carried nested obligations,
including constructors which returned nil instead of a File/Index, and retires
them before their protectors. Then:

1. `walindex.Index.Close`, which delegates all views/mapping objects and the DMS
   lock to its managed file owner. No WAL or main close occurs until this returns
   success, so main SHARED protection cannot disappear below a retained view.
2. Managed WAL file close. Main protection remains while that owner is unresolved.
3. Managed main database file close.

Only a successful owner close clears its group slot. A close error by itself is
never interpreted as proof of a live retryable handle or proof of retirement.
The existing managed owner decides whether to retry represented live state,
report completed retirement, or keep a terminal tombstone. This layer never
extracts a native descriptor or retries a native close directly. Linux's
already-consumed `os.File` is not closed a second time; Darwin's shared descriptor
retirement remains governed by its process-inode owner. Windows mapping-object
close uncertainty exposed a separate backend source defect during this review.
The independently frozen [mapping-retirement correction](windows-mapping-retirement.md)
now latches that terminal state and passed source, memory/race and compile review.
It is a separate increment, outside this cleanup source snapshot; no native
Windows teardown-failure behavior was executed or qualified.

A pending group is retained without expiration or eviction. It has no finalizer,
background goroutine, pathname lookup, sweep, checkpoint, or publication action.
The registry mutex covers only admission/removal; it never spans cleanup or a
native operation. A group mutex serializes retries for that same group only.

## Public API and errors

`sqlite.ErrOpenCleanupPending` identifies the historical unsuccessful opening
error. That error also matches `sqlite.ErrPoisoned`; all original opening and
cleanup causes remain available through `errors.Is` and `errors.As`. The opening
error has `CodeIOErr`, even when the original opening cause was BUSY. It must not
be treated as safe retryable lock contention.

Call `sqlite.RetryOpenCleanup(openErr)` to deliberately attempt retirement for
every distinct failed-open group reachable through ordinary wrapped/joined errors.
Retain the original errors until this is resolved.

- Nil, or an error with no retained group, is a no-op returning nil. This is not
  a general assertion that every subsystem's resources are retired.
- Ordinary `%w` wrappers and `errors.Join` preserve every group's retry route.
  Discovery and pointer deduplication finish before any cleanup. Each distinct
  group is then attempted independently; a completed first group cannot hide a
  later pending group, and duplicates are not retried within that call.
- Error-tree discovery is limited to 4096 nodes, including custom cyclic wrappers.
  Over-budget input returns `ErrMemoryLimit`/`CodeNoMem` without touching any owner.
  This is an explicit exception to the reusable pending-error route: the refusal
  cannot safely unwrap an unbounded/cyclic input. Keep original opening errors
  and retry smaller trees. Recognized group tokens are discovery leaves because
  their native causes have already been adopted.
- Nil means every discovered group's retirement completed. It does not open a database,
  replay a transaction, fix the original failure, or promise a future open works.
- An ordinary pending result joins the current failures, matches
  `ErrOpenCleanupPending` and `ErrPoisoned`, retains the original causes, and can
  itself be passed for another deliberate attempt. Completed groups disappear
  from that result. This pager does not accumulate retry history. The separate
  [Darwin poison-retention correction](evidence/darwin-poison-retention-delta.txt)
  now keeps only its first stored poison graph and returns first plus current
  causes without retaining later joins. Its isolated memory/race review passed;
  it is outside this cleanup snapshot and is not native-failure qualification.
- Concurrent retries are safe and serialize on this group's mutex. After success,
  another retry is an idempotent no-op and does not repeat any resource operation.
- Original error values remain immutable, so their historical pending marker
  remains after cleanup. The return from the current retry gives its outcome.
- Some terminal ownership states cannot be resolved by retry. Those groups and
  their remaining protection may stay retained for process lifetime. Discarding
  the error discards the caller's retry route, without evicting native ownership.

The common VFS now carries an unresolved managed File in a private cleanup
error whenever `File.Close` cannot finish. Existing VFS/index constructor close
paths preserve those errors, so a nil constructor result no longer loses the
obligation. `CleanupOwners` collects all distinct File/terminal identities and
all dependency edges before choosing child-before-parent retirement order. Two
wrappers for the same File can carry different children; none may be lost by
deduplication. An inconsistent dependency cycle introduces a leading terminal
barrier while retaining the concrete owners.

Pre-File poisoned native-open rejection (including Darwin quarantine) without a
safe managed retry is an opaque terminal obligation. Windows metadata close
uncertainty similarly carries its existing permanent tombstone marker; no numeric
handle is exposed or retried. Prior/global poison can be propagated without a new
resource, so a later rejected open can conservatively consume an empty group.
This availability cost is deliberate: the layer cannot prove that unrepresented
ownership retired, and never evicts it merely to restore admission.

The public path canonicalizer can perform separately bounded managed metadata
probes before entering the pager. This group starts at `OpenSharedSession`; it
does not take over resources acquired by that earlier public canonicalization.
Inside the pager, native probes and nested File/Index/journal/directory cleanup
errors carry obligations into the already-reserved group. No backend raw-open,
locking or mapping algorithm was changed by this worker.

## Driver propagation and separate gap

`sqlite.OpenContext` wraps the pager opening error without losing its cause;
`driver.Connector.Connect` returns a failed `sqlite.OpenContext` error unchanged.
Thus errors surfaced by a `database/sql` connection attempt retain this retry
route. No new driver API or driver-side retry loop is introduced.

A separate, pre-existing branch in `Connector.Connect` checks cancellation after
a successful `sqlite.OpenContext`, calls `db.Close`, and discards that close's
error. That branch has no bounded ownership transfer and can lose the session's
caller retry route if cleanup does not finish. It is a remaining code gap, not
native-failure evidence. It was source-reviewed only and is not changed here.
Whether to remove that redundant post-success cancellation branch or add a
bounded transfer needs a separate Connector/close-contract decision. The pinned
Go 1.27.2 Connector contract describes context as for dialing; its successful
synchronous `DB.conn` path accepts the returned connection without a second
context check. Thus returning a successfully acquired connection despite a late
cancellation is plausible, but is only source analysis here. General
`database/sql` disposal can also ignore close errors, and public `DB.Close`
currently marks itself closed before session close, making repeat calls no-ops;
those broader close/retention contracts need separate work.

## Verification and limits

`docs/evidence/shared-open-cleanup-components.txt` records exact scoped commands.
`shared-open-cleanup-source-sha256.txt` freezes the owned implementation/test/doc
paths; `shared-open-cleanup-owned-paths.txt` lists direct changes and the reviewed
ownership/error propagation dependencies.

The [independent cleanup review](evidence/shared-open-cleanup-independent-review.txt)
found no remaining blocker in the frozen scoped implementation. It checked an
isolated immutable base plus these cleanup paths, the separately frozen Windows
mapping correction, and the isolated Darwin poison package. It records exact
hashes and the additional exhaustive descriptor-free DAG law. The [independent
Windows report](evidence/windows-mapping-retirement-independent-review.txt) also
stands alone for its three production/test paths. These companion corrections
are not retroactively part of the original eleven-path cleanup snapshot. This
final documentation-only update changes dependency status; the nine cleanup
production/test hashes are unchanged, and earlier evidence remains historical.

New negative tests use private fake in-memory owners only: partial view retirement
keeps DMS/WAL/main, each stage stops at its unresolved dependency, terminal raw
close is attempted only once, capacity never evicts, success transfers without
closing, no-resource failure returns its original error, retry preserves causes,
parallel retry retires exactly once, nested constructor obligations retain main
and capacity, joined groups deduplicate and all retry, over-budget discovery has
no resource effects, and a blocked fake owner does not hold the global registry
mutex. VFS laws cover multiple/transitive child dependencies, identity deduplication,
terminal pre-File and metadata poison, and fail-closed dependency cycles. No production fault-injection hooks were added. An ordinary
regression keeps 65 successful shared sessions live and verifies committed reads,
proving the 64-group bound is not a live-session limit.

Focused public native/shared-WAL and correct-key encrypted/driver lifecycle checks
exercise ordinary create/read/write/rollback/close and cooperating concurrency.
They do not execute or qualify actual failed lock/unmap/close/Sync/short write,
permissions, disk-full, physical corruption/tamper/wrong-key, missing selected
files, namespace retargeting, forced exit, crash/hot recovery, or interrupted
publication/rekey. Those native failure/interruption acceptance gates remain
BLOCKED/NOT_RUN and are not relocated to another environment. Cross-builds are
compile evidence only; native Darwin/Windows runtime qualification, full M3
safety, soak and independent release review remain outstanding.
