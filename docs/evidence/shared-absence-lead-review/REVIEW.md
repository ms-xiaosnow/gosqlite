# Independent lead review: shared initial-absence provenance

Scoped PASS for b983d28c1e217d121bf5cda0ff92c6e85accef41; no blocking finding in
the two production files. Base13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33.
Full patch SHA256 a4aa67ed7b2764726b059d0f18a6ce6fe2b8f2ea3d0663968b0137f6310c7770.

The lead independently recomputed complete Git-compatible trees, every1325-file
SHA256/mode and a checked baseline-plus-patch replay. All identities match.
The six-path delta includes a historical base progress append; integration must
exclude that stale central document and carry a current separate progress entry.

## Source and ownership review

Three shared-session classifications now use the existing explicit initial
no-handle absence result: database create resolution, rollback-mode WAL preflight,
and shared journal-sidecar validation. A later metadata or retirement diagnostic
containing ENOENT is propagated instead of authorizing create/ignore. Parent
resolution errors remain unsuppressed.

The private transition Session.probePathLocked now forwards this provenance
whether or not it carries exclusive-session plaintext cleanup. Existing exclusive
session behavior is unchanged. Calls in journal checking, WAL opening/identity
and SHM preflight retain the same file-info/error contract. The shared transition
uses only its normal rollback-to-WAL path, not an exclusive scanner or namespace
cleanup path. File formats and public layouts are unchanged.

Shared pre-acquisition admission, main/WAL/index adoption, error-carried child
collection, and child-before-main retirement are untouched. No raw descriptor
close, new retry, or reordered logical publication is introduced. The source
laws retain those orderings. The VFS model test uses inert integers/error values;
its callbacks never call a native operation. It verifies that later inspection,
retirement or poisoned acquisition preserves errors/owner discovery and cannot
be mistaken for initial absence. Pager memory laws keep original late causes and
terminal nested ownership barriers through explicit memory retries.

## Independently executed evidence

Exact unedited frozen copy, Go1.27.2 with CGO0; CGO1 only for race tooling:
- New source/descriptor-free memory laws normal count3.
- Audited ordinary shared snapshots, read promotion/BUSY, normal commit/rollback,
  checkpoint and retained fully-committed normal reopen at512/4096/65536, count2.
- Same selected scope under race, count2; pager1.149s, VFS1.012s.
- Pager/VFS vet, complete pure-Go build, and all final hashes.
- Terminal PASS2026-10-10T21:21:32Z; checks.sh and checks.txt/.exit.

A separate replay copy restored ONLY the original two production files and ran
ONLY new AST source laws. It failed on all four generic NotExist classifications
and the absent explicit-resolution source references, exit1. See negative-source
logs. No native database or synthetic file absence was executed by that control.
The production/adopted source copy and original baseline remain unmodified.

Native failed acquisitions/retirements, physical missing-selected files,
namespace interference, wrong-key/tamper, crash/hot recovery and interrupted
publication/rekey remain NOT_RUN. No target-OS binary was executed. Full combined
checks, exact remote/restore/CI and independent final security acceptance remain
separate gates; this closes the source classification gap only.
