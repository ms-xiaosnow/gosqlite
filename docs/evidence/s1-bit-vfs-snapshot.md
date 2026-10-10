# S1, BIT and Linux ownership frozen-tree checks

The prepublication aggregate executes the exact archived Git tree
2a974fa1f54c29ea804cc5fcc16f76bb38d976bc, excluding concurrent converter,
HALFVEC and native-platform implementation edits. Its complete-file SHA256
inventory has digest 68d6caa5356d5dc46be3fbff8bc6ef299b1fed4adf722b79c9a2455ee089dd4a.
The source archive has no Git metadata; this is tree evidence, not a claim that
an independently generated local commit equals the eventual remote commit.

The first attempt passed full CGO0 unit tests (sqlite86.191s), then failed VCS
status discovery for the standalone archive. Its exit1 and output are retained
in s1-bit-vfs-aggregate-initial.txt. No production or test source was changed.
The corrected command explicitly sets GOFLAGS=-buildvcs=false. This disables
build metadata discovery only, not any test or source-scope check. Final guarded
unit/vet/build, race, pinned oracle and cross-build results are recorded separately.

Two CGO0 trimpath builds from this exact archive were byte-identical and statically
linked. SHA256 35e7f3099324e8c66463ab6ae9cef2b9c17bc6a0ac5ed65fbd31bdf7881f19c6;
see s1-bit-vfs-build.txt. VCS stamping was explicitly disabled and no eventual
commit identity is asserted for these binaries.

Later prepublication documentation/evidence additions are separate from the
archived source tree. New converter/half/native-platform code is not part of this
checkpoint. Restricted physical failure/recovery and independent release gates
remain open.

At the durable checkpoint decision (2026-10-10T13:57:41Z), the corrected archive
run had passed complete CGO0 unit/vet/build and the complete CGO1 race suite
(sqlite81.243s and406.026s respectively). The full pinned oracle and following
cross-build/unchanged-source checks were still running. The explicitly partial
s1-bit-vfs-aggregate-in-progress.txt is not a completed aggregate. Focused S1,
BIT and native-ownership oracle/race gates had already passed separately.
Publication preserves this coherent reviewed slice while those final gates are
monitored; no full aggregate or release acceptance is asserted at that point.

The corrected frozen-source aggregate subsequently completed exit0 at
2026-10-10T14:02:38Z: full pinned oracle passed (testkit301.806s/sqlite97.002s),
four cross-builds passed, module and empty-CGO closure verified, and every frozen
source/fixture hash stayed unchanged. See s1-bit-vfs-aggregate-final.txt. This
closes that aggregate only; remote and Library-restored gates remain separately
tracked. Native Darwin/Windows code was still excluded from this checkpoint.
