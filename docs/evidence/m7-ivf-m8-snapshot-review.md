# Scoped engineering reviews: IVFFlat and encrypted snapshot integration

Date: 2026-10-10. These are ordinary implementation reviews, not independent
release, cryptographic, legal or physical-durability qualification.

## IVFFlat

Independent review found no blocking correctness/resource issue. Added only
sqlite/ivf_review_test.go and extension/vector/ann/ivf_review_test.go: separately
calculated membership across L2/cosine/IP, extreme signed rowids, fixed centroid
bits, NULL transitions, multi-index statement undo, reopen, cumulative fallback,
training objective and exact work/memory boundaries. Unit and race pass; additional
memory-only fuzz3834 executions after83 baseline cases passed. Earlier algorithm
fuzz121122 and canonical config fuzz84262 were separate campaigns, not summed as
one target. Integration log records actual algorithm/storage/query commands.

Automatic first-match index selection and EMPTY request index-dependent validation
are deliberate disclosed limitations, not hidden fixes. No damaged database or
physical recovery workflow was used. Genuine older HNSW fixture is copied into
test temp space before normal upgrade and remains byte-identical at its source.

## Encrypted snapshots

Independent source review confirmed expected authenticated child identity/geometry,
complete page/footer/exact extent checks before logical parsing, fresh write-once
identity per publication/rekey, direct plaintext publication isolation, and poisoned
uncertain namespace outcome with no historic fallback.

Review-driven changes moved new-family credentials/actual geometry and quotas
before stable lock creation; reserved the upcoming lock inventory slot; bounded
retained/abandoned family bytes and portable backup size; added O_NONBLOCK plus
nofollow regular/directory guards; rejected hard-linked held plaintext source.

Review tests only: internal/encstore/review_test.go uses memory framing/manifest
mutations, geometry and accounting. sqlite/encryption_review_test.go uses intact
correct-key SQL, overflow/freelist reuse, rollback, same-key rekey fresh identities,
retained-byte immutability, verify and reopen. Focused CGO0, race and vet passed.
Actual worker command filters are preserved in m8-snapshot-worker.txt. Lead's first
three-page-size oracle run exposed omitted standalone-validator connection defaults;
fix shared their normalization, and the unchanged lifecycle passed77.501s. Failed
and successful output are both retained.

No wrong-key-on-disk, tampered-on-disk, physical I/O fault, crash/forced-exit,
interrupted rekey or hot recovery test was run. No equivalent workflow was moved
to CI. Retained-old-key history, full-family replay/substitution, non-pinned-parent
namespace and exclusive-writer snapshot limitations remain explicit. Encrypted WAL
is required future code, not an acceptance-only omission.

Final lead source check found plaintext-sidecar refusal occurred after a new
stable-lock creation for an absent root. The narrow fix probes readonly before
possible lock creation and still rechecks under the acquired lease. It avoids
stranding a new family for a pre-existing suffix while preserving fail-closed
behavior for a later namespace race. The refusal branch is source-reviewed only;
no fabricated sidecar/disk-negative fixture was executed. Existing allowed focused
unit/race/vet passed; exact output is m8-snapshot-sidecar-preflight.txt. The first
aggregate had begun before this final source change, so final-supplement and
clean Library restore evidence must qualify the final source, rather than
misstate the first compiled test binaries as including it.
