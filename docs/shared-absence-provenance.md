# Shared-WAL initial-absence provenance

Date: 2026-10-10. This narrow source correction removes the remaining generic
`os.IsNotExist` decisions from native shared-WAL database creation, rollback-mode
WAL-sidecar preflight, and the live journal check. The private rollback-transition
Session probe now forwards the same explicit provenance regardless of whether
it has an exclusive plaintext cleanup scope.

Only the existing VFS initial, no-handle absence result permits creation or
ignoring an absent sidecar. Later inspection, retirement, or acquisition errors
remain diagnostics even if they contain ENOENT. On Windows the existing managed
metadata probe provides this distinction; Linux/Darwin retain the existing
no-handle path behavior. No raw descriptor, OS backend contract, public layout,
SQL behavior, or persistent format is changed.

Both shared retirement schemes remain intact. Failed-open admission precedes
metadata; successful main/WAL/index acquisitions are adopted before later work;
failed-open deferral collects error-carried children before index, WAL, and main
retirement. Successful open transfers to its separately bounded live/close
reservation. The transition continues borrowing its main descriptor and never
runs exclusive-WAL scanner, replacement, or SHM-removal paths.

The transitive review covered Session.checkJournalLocked, enterWALLocked,
Tx.checkStable/checkSidecars and Commit, native metadata wrappers, the platform
provenance implementations, and failed-open/successful-close ownership. Existing
checkSidecars already uses explicit provenance. Only shared_session.go and
plain_session_cleanup.go change production behavior.

## Verification scope

New AST laws pin the four decisions and admission/adoption/error-exit boundaries.
Descriptor-free pager laws preserve late ENOENT diagnostics and nested-owner
barriers. VFS memory callbacks verify inspection/retirement ENOENT cannot become
absence, error-carried terminal discovery remains available, and a later poisoned
acquisition neither opens a handle nor authorizes absence. Existing memory
absence/cleanup controls are unchanged. New source laws are expected to fail on
the immutable baseline, while memory controls pass there.

The frozen handoff records focused CGO-disabled memory and normal Linux shared
lifecycle tests, a separate CGO-enabled race-tooling run, vet, and compile-only
Linux arm64, Darwin amd64/arm64, and Windows amd64 checks. Base-plus-patch replay,
whole-source hashes/modes, owned changes, and exact commands are recorded outside
this source snapshot by the verification handoff. These focused checks are not a
new full aggregate or hosted CI result.

No physical missing-selected-file, corruption/tamper/wrong-key, namespace
mutation/replacement, crash/kill/abrupt exit/hot recovery, native I/O/lock/unmap/
close/Sync/short-write/disk-full/permission failure, interrupted publication/rekey,
or manual-tag workflow is executed. Native failure and non-Linux runtime
qualification remain NOT_RUN; full durability/security/release gates stay open.
