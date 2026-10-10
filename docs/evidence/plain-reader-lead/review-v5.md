# Lead independent review: Reader V5 provenance correction

2026-10-10 UTC. Decision: **accept this scoped source increment for integration**,
subject to the exact combined-tree gate. This extends the earlier independent
Reader V2 review and preserves the V3/V4 findings and negative controls. It is not
native Windows/Darwin execution, actual failure qualification, or milestone/release
acceptance.

## Source result

V5 fixes the V4 Windows source blocker. VFS ResolvePathForCreate/LstatForCreate
return explicit absence provenance. On Windows, it is true only for a sole
ordinary NotExist from the initial metadata-open failure with no acquired owner.
Any later snapshot/final-path/volume inspection error returns false even after
successful retirement. Legacy ResolvePath/Lstat still expose their previous error
shape; the additional boolean is consumed by the Reader and plaintext public
canonicalization. The no-handle metadata open/observe transitions were checked
against metadataOwners.open and observeMetadata, not inferred from error text.

Portable plaintext canonicalization now rejects a nonabsence final Lstat error
rather than discarding it. Plain Windows resolution and final-component probing
both require the explicit provenance before taking a create fallback. Reader
sidecar absence follows the same rule. The encrypted branches retain their prior
source and operation order. The pager additionally requires its private initial
Reader marker through a bounded sole-cause graph; later main-file failures,
additional diagnostics, duplicate markers and pending ownership cannot authorize
create. No new native call, background cleanup or raw-handle retry is introduced.

The VFS helper uses a fixed4096-node bound and validates before growing traversal
storage. It does not invoke arbitrary Is methods. Mutable/custom application error
implementations and allocations performed by those implementations are not a total
process-RSS bound. Native provider/descriptor APIs remain separately controlled.

## Independent checks

Both original V5 and the independent copy match all1024 owner-manifest files:
12bb816edaf4b88af31c243bd409fbf2262d7ef1a5f0c1c7a3b5d46eeb725f50.
V4-to-V5 patch SHA256:
728f8bc08033ded98da255c2b72c12a87c6c3386fa20b1c8119cc10a7b7fcd9f.
The copy adds exactly two declared test files, preserved here as .go.txt probes.
No owner freeze or main production source changed during this review.

Descriptor-free independent probes cover65 wrapper depths for both initial
no-owner failure and post-acquisition inspection NotExist, requiring unchanged
error identity, correct retirement count and empty local ownership registry.
Mixed acquisition causes are rejected. The unchanged four-case V3 negative control
also passes against V5; its original V3 failures are retained. Existing Reader,
public cleanup, VFS provenance/metadata/callchain memory laws and intact ordinary
creation/close/reopen were independently rerun. Normal and race selections both
passed, exit0 at19:39:06Z. Exact commands and raw results accompany this report.

The owner additionally records build/vet and Darwin/Windows compilation on the
exact source. This lead rerun was scoped normal/race, not a new full suite or native
target execution. No path replacement, missing-selected-file workflow, corruption,
native failed close/unmap/lock/Sync, crash/kill/process-exit or recovery was run.

## Remaining scope

Standalone Tx/journal and WALExclusive are separate pending increments and need
this base plus their own independent review. Shared-session/recovery legacy
os.IsNotExist callers currently reject Windows inspection joins but rely on the
error shape rather than this explicit provenance; that coupling remains tracked.
The reviewer's earlier direct-OpenTx example was corrected: os.IsNotExist does not
unwrap observeMetadata's generic join, so that specific current bug was not
established. The planned direct-constructor provenance contract must not be
reported as evidence that a prohibited native failure was reproduced.
