# ADR 0014: Managed native files before additional OS backends

Date: 2026-10-10. Status: accepted for the Linux ownership extraction only.

## Decision

The plain pager and WAL-index now own `internal/vfs.File` leases instead of
exposing `*os.File` across their boundary. This is an integrated native I/O
layer, not a second transaction engine. It is used by read-only header probes,
rollback transactions and journals, exclusive/shared WAL, SHM, imported-WAL
candidates, directory synchronization, and source-only recovery primitives.
Encrypted container storage is separate and unchanged.

A lease has a private descriptor, a file role, stable file/parent identity
observations, lock state, and its live mappings. Strong lease retention lasts
until explicit close; garbage collection cannot close a descriptor underneath
native locks or a retained view. No raw-handle or raw-file accessor is exposed.
The registry currently retains resources only. It is **not** a POSIX inode-lock
aggregator and does not make Darwin safe automatically.

Linux keeps descriptor-local OFD locks, with SQLite database PENDING at
0x40000000, RESERVED at 0x40000001, and the 510-byte SHARED range beginning at
0x40000002. SQLite SHM slots remain bytes 120–127; DMS remains byte 128.
Unsuccessful EXCLUSIVE acquisition releases temporary PENDING and retains
SHARED. Cleanup errors are not reported as safely retryable contention.

The portable SHM API accepts an unlocked slot or a same-mode repeat. It rejects
held-mode conversion before any syscall; callers release before changing mode.
Linux's atomic OFD conversion and arbitrary subrange/union unlock behavior
remain tested directly at the Linux backend. Real WAL-index read marks,
per-connection snapshots, publication atomics, dual-header ordering, and
WAL/main/directory synchronization order remain owned by existing algorithms.

Each shared `Mapping` retains the original native view and exposes exactly the
requested logical slice. Offset arithmetic distinguishes logical offsets from
native alignment, including a 32 KiB span with 64 KiB allocation granularity.
Mappings cannot extend files implicitly. Live views stay at stable addresses.
Close retires mappings before the descriptor/DMS hold; an unresolved unmap
retains the owner and refuses further I/O. Shared-session cleanup also retains
its database SHARED and WAL leases until index retirement succeeds. These
exceptional branches were reviewed and modeled in memory only.

Imported-WAL normalization uses a managed exclusive-create candidate and a
role-checked replacement operation. Replacement requires the main database's
EXCLUSIVE owner and explicitly replaceable WAL leases; shared-WAL leases do not
have that permission. The pager still syncs the candidate, replaces the name,
syncs the directory, adopts the complete snapshot, and closes the old lease.
No delete-then-rename or copy-and-delete fallback is introduced.

## Evidence

`docs/evidence/native-ownership-components.txt` records scoped ordinary Linux
unit/race/vet tests, the pinned official SQLite transaction/lock oracle, and
compile-only target checks. The new cooperating-process probe regression shows
that closing a managed read-only probe does not release a sibling writer's
reservation. Existing native-index hash blocks, pinned snapshots, checkpoint
limits, and concurrent header publication tests still pass.

The test oracle remains test-only. Production checks use `CGO_ENABLED=0` and the
pinned offline Go toolchain. Race tooling uses a separate `CGO_ENABLED=1` run.
Source-only recovery tests are compiled, never executed. No disk fault,
corruption, namespace replacement, killed client, or interrupted recovery test
is part of this evidence.

## Missing implementation, distinct from missing native tests

Darwin and Windows still select an explicit unsupported backend. A successful
cross-build is not working file-backed database support on either OS.

Darwin still requires an inode-keyed process registry covering registration,
failed-open cleanup, descriptor reuse, aggregate local-owner conflict checks,
and deferred physical closes while any process lock remains. DMS initialization
must be once per local SHM inode; mmap must respect the runtime page size.
Native handle identity/open policy and the declared fsync/full-flush policy must
be implemented before enabling that backend. The present Linux stat-based
parent checks are observations, not an anchored-directory opening guarantee.

Windows still requires reviewed handle metadata and native path/open/share
policy, local-filesystem restrictions, exact LockFileEx acquisition/unlock
records, safe SHARED/EXCLUSIVE restoration, one-byte SHM slot acquisitions with
partial-acquisition rollback, and native DMS lifecycle coordination. Mapping
objects must own allocation-granularity-aligned views and mapping handles.
Replaceable exclusive-session WAL leases need native same-volume replacement;
ordinary shared-WAL handles retain no-delete-sharing policy. Regular-file and
namespace synchronization guarantees must be declared explicitly.

Only after those implementations exist can actual Darwin/Windows native
ordinary lock/mapping/process/SQLite checks run on approved native hosts. Linux
cross-builds and memory models do not satisfy those gates. M3 interruption and
physical durability, prohibited recovery/fault tests, long soak, and independent
security/release reviews remain separate unfinished acceptance gates.

## Review correction and exceptional lifecycle boundary

Independent source review found that uncertain lock cleanup could return
ErrPoisoned without latching the managed lease. In particular a partially
completed EXCLUSIVE-to-SHARED downgrade could leave a stale cached EXCLUSIVE
bit usable by a later call. All native lock results now latch ErrPoisoned before
return, failed SHM unlock poisons, and first-opener DMS errors after acquiring
its write lock are marked poisoned. Pure-memory ownership regressions exercise
state handling without producing native lock/unmap/close failures.

Normal SharedSession.Close retains its index, WAL and main ownership on failed
view retirement and may be retried. Failed OpenSharedSession cleanup also
retains protecting resources safely, but returns no reachable session to retry;
those exceptional resources can remain retained for the process lifetime. This
is an explicit fail-closed availability/resource lifecycle gap, not universal
retryable cleanup. A future bounded, synchronous pending-retirement group must
retain index-before-WAL-before-main ordering and never evict live resources.
No OS failure execution or resolution of that gap is claimed here.
