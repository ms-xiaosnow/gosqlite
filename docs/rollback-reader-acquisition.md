# Rollback Reader snapshot acquisition

`pager.OpenAdmitted` must establish SHARED before capturing the metadata used
for a read snapshot. Its preliminary descriptor stat rejects non-regular files;
that preliminary size and modification time cannot describe the later snapshot.
A cooperating writer can complete an ordinary commit between that stat and the
successful lock. Retaining the old metadata makes a valid newly committed
snapshot fail with `ErrChanged`, or supplies stale physical extent to header
parsing. The writable `OpenTx` path already refreshes metadata after SHARED.

The Reader now refreshes stat on the same owned descriptor immediately after
SHARED succeeds. It propagates a metadata error through the existing retirement
owner. The initial regular-file rejection, descriptor adoption, sidecar checks,
header validation, pathname identity and subsequent snapshot checks remain in
place. `ErrChanged` has not been converted to BUSY or silently retried.

## Focused verification

`TestReaderAcquisitionSourceSnapshotAfterShared` parses the constructor source
and pins adoption, regular-file validation, lock, snapshot stat, error handling,
Reader/header use, stability checking and deferred retirement ordering. It fails
on the old constructor without exercising a physical race or a native fault.

`TestReaderAcquisitionOrdinaryCooperatingWriter` uses a fresh ordinary database,
128 schema-cookie commits, 32 intentional rollbacks and two independent readers.
Each reader checks a pinned valid page twice and checks its committed generation.
BUSY permits a bounded cooperative retry; joined unknown diagnostics fail. All
workers close normally and join before the final generation check. Its
20-second cooperative bound is an incomplete-work failure, not early success.
`TestReaderAcquisitionMemoryBusyClassification` checks these error decisions
using only in-memory values.

After reviewing the selected bodies and helpers, use the pinned local Go and
module caches, with no manual tags:

```sh
CGO_ENABLED=0 go run ./internal/testscope
CGO_ENABLED=0 go test ./internal/pager -run '^TestReaderAcquisition' -count=20
CGO_ENABLED=1 go test -race ./internal/pager -run '^TestReaderAcquisition' -count=5
```

The live test is scheduler-dependent coverage; a passing sample alone does not
prove absence of every timing race. The previously observed ordinary rollback
soak failure is preserved, even when subsequent bounded samples pass. The
separate fixed-workload transaction harness verifies exact SQL row histories,
normal savepoints/rollback/close/reopen and bounded cooperating readers/writer.
Those reports must identify the exact engine plus harness composition and retain
all failed attempts. No long-run, crash/recovery, native-fault, hostile namespace
or cross-platform execution acceptance follows from these focused checks.
