# Windows read-only SHM backend groundwork

Status: **SOURCE_ONLY / NOT_RUN**. This increment does not enable public Windows
read-only WAL. The existing Linux-only gates in `sqlite/readonly_wal_policy.go`,
`internal/pager/shared_readonly_policy.go`, and
`internal/walindex/readonly_policy.go` are unchanged and continue to reject it.
No Go formatter, parser, test, build, native run, oracle, or qualification result
is claimed for these candidate bytes.

## Bounded backend change

`internal/vfs/readonly_shm_windows_policy.go` is a portable, descriptor-free
policy actually called by the Windows `nativeAttachDMS`. Its typed observations
are unknown, an existing local attachment, an acquired exclusive DMS probe, and
a contended exclusive probe. These are Windows handshake outcomes, not Unix
peer-lock states. Only the writable acquired-probe outcome may initialize.
Unknown and unrecognized values refuse with `ErrUnsupported`.

A successful read-only exclusive probe is cold input. The backend releases that
probe exactly once and returns its refusal joined with any cleanup error. A
failed release retains the exact held interval and its `ErrPoisoned` cause;
there is no truncate, later shared attempt or publication on that path. Poison
has precedence over BUSY when classifying the real probe result, and unrelated
native errors retain their original cause. A local attachment or contended
probe only permits this owner's independent shared-lock attempt. Attachment is
published only after that attempt succeeds. Writable initialization retains its
existing truncate, checked release and shared-acquisition sequence, without a
header-write fallback.

`nativeMap` defaults to `PAGE_READONLY` and `FILE_MAP_READ`; only the managed
writable capability changes protection and adds write access. Those selected
values are passed to the actual native mapping calls. The zero maximum size,
existing-extent check, aligned base, exact view length, retained owner,
failed-view tombstone, unknown-close behavior and retirement ordering remain
unchanged. No read-only slice is written to test its protection.

## Source controls awaiting execution

- `TestWindowsReadOnlyDMSDecisionMemory` calls the production helper for both
  access modes and all 256 values of its observation type.
- `TestWindowsReadOnlyDMSProductionSourceBoundary` checks complete AST structure
  for the actual probe, helper, cleanup, shared acquisition, attachment and
  managed poison/ledger boundaries.
- `TestWindowsReadOnlyMappingProductionSourceBoundary` checks complete AST
  structure for real mapping arguments, extent/registration and the retirement
  bridge through native/file ownership.
- `TestWindowsReadOnlySourceMutationCounterexamples` supplies 36 inert source
  changes to that same structural checker without consulting byte pins. These
  cover capability bypass, unrelated-error admission, missing/duplicated or
  unchecked release, early truncate/publication, borrowed local attachment,
  writable mapping flags, extent growth, discarded ownership and unknown-close
  retries. The source requires parseable counterexamples; none is compiled or
  executed as a mutated program.

The 21 full-function AST contracts deliberately freeze bounded argument and
control-flow shapes. Separate new source pins are change detectors, not native
proof. Existing guards and pins have not been changed. Neither this document
nor these controls qualify uncertain sibling cleanup or interrupted initializer
schedules, which need separate protocol work and evidence.

## Preserved boundaries and remaining evidence

The plain VFS's native handle ownership, metadata tombstones, exact range locks,
DB upgrade/downgrade restoration, mapping retirement, and managed cleanup are
unchanged outside `nativeAttachDMS` and `nativeMap`. Windows encrypted rekey and
its lifecycle protocol are separate and unchanged. Linux/Darwin backends,
upper-layer admission/caller ordering, Go/module pins, manual source gates,
oracle contracts, and all existing test sources are also unchanged.

The local official SQLite 3.54.0 `src/os_win.c` reference uses a cold read-only
DMS refusal with exclusive release, and read-only map protection/access. Its
process-shared native handle lifetime, header-zeroing fallback and open behavior
are not copied. Provenance is recorded in [upstream.md](upstream.md).

Future evidence requires separately authorized frozen-source formatter/parser
and complete execution-closure review, then bounded tests and native Windows
healthy-source work. Cross-compilation or host models cannot establish native
Windows locks or mappings. Public read-only support, external keeper scenarios,
read-only media qualification, physical durability, and M3/M8/M10 acceptance
remain outside this source-only groundwork.
