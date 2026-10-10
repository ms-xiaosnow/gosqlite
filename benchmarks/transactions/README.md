# Ordinary transactions and concurrent soak

A native, runnable fixed-workload measurement for goal M10 / §11.3. This harness
measures transaction throughput and actual elapsed concurrent activity. It is
not a durability/recovery qualification and does not replace release security,
format, license, native-platform, or other safety reviews.

## Reproduce

Use the project's pinned Go 1.27.2 and local module/build caches. Production
builds use CGO_ENABLED=0; the race detector uses a separate CGO_ENABLED=1 build.
From the repository root, after reviewing the selected source behavior:

```sh
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
export CGO_ENABLED=0
go run ./internal/testscope
go test ./benchmarks/transactions -count=1
go vet ./benchmarks/transactions
go build -trimpath -o /tmp/gosqlite-transactions ./benchmarks/transactions
CGO_ENABLED=1 go test -race ./benchmarks/transactions -count=1

# Fill these with the exact frozen source identity and controlled host label.
SOURCE_ID='replace-with-tree-or-source-manifest-sha256'
HARDWARE_ID='replace-with-host-cpu-os-label'
for profile in rollback wal-shared encrypted-shared; do
  /tmp/gosqlite-transactions -mode throughput -profile "$profile" \
    -transactions 256 -rows 64 -readers 2 -batch 2 -seed 20261010 \
    -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID" \
    > "throughput-$profile.json"
  /tmp/gosqlite-transactions -mode soak -profile "$profile" \
    -duration 20s -rows 64 -readers 2 -batch 2 -seed 20261010 \
    -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID" \
    > "soak-smoke-$profile.json"
done
```

Use new output filenames and preserve each exit status, including nonzero ones.
A nonzero workload exit still emits the failure report after normal cleanup
attempts; flag-parser usage errors happen before a workload/report exists.
The command prints JSON on stdout and creates a new isolated temporary data
folder, whose exact path is in `retained_data_directory`. It never removes
results or database files. The caller may clean up a completed, closed fixture
later. Unit tests similarly retain their small native fixture directories.

Do not impose a process-killing shell timeout. Duration expires through
cooperative context cancellation; all worker goroutines join, Rows and Tx
resources close, and then DB handles close normally. An ordinary Ctrl-C is
handled through context cancellation and yields an incomplete/failed report.
No command in this harness sends a signal to any process.

Unexpected close/retirement diagnostics latch a failed result. A single owner
ledger tracks the writer and every reader, and only releases a handle after that
exact Close succeeds. A successful writer Close cannot erase a failed reader
Close. Failed handles and original error/token references remain retained through
report emission; no automatic retry is added, because a second nil Close from a
one-shot handle is not proof of retirement. JSON exposes close_failures,
unverified_close_handles and unverified_retirement. Unverified retirement is
never labeled workers_joined_normal_close or passed. Ordinary process completion
is not evidence that these obligations retired, and this command does not promise
recovery from terminal native uncertainty. These exceptional contracts are tested
only with descriptor-free memory objects and source checks, not native failures.

## Explicit profiles and scope

- `rollback`: normal DELETE-journal connections, independent readers and one
  writer; lock contention is reported. Checkpoint is explicitly skipped because
  that API is unsupported in rollback mode. Normal close/reopen still runs.
- `wal-shared`: native shared-memory WAL, with the conservative default storage
  assertion (WALPowersafeOverwrite=false); no invented power-safety assertion.
- `encrypted-shared`: correct-key EncryptionWALShared/layout 3, using a public
  synthetic benchmark-only RawKey256 provider. Every borrow is separately owned
  and cleared. This is not a secret or production credential, and no credential
  is supplied through SQL, DSN, environment, or command line. Ciphertext has fresh
  engine-generated salts, so ciphertext byte hashes are not reproducible.

Unknown profiles are rejected before database creation. If an otherwise valid
profile's initial Open is unsupported on the host, the report says
`skipped_unsupported` and exits nonzero. Later unexpected unsupported operations
fail the run. Exclusive WAL/encrypted modes are not relabeled as shared coverage.
There is no fallback to another profile or to an in-memory database.

All work is intact ordinary SQL, rollback/savepoints, normal checkpoint and
normal reopen. There is no process kill, forced disconnect, crash/hot recovery,
physical tamper/corruption/wrong-key test, selected-file removal, namespace
replacement/race, injected I/O/lock/unmap/close/Sync/shortwrite/disk-full/permission
failure, interrupted publication/rekey, or external database oracle. Normal
reopen and checkpoint latency are never called WAL recovery time.

## Workload and exact model

The version is `ordinary-transfer-history-v1`; seed defaults to 20261010.
The initial corpus has fixed row IDs and deterministic integer balances.
`dataset_sha256` hashes that corpus. `workload_sha256` hashes the version and
complete configuration except source/hardware labels, so two source versions
can be compared under identical inputs. Source/hardware labels remain in the
report. Changing a workload parameter changes its identity.

There is one writer and 1–8 independent reader connections. Each writer attempt
uses a deterministic seed/ordinal transfer sequence and 1–16 pairs of updates.
Each epoch retains its first writer ordinal and a bounded one-byte-per-attempt
terminal-outcome trace: C committed, R intentional full rollback, B BUSY,
X canceled, F failed. The final model generation and SHA256 are retained even
for failed runs. Together with seed and deterministic ordinal mixing, the C
entries identify the successful transfer history without an unbounded event
log. Scheduling and BUSY outcomes themselves are naturally nondeterministic.

Attempts cycle through:

1. An ordinary full rollback, followed immediately by a complete unchanged-model
   check on a new read transaction.
2. An outer transfer, an inner savepoint transfer, ROLLBACK TO and RELEASE;
   validate the restored overlay, then commit only the outer change.
3. Outer and inner transfers with RELEASE; validate and commit both.
4. An ordinary transfer and commit.

Every committed transaction updates a generation row atomically with account
rows. A synchronized immutable model records exactly successful commits. Each
reader pins a transaction, reads its generation, checks every row and ID against
that generation's model, holds the snapshot for the configured period, and
checks every row again. Sum conservation alone would not detect a permutation;
the oracle compares each row value. Failed/BUSY/canceled attempts do not update
the model. BUSY commit retries are bounded (20 retries after the initial try),
counted separately and wait cooperatively. Cleanup/I/O errors cannot be masked
as BUSY or cancellation. Classification examines every branch in at most 128
error nodes, accepting only exact known native/pager/encrypted/WAL-index BUSY
leaves or context cancellation/deadline leaves through standard fmt/errors.Join
wrappers and consistent SQLite code envelopes. Unknown leaves/wrappers, extra
diagnostics, unsafe codes, typed nils, cycles and oversized graphs fail closed.
Custom Is/As/Error/Unwrap callbacks are not executed by classification. Mixed
known BUSY and cancellation is canceled and cannot enter commit retry. One writer is intentional, not a multi-writer claim.

At most 64 writer attempts or 5 seconds form an epoch. Cancellation joins every
worker and closes reader handles. The writer performs a normal checkpoint when
supported, closes, reopens the same intact file/family, and checks the entire
latest model. Only then is history pruned. There is a final verified reopen even
when the requested duration expires. No SQL growth is hidden by recreating the
database between epochs.

Pass requires the requested number of attempts or actual accumulated active
seconds, correct outcome accounting, at least one committed transaction,
successful reader, intentional rollback, committed savepoint rollback and
committed savepoint release, plus a final exact-model check and normal closure.
A very small or starved run may fail required coverage; that is not waived.

## Bounds and timing interpretation

Rows are 4–4096; batch is bounded by rows/2 and 16; there are at most nine worker
goroutines, plus Go runtime/timer/signal support. Each epoch holds at most 65
model versions, and reports at most 4096 epochs. SQL sort memory is 1 MiB;
transaction overlays are 128 pages; undo is 8 MiB. Histograms use 48 fixed
buckets rather than storing unbounded latency samples. Go has a 256 MiB **soft**
memory limit, and heap is sampled and guarded at epoch boundaries; this is not
a hard RSS limit or a continuously sampled peak memory claim. Reports state the
maximum *observed* heap, not a true peak.

Logical row count stays fixed. A configurable 1 MiB–1 GiB apparent-file-byte
ceiling is checked at epoch boundaries. Plaintext can overshoot by one bounded
epoch. Encrypted storage also receives that retained-bytes quota, 100000
retained generations, 200000 directory entries, 2048 logical pages/8 MiB logical
bytes, 128 active segments and a 64 MiB adapter memory quota. Immutable encrypted
history grows legitimately; reaching any quota fails rather than silently
shortening the measurement. There is no disk-full fault injection.

- `elapsed_seconds` includes bootstrap, active work, ordinary checkpoint/reopen,
  verification and final normal close. UTC start/end are actual wall clocks;
  duration arithmetic uses Go monotonic clocks.
- `active_seconds` sums concurrent epoch wall times including cooperative drain;
  it excludes bootstrap and between-epoch checkpoint/reopen/verification.
- Throughput reports committed transactions and writer attempts per active
  second, plus committed transactions per end-to-end second. Intentional
  rollbacks are successful outcomes but not committed transactions.
- Latency includes the full attempt, its exact-model checking, any commit retry
  wait, and cleanup, with all BUSY/canceled/failed attempts included. These are
  workload transaction latencies, not isolated commit syscall latencies.
- P50/P95 are explicitly logarithmic histogram **upper bounds** in milliseconds.
  Exact min/max/mean/sample standard deviation and all histogram counts are
  retained. Bucket i has upper bound 2^i microseconds. This approximation is
  suitable for a bounded long run, not claims of tiny latency improvements.
- Data is warm after initialization; no OS-cache eviction is performed. Host
  sharing, process scheduling, filesystem/cache conditions and simultaneous
  workloads must be recorded externally. A host label is not hardware isolation.

## Predeclared longer release proposal (not yet executed)

The goal gives no numerical definition of “long” or a required hour count. The
following is a proposed, reproducible release profile, not a newly invented
acceptance threshold and not evidence that a long run passed:

```sh
for profile in rollback wal-shared encrypted-shared; do
  /tmp/gosqlite-transactions -mode soak -profile "$profile" -duration 2h \
    -rows 256 -readers 4 -batch 4 -seed 20261010 -writer-pause 250ms \
    -reader-pause 2ms -snapshot-hold 2ms -max-data-bytes 1073741824 \
    -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID" \
    > "release-soak-2h-$profile.json"
done
```

Two hours of actual concurrent activity per profile gives repeated transaction,
snapshot, checkpoint and reopen lifetimes beyond a smoke run. Four readers and
256 modeled rows exercise retained snapshots while 250ms writer pacing bounds
ciphertext generation rate and leaves ample margin below the epoch cap.
A preliminary 20.0016-active-second run of exactly this encrypted cadence
recorded 51 commits, four verified checkpoint/reopen cycles, 920652 retained
bytes, 117 observed files and encrypted commit sequence 57. Straight-line
projection to two hours is about 332 MB, 42120 files and sequence 20520, below
the declared 1 GiB / 200000-entry / 100000-generation ceilings. These projections
are capacity planning, not measurements or a guarantee of sustained throughput.
With 250ms pacing there are approximately 28800 attempts at most plus bounded
epoch-cancellation edges; only three quarters are planned commits. This leaves
substantial entry/generation headroom without changing or rotating the family.
The fixed logical row count limits per-generation growth, but increased
directory-scan cost or a quota can still stop the run. Any such result remains
failed/incomplete evidence and motivates a separately declared profile change.
Run sequentially on an otherwise controlled host; record filesystem, CPU,
Go/runtime build, source identity and all raw reports. Reviewer acceptance of
this chosen observation window is separate from the original qualitative goal.

For unpaced transaction throughput, separately predeclare 10000 attempts,
1024 rows, four readers, batch four, the same seed, and the 1 GiB ceiling.
Do not compare this to paced-soak throughput or a different row count.

## Future CI confidence policy

CI wiring is deliberately outside this change. A future comparator should first
require passed status, all invariant/coverage gates, matching workload/dataset
hashes, same declared host/OS/filesystem/cache/page/Go settings and independent
raw runs. Use at least five interleaved baseline/candidate repetitions on the
same controlled host, predeclare the metric/regression threshold, and report
paired uncertainty or a justified independent-run interval. Preserve failed
runs and disclose exclusions rather than selecting only fast runs.

`window_commit_rate_statistics` exposes sample count, mean, sample standard
deviation and null 95% bounds with an explicit confidence-method explanation.
Epoch rates are correlated and unequal-duration; their descriptive mean is
not the overall weighted throughput. No normal-theory confidence interval is
invented from transaction counts or correlated within-run windows. No SQLite,
pg_bigm or pgvector comparison is fabricated.

## Correction provenance and V4 admission

V1/V2 reports retain their original identities. Independent pure-memory review
found that V2's errors.Is/errors.As classification could hide extra joined
diagnostics, and source review found incomplete all-handle closure reporting.
V3 corrected the classifier but its ownership reporting still missed public
pending-retirement markers and failed-Open paths. V4 recognizes every public
cleanup-pending marker, conservatively latches SQLite CodeIOErr, poison/commit
uncertainty and opaque ownership envelopes, and installs finalization before the
first database Open. Original token graphs remain retained even when no DB handle
was returned. Unsafe failure cannot become an unsupported skip or a later pass.
Its diagnostic discovery and rendering are bounded and do not invoke unknown
error callbacks. Only a wholly understood unsupported graph can be skipped.

Every V4 JSON report includes harness_revision="ordinary-transactions-v4". A
performance comparator must require this exact revision; matching workload math,
schema_version or zero closure fields in V1–V3 is insufficient corrected-report
evidence. The workload math is unchanged. V4 bounded runtime checks use the
separately accepted ordinary reader-acquisition fix composition. All original
failed controls remain failures; no native failure path was executed.
