# Windows mapping-object retirement correction

Date: 2026-10-10. Target: Windows/amd64. This is a bounded source and memory-law
correction, not native runtime, physical failure, durability or release
acceptance. Production remains pure Go with CGO disabled.

## Previously uncovered ownership gap

Source review identified two paths in the historical plain Windows VFS and in
the frozen Windows/public-metadata tree
`5207d59b667614c40c184ec277fdc1fc1ca80fae`:

1. `nativeUnmap` returned a mapping-object CloseHandle error but retained the
   same nonzero numeric handle. Repeated Mapping.Close/File.Close could close
   that value again without knowing whether it still identified this object.
2. After MapViewOfFile failed, `nativeMap` tried to close its mapping object.
   If that close failed, it appended an ordinary mapping record to the file's
   retained list. Later nativeClose called nativeUnmap and retried the same
   numeric handle.

Earlier `native-ownership-*`, `native-windows-*`, and
`native-plain-metadata-*` evidence did not cover these terminal mapping-object
states. Those records remain factual and unchanged; neither their portable
models nor their compile checks establish safe exceptional mapping retirement.
This is a source-discovered correctness flaw, not an observed data-loss event.
No native close/unmap failure experiment was performed.

## One private retirement gate

`internal/vfs/mapping_retirement.go` holds the native view base and length,
mapping-object handle, and the first indeterminate close cause. The Windows
nativeMapping embeds that state. Both a published mapping's nativeUnmap and a
failed MapViewOfFile acquisition's cleanup call the same transition.

- The first mapping-object close error latches ErrPoisoned plus its original
  cause. Its numeric handle, view base and length stay represented. All later
  retirement calls return that same stored cause without invoking either close
  or unmap callbacks. The stored cause is allocated once and never recursively
  joined with its own previous value.
- An object close that succeeds clears only its handle. The independent view
  remains represented until a successful UnmapViewOfFile clears its original
  base and length. An unsuccessful represented view-only retirement can be
  retried, without ever closing the already-retired object handle again.
- A fully retired or empty state is idempotent. A rejected view acquisition has
  no view base, so cleanup never calls UnmapViewOfFile for it. Successful cleanup
  returns the original acquisition error; uncertain cleanup retains the
  terminal mapping record on its already-owned File.

The private native adapter supplies fixed Win32 operations. Tests exercise the
same portable transition with synthetic integers and memory callbacks; they do
not replace native operations or inject physical failures.

## Preserved mapping and protecting-resource contract

Teardown still closes the object handle before unmapping its view. Microsoft
documents that open views can outlive the mapping-object handle. This order
keeps Mapping.Bytes live after a failed close. Reversing the order would leave
the generic Mapping contract with an invalid slice after an unsuccessful Close,
so the correction deliberately preserves it.

An uncertain published mapping close leaves its Mapping in File.maps and keeps
its Bytes intact. File.closeResourcesLocked does not enter native file/lock
retirement while any mapping remains unresolved. File ownership stays strongly
retained. A failed view acquisition's terminal record stays in native.retained;
nativeClose returns its poisoned error before releasing any held slot, DMS,
ordinary file handle, retained parent, or native registry lease. Ordinary native
operations reject the poisoned file. Known independent mapping siblings may
still retire, but they do not clear the terminal record or release its lease.

Allocation granularity, mapping-span arithmetic, original-base unmap, overlapping
view behavior, exact byte-lock intervals, identity checks and sharing policies
are unchanged. Neither a retry count nor garbage collection resets terminal
uncertainty. Exceptional resources may remain retained permanently. This
correction does not add process-wide mapping recovery, bounded reclamation, or
the separate failed-OpenSharedSession pending-retirement group. It does not
claim that a failed native close proves its handle remains valid.

## Verification boundary

The isolated check tree is an archive of
`5207d59b667614c40c184ec277fdc1fc1ca80fae` with only the three mapping source/test
paths overlaid. Unrelated live Darwin, typed-ANN and failed-open-retirement edits
are excluded. Exact commands, reviewed test behavior, results and source hashes
are recorded in `docs/evidence/windows-mapping-retirement-components.txt`,
`docs/evidence/windows-mapping-retirement-owned-paths.txt`, and
`docs/evidence/windows-mapping-retirement-source-sha256.txt`. The independent
source/model review is recorded separately in
`docs/evidence/windows-mapping-retirement-independent-review.txt`.

Five portable memory cases cover terminal repeated close, successful object
close followed by retryable view retirement, rejected view acquisition cleanup,
published mapping and protector retention, and complete/empty idempotence.
Repeated terminal paths check 1,000 attempts without a second close callback or
growth of the stored cause. Selected pre-existing source, arithmetic, ownership
and ordinary Linux mapping/metadata regressions are also checked, including
separate race tooling. Cross-target builds and test-binary compilation do not
execute Windows code. The existing default Windows vet unsafe.Pointer diagnostic
at the successful mapped-address slice conversion remains disclosed; no native
unsafe boundary acceptance follows from compiling it.

Native Windows runtime remains **NOT_RUN**. No failed native I/O, lock, unmap,
close or Sync experiment, physical corruption/tamper/wrong-key test, missing
selected-file test, namespace mutation/retarget/race, crash/kill/process-exit,
hot recovery, interrupted publication/rekey, or manual harness was executed.
Full security, durability, cross-process interoperability and release gates
remain open.

## Primary sources checked

Microsoft documentation was checked on 2026-10-10:

- [CloseHandle](https://learn.microsoft.com/en-us/windows/win32/api/handleapi/nf-handleapi-closehandle): checked outcomes and one close per acquired handle. Terminal uncertainty is this implementation's conservative policy.
- [Closing a File Mapping Object](https://learn.microsoft.com/en-us/windows/win32/memory/closing-a-file-mapping-object): handle and mapped-view lifetimes are independent.
- [UnmapViewOfFile](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-unmapviewoffile): the original mapped base identifies the view; successful unmapping invalidates its range.
