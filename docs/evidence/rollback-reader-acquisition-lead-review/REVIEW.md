# Independent lead review: rollback Reader snapshot acquisition

Decision: no blocking finding in the exact six-line production correction.
This scoped source/ordinary-path conclusion is not complete durability or native
platform acceptance. The original failed soak remains a failed historical run.

Base tree: 13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33.
Reviewed tree: ad066d04d676b8368ca7d02d83664e50dfa47216.
Patch SHA256: ccc9bf728b3df6b0daa65562955ae54cc84fcdf690a9d8985d17888d740c3659.

The independent lead recomputed Git-compatible blob/tree hashes from every path
and mode, checked all 1324 final files against the inventory, and reconstructed
the candidate by applying the patch to a new copy of the full1322-file baseline.
Both constructions equal the reviewed tree. The independent executable test
copy contains no reviewer source edits; an additional separate source-only
negative copy removes only the six new production lines.

## Correctness and ownership

OpenAdmitted first adopts the managed descriptor and rejects non-regular files.
Its old stat was before SHARED, so a lawful writer could commit before SHARED
acquisition and leave an obsolete size/ModTime. The correction refreshes stat
on the same owned descriptor immediately after successful SHARED. Reader.info
and ParseHeader use this refreshed snapshot. A cooperating rollback writer
cannot publish a database change while that SHARED lease is held.

The added failure branch goes through the already established deferred bounded
retirement owner. No descriptor/lock is discarded, error suppressed, raw close
introduced, or cleanup retried by this delta. Existing sidecar checks, header
validation, path identity, exact header/page snapshot checks, initial regular-
file validation and public/admitted owner behavior remain unchanged. ErrChanged
is never translated to BUSY. Stat does not need to re-open a pathname.

The AST law checks exact ordering and error branches; it is deliberately a
source regression rather than a claim to deterministically execute the narrow
race. The ordinary concurrency test uses createEmpty's normal native Tx create/
commit/close helper, two Readers and one normal writer. It performs128 commits
and32 rollbacks, compares pinned valid pages/generation, permits only bounded
BUSY-only error trees, and joins/normal-closes workers on every terminal path.
It neither alters names nor installs native-operation hooks. Live coverage is
scheduler dependent, as documented.

## Independent executed evidence

checks.sh and checks.txt/.exit record exact frozen selected tests:
- Normal reader acquisition source/memory/live suite, count5, PASS.
- Race reader acquisition source/memory/live suite, count3, PASS (pager1.481s).
- Pager vet, full pure-Go build, and all final hashes PASS.
- Terminal PASS2026-10-10T21:09:11Z.

The old-constructor source-only control failed exactly on the missing post-lock
stat statement. See negative-source.txt and negative-source.exit (1). That run
selected only TestReaderAcquisitionSourceSnapshotAfterShared: no native database
was opened and no physical scheduling/error injection occurred. Replaying the
patch was verified before this explicitly separate negative mutation.

Owner frozen repeated ordinary controls/candidate soaks are still in progress
at this report time; their later raw results are separate evidence. The original
V1 failed report and closed database are retained by its owner. A passing short
sample does not supersede the failure or establish long-soak success.

No native failure, physical corruption/tamper/wrong-key/missing-selected-file,
namespace interference, crash/forced exit/hot recovery or interrupted publication
was executed. No foreign-target binary was run. Full combined-source gates and
exact remote/Library/CI identity are still separate integration requirements.
