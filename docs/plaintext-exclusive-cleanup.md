# Exclusive WAL managed retirement

Date: 2026-10-10. This third increment extends the accepted Reader and standalone
rollback-transaction owner scheme to WALExclusive Session construction, live
metadata/read failures, mode-transition journals, WAL creation/reopen,
uninterrupted normalization, checkpoint and disposal. Shared WAL and encryption
retain their existing protocols, bytes and separate cleanup APIs. Native failure
qualification and full durability/security acceptance remain NOT_RUN.

## Bounded ownership and dependency order

The common plaintext registry admits at most 1024 opening/live/pending roots.
An exclusive root is reserved before path resolution or native acquisition and
precharges three fixed direct slots: main, current WAL, and temporary. The
last holds at most one transition journal, normalization candidate or superseded
WAL. Explicit child reservation precedes each acquisition, and successful child
handles are adopted immediately before any observation. Native constructor errors
are adopted before any protector can close. Nested VFS discovery retains its
4096-node bound. Admission/snapshots never wait on native/status calls.

The owner keeps no Session, DB, Reader, Tx, Rows, options, page map, undo buffer or
application callback. Retirement order is nested obligations, temporary child,
current WAL, then protecting main. Status decides completion independently of
Close's diagnostic. Terminal managed resources never retry a numeric handle.
A live retry retires only failed nested or explicitly close-requested children;
it cannot close an active WAL/main or an unrequested journal/candidate. Ordinary
live/opening groups are excluded from pending snapshots but count toward capacity.

## Mode transition and normalization

The exclusive rollback-journal mode transition borrows its Session scope. It
uses the precharged temporary slot, adopts errors before journal/main protection
can drop, and logically ends its ephemeral Tx without closing the Session main.
Shared WAL's private nil-scope KeepFile branch is unchanged.

A normalization candidate is reserved/adopted before writing. On successful
replace, already-owned candidate/current-WAL slots are relabelled atomically
under the group lock, with the old WAL moved to temporary ownership. Its
retirement completes before the current WAL/main can retire. A failed child
close prevents unlink/removal and parent release. Pending constructor/journal
errors and ordinary diagnostics survive one-time closure and public wrapping.
Existing frame/checksum/sync/publication algorithms and on-disk formats are
unchanged. Exceptional branches were source-reviewed and model-tested only.

## Checkpoint, Close and public disposal

Checkpoint's current WAL close uses the managed slot. Incomplete retirement
holds main and prevents later unlink. A consumed close diagnostic still poisons
the operation; explicit retry never repeats checkpoint, namespace mutation,
transaction replay or synchronization.

CloseWithError accepts prior Rows/Tx disposal errors before protection drops.
An active lease returns BUSY without clearing options or logical state. Otherwise
Close attempts its optional checkpoint once only when there is no prior error,
poison or KeepWALOnClose option. It then invalidates logical Session resources,
clears options and snapshot, and begins ordered retirement. A just-failed child
close is not automatically attempted a second time by the same Close operation.
Repeated Close performs only explicit owner retirement. The DB Close bridge
invalidates Rows/Tx once and supports retry without rerunning their callbacks.
Errors discarded by database/sql remain discoverable in the independent pending
snapshot. RetryPlaintextCleanup/PendingPlaintextCleanups now explicitly cover
rollback Reader, standalone Tx and WALExclusive Session; WALShared/encryption
remain separately scoped. Existing public struct layouts are unchanged.

## Evidence and remaining limits

Evidence is under evidence/plain-exclusive-cleanup. Exceptional outcomes use
only descriptor-free fake owner/error/callback laws, including 256 ordered
four-stage graphs, first diagnostic retention, admission-before-path, active BUSY,
checkpoint-once, borrowed journal and candidate swap barriers. Actual native
checks use intact ordinary read/write/rollback/BUSY/cancel/close/reopen/checkpoint
and pinned official normal interoperability. Darwin/Windows checks are compile-only.

No physical database corruption/tamper/wrong key/missing selected file, namespace
replacement/retarget/race injection, crash/kill/process exit/hot recovery, failed
native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permissions or interrupted
publication/rekey was tested here or elsewhere. Legacy shared/recovery absence
checks still depend on current os.IsNotExist/joined-error behavior rather than
the explicit provenance variants. The Reader V5 report tracks that separate
source coupling; this increment does not silently alter those protocols.

## V2 public metadata admission

WALExclusive public OpenContext now reserves its plaintext root before public
canonicalization. An untransferred admission adopts any early canonicalization
or setup error into the same bounded owner/snapshot scheme. Its one-shot atomic
transfer gives that exact root to OpenSessionAdmitted; the public deferred Finish
cannot then close the live Session. Direct OpenSession still reserves its own
root before pager metadata. An unsafe graph is replaced by bounded opaque
adoption, rather than rejoined to the unbounded original at the public boundary.

This explicitly extends the earlier V1 pager-only admission boundary. ReaderV5's
public rollback canonicalization and the shared CloseAdmission Cancel path remain
separate actual discovery gaps: VFS metadata tombstones are strong, but those
public early errors do not enter their mode's public pending cleanup API.
Encrypted Windows canonicalization already opens directories through the
pre-acquisition encrypted registry and adopts child errors before Close; its
public pending/retry route therefore remains available for those failures.
