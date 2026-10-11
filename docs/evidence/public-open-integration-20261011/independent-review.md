# Independent review: public rollback Open integration

## Decision

**PASS for this exact source/model/compile slice. No blocking source defect found.**
This is not a physical recovery, default-off real-database regression, production
safety, release, full ordinary/oracle suite, or complete M3 acceptance result.
**All actual recovery and recovery-enabled real Open remain BLOCKED / NOT_RUN.**

Reviewed on 2026-10-11 UTC. The source audit preceded project execution. All 13
independent command stages naturally exited 0; the final race stage finished at
07:49:34.878759 UTC and the post-check identity completed at 07:49:35.178221 UTC.

## Exact identity and independent reproduction

- Baseline commit: `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`.
- Baseline tree independently confirmed from Git and fresh archive:
  `d735409ebd1c43d90ee1650c8a7c2a084e270674`, 2962 files.
- Frozen candidate: `public-open-integration-freeze-v1`.
- Before, after, and independently replayed tree:
  `75c074548e8b5208beb06e6ead314d835cd16f4f`, 2969 files.
- Patch SHA-256:
  `4285b5bae09bcdb515a484b777eeb858f8ad5572d3f808917a89a17dba68080f`.
- Exactly 16 owned paths match the supplied manifest. Every changed old file's
  before hash/size/mode and every final file's after hash/size/mode was checked.
  The independently calculated full inventories equal the supplied inventories.
- A fresh baseline archive was extracted outside the frozen source. Independent
  `git apply --check`, then `git apply`, reproduced the entire final inventory,
  including POSIX file modes, not merely the claimed Go changes.
- `goal.md`, `go.mod`, `go.sum`, and all unowned paths are unchanged.
- No file inside the frozen candidate was changed. Git archive/rev-parse only
  read the source repository; no index/object/commit/branch/remote/CI write was
  performed. Reproduction directories contain no `.git` metadata.

See `identity-summary.json`, the four full `identity-*.json` inventories,
`patch-check.json`, `patch-apply.json`, and reproducible `review_checks.py`.

## Source findings

### 1. Nil-default policy and options snapshot: PASS

`sqlite/rollback_recovery.go:40-66` returns immediately when disabled, so it adds
no new rejection to nil-policy configurations. Enabled readonly access returns
ErrReadOnly; encryption, all four native WAL-related flags, and non-Linux
platforms refuse before admission/path acquisition. Readonly access is never
upgraded. Readonly takes precedence when several prohibited settings coexist.

`sqlite/sqlite.go:225-229` snapshots the policy before connection limits,
plaintext admission, canonicalization and DB storage. The policy contains only
three scalar fields, so dereferencing into a fresh value isolates the stored
configuration. No concurrent caller mutation during construction is promised.

`driver/driver.go:58-70` applies URI restrictions first, then performs its own
scalar policy copy before returning a Connector. `mode=ro` is rejected by actual
Connect/Open if recovery is enabled; NewConnector itself does not perform that
policy rejection. `mode=rw` keeps NoCreate. Registered driver entry points still
construct zero-value Options, and parseDSNMode adds no recovery switch.

### 2. Bounds and fail-closed modes: PASS by source/model review

`RollbackRecoveryOptions.pagerOptions` copies all three fields. Existing
`journal.ReplayLimits.WithDefaults` independently defaults zero fields to 64 MiB,
65536 records and 1 TiB original extent. No connection sort/transaction/page
budget is reinterpreted or changed by this patch. Large caller-specified limits
are intentional caller policy, not a total process-memory guarantee.

Reviewed the unchanged physical implementation and parser as source only:
journal size is checked against policy and platform `int` before allocation;
the full replay parser bounds cumulative records by subtraction, checks extent
using uint64 geometry, requires a complete supported plan, and rejects malformed,
unsupported or ambiguous input. Recovery checks even empty WAL/SHM sidecars and
does not attempt broader recovery. No claim here verifies these physical effects.

### 3. Retired reader/path proof: PASS

`internal/pager/pager.go:33-91` still owns acquisition and unsuccessful-reader
cleanup. The new deferred helper first invokes the original cleanup `begin`
exactly once. All preexisting acquisition/SHARED snapshot/stability ordering is
retained. The mutable local path captured by the defer is the resolved path.

`internal/pager/reader_recovery.go:25-40` issues proof only for the exact private,
non-nil journal-presence pointer at that path, completed managed retirement, and
a result whose sole accepted leaf is that same pointer. Pending ownership,
consumed-close diagnostics, terminal/nested obligations, extra errors, wrapped
input, or mismatched paths cannot issue proof. A later explicit cleanup retry
does not retroactively authorize the old error.

`ReaderProbeRecoveryReady:48-50` uses direct private-type/path checks. It does
not call Is/As/Unwrap, perform native work, or accept joins/wrappers. Its typed-nil
and panic-on-traversal witnesses are genuinely memory-only. Presence is only
admission provenance, not proof that a journal is hot, valid, or authenticated.

### 4. One recovery, one fresh no-create probe: PASS by source/model review

`sqlite/sqlite.go:329-350` places recovery in a mutually exclusive branch before
ordinary initial-absence creation. A failed recovery/reprobe cannot fall through
to creation. The nil-option branch cannot reach recovery.

`sqlite/rollback_recovery.go:72-91` is the sole production bridge. It rechecks the
private proof, invokes RecoverRollback once and, only after successful recovery,
calls `pager.Open(path)` once. That call allocates a fresh root, opens read-only,
does not reuse the consumed admission, and cannot create. It closes the reader
before accepting metadata. Neither recursion nor implicit cleanup retry exists.

`PlainOpenAdmission.group.Swap(nil)` transfers the initial reservation once.
The reader failure retires that owner before proof exists; physical recovery
would independently reserve/revalidate, and the fresh reader reserves again.
The private proof itself is immutable rather than a consumed token; the exact
one-attempt bound comes from the private single callsite and sequencing law.

### 5. Cancellation, failure identity and metadata: PASS

`runRollbackRecovery:99-113` checks cancellation before recovery and before the
fresh probe. Recovery failure suppresses probing. Any failure joins the original
and later error, preserves ownership tokens, and returns zero metadata. Success
is not discarded by a late cancellation after the fresh probe has succeeded.

The existing `wrap` classification gives historical ErrRecoveryRequired
CodeIOErr precedence over BUSY/cancellation/limits. This remains true after a
successful physical recovery followed by cancellation or probe failure; it is
not a statement that the current disk still requires replay. A zero original
extent can likewise recover successfully but fail the ordinary empty-file probe.
Later SQL, Reader, and Tx operations have no new recovery entry point.

### 6. Default-off preservation: PASS for control flow and error contract

Nil policy preserves existing acquisition/creation/mode/budget behavior and
refuses every journal with `errors.Is(err, ErrRecoveryRequired)` and public
CodeIOErr. For non-presence errors the helper returns the original `begin`
result. No default recovery has been enabled.

**Intentional compatibility delta:** the journal error's text and private
wrapping type change even with nil policy. The previous text was
`SQLite database requires rollback-journal recovery: existing journal; recovery is not yet acceptance-verified`;
the new presence diagnostic says
`SQLite rollback journal exists; explicit recovery policy required`.
Therefore “unchanged” should describe behavior, sentinel and primary code, not
byte-for-byte diagnostics or error-chain shape. No existing tests rely on the
old text. This is informational, not a source blocker.

## Guard and execution-safety audit

Inspected all of `internal/testscope/main.go` and the three selected guard tests
before executing them. The command reads/parses test source and reports AST
references; it does not import or invoke database code. Every fixture in those
tests is an inert source string, never compiled as a database workflow.

The new guard detects named/selector references to physical recovery, captured
function values, enabled field references, and explicit nonempty unkeyed Options
through direct types, dot imports and same-file alias/defined-type chains.
Reverse order chains reach a fixed point; type cycles terminate. Existing
restricted-reference assertions are retained and six cases are added.

Known limits are accurately documented: inferred literal element types and
cross-file aliases are not fully resolved; scope-insensitive shadowing can
false-positive. This is prevention, not authorization or a complete call-graph
proof. None of the selected tests uses these gaps, reflection, hidden native
callbacks, or enabled real Options to reach recovery. The separate Rename/rename
selector and OS namespace gate work is absent and was not imported or tested.

The only production init found is driver registration, which opens no database;
there is no TestMain. Selected-package top-level initialization consists of
sentinels, empty bounded registries and inert API-compatibility literals.

Audited selected helper closures:

- Guard: inspectSource and standard parser/AST/string/formatting operations.
- Pager: local plainRetirementRegistry.reserve/snapshot; plainMemoryOwner status
  and Close; finishReaderProbe; begin/beginClass/adoptLocked/retireLocked/
  releaseLocked/retry; plainDiagnostic/plainSingleCause; bounded error discovery;
  RetryPlaintextCleanup only on that local model token. All installed owners are
  plainMemoryOwner, never a file/native owner or process-wide registry token.
  The source test reads/parses pager.go and reader_recovery.go without invoking
  their physical constructors.
- SQLite: validation, scalar limits conversion/defaults, checkContext,
  runRollbackRecovery with local callbacks, wrap/classifyEncryptedCleanup,
  AST/printer source helpers and option-copy order assertions. os.ErrNotExist is
  a synthetic sentinel; os.ReadDir/ParseFile inspect source files only.

## Exact regression scope and actual results

Go version: **go1.27.2 linux/amd64**, pinned absolute tool path.
GOTOOLCHAIN=local; GOPROXY=off; GOSUMDB=off; GOFLAGS=-buildvcs=false;
GOPATH=ROOT/go-path; GOCACHE=ROOT/go-cache; GOMAXPROCS=2; every Go
build/test/vet/run/list command uses -p=1. Default /tmp; GOTMPDIR unset.
Every command passed the original
>=1 GiB /tmp, >=2 GiB workspace, >=10000 available-inodes and writability checks.
No cleanup, cache relocation, threshold change, alternate machine or workaround.

Executed and PASS:

1. Pinned version; read-only gofmt -l on all 13 owned Go files (empty output).
2. Ordinary source-reference guard before tests.
3. Exactly these 11 top-level tests with CGO_ENABLED=0, -count=1, -v:
   - TestScopeSourceReferenceGuard
   - TestScopeRecoveryOptionTypeChains
   - TestIndependentReviewGuardDirectAndAliasLimit
   - TestReaderRecoveryMemoryRetiredPathProof
   - TestReaderRecoveryMemoryDiagnosticAndOwnershipRefusal
   - TestReaderRecoverySourceProofBoundary
   - TestReaderAcquisitionSourceSnapshotAfterShared
   - TestRollbackRecoveryMemoryPolicy
   - TestRollbackRecoveryMemorySingleAttemptSequence
   - TestRollbackRecoverySourcePublicAndDriverBoundaries
   - TestRollbackRecoveryMemoryFailureClassification
4. CGO0 vet of testscope, pager, sqlite and driver.
5. CGO0 `go build -p=1 ./...` (compilation only).
6. CGO0 production dependency listing with empty CgoFiles result.
7. Source guard again, then the identical 11 tests with CGO_ENABLED=1 and -race.

All selected tests matched and passed; no skips, failures or unmatched selectors.
The tests also execute 9 ownership-refusal and 11 sequence subcases. The policy
test enumerates 64 mode/platform combinations. Snapshot/driver integration is
source-inspected rather than exercised through recovery-enabled connections.
Exact argv, cwd, timestamps, CGO mode, resource readings and exit codes are in
`commands.json`; stdout/stderr is retained in the individual logs.

Only two baseline test files change: main_test.go adds assertions, while
reader_acquisition_test.go changes one expected defer spelling from cleanup.begin
to finishReaderProbe. Every previous ownership/order assertion remains. The new
proof-source test additionally requires begin-first cleanup and a call-free
checker. All other old tests retain their baseline bytes; no assertion was
weakened, removed or skipped.

No supplemental executable model cases were added or run, respecting the exact
existing-selector restriction. Nonblocking coverage opportunities for a future
authorized test-only change are mixed zero/nonzero limit defaults, explicit
DeadlineExceeded and limit-error classification, and expanded inert witnesses
documenting the guard's inferred-element/cross-file limitations. Those are not
new PASS claims and do not justify running physical operations.

## Remaining acceptance boundaries

The public physical harness is still absent; the existing internal manual
recovery harness does not exercise the new public bridge. Release/production
qualification therefore remains blocked on separately authorized, exact-fixture
physical evidence, plus whatever ordinary/oracle combined gates the lead selects.

No actual RecoverRollback, recoverRollbackOpen, recovery-enabled real
Open/OpenContext/Connect, real database fixture, native fault/crash/kill/forced
exit, tamper/wrong-key/namespace/missing-selected-file workflow, interrupted
publication/rekey, manual tag, OS runner, CI runner, ANN V4, 41-target fuzz, or
unknown interrupted chain was executed or resumed. There were no denied actions
and no attempts to route around a restriction. These statuses remain NOT_RUN.

All review artifacts are outside the frozen source. Main and remote remain
untouched by this reviewer; the lead's independent publication work is outside
this review's evidence and conclusions.
