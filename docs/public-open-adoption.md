# Public plaintext metadata ownership adoption

Date: 2026-10-10. This explicit delta extends the previously accepted ReaderV5
pager-only boundary and exclusive V2 public boundary to every plaintext public
OpenContext canonicalization prelude. It does not change SQLite bytes, shared
WAL algorithms, encrypted protocols, or existing public struct layouts.

## Root transfer and error replacement

Public rollback and exclusive opens reserve one of the same 1024 plaintext
roots before canonicalOpenPath. The admission transfers once into the Reader
probe or exclusive Session. Healthy idle rollback DBs still retain no root once
their probe retires. Ordinary initial absence consumes the probe root before a
new OpenTx obtains its own existing root. Direct pager constructors retain their
own pre-acquisition admission. The earlier exclusive-only internal admission
entry point remains as an alias/wrapper around the generalized implementation.

An early public error is adopted by Finish before an unused root can disappear.
Its handled result replaces the old error, so an overbound/cyclic original is
never rejoined after opaque bounded adoption. Pending metadata owners remain
reachable through PendingPlaintextCleanups/RetryPlaintextCleanup even when no DB
was returned or database/sql discards the error. Healthy opening admissions are
excluded from pending snapshots but count toward the common bound.

Shared OpenContext already reserved its 1024-session CloseAdmission before
canonicalization, but Cancel previously abandoned that empty-looking group
without adopting error-carried native metadata obligations. The new Finish path
adopts those early errors with bounded 4096-node discovery into that same reserved
close group. Its existing retirement order/status law and immutable snapshots
provide PendingCloseCleanups/RetryCloseCleanup for this public prelude. Pager
construction still transfers the admission and retains the separate 64-group
OpenCleanup route. Existing Session Close, Cancel for known-unused reservations,
and failed-pager-open state machines are unchanged; no extra native probe occurs.

Both admissions have one-shot transfer/Finish arbitration. Neither root keeps a
DB, Session, Reader, Tx, Rows, options, page data or callback. Global admission
and snapshot locks do no native work or resource status waiting. Terminal native
ownership remains terminal; no raw numeric handle is retried. No sweep, finalizer
or eviction was introduced.

## Scope and evidence

Windows encrypted public canonicalization was source-audited and left unchanged:
CanonicalWindowsPath opens directories through encryptedCleanups reservation,
scopes metadata children under those roots, and remembers failures before managed
Close. Its pending metadata ownership therefore already reaches the encrypted
public pending/retry APIs. Non-Windows encrypted canonicalization semantics also
remain unchanged.

The shared/recovery direct absence guards still rely on legacy os.IsNotExist
versus joined-error shape, rather than the explicit provenance APIs. That distinct
source coupling is not silently included here. No current Windows post-inspection
create bug was demonstrated for those os.IsNotExist callers.

Evidence is under evidence/public-open-adoption. Exceptional outcomes use only
descriptor-free admission/owner/error/callback laws. Native checks use intact
ordinary create/read/write/rollback/BUSY/cancel/close/reopen/checkpoint and pinned
official normal WAL interoperability. Darwin/Windows checks are compile-only.
Physical corruption/tamper/wrong-key/missing-file or namespace fault injection,
crash/kill/process-exit/hot recovery, failed native I/O/lock/unmap/close/Sync/
shortwrite/diskfull/permissions and interrupted publication/rekey remain NOT_RUN.
This is not full durability/security or release qualification.
