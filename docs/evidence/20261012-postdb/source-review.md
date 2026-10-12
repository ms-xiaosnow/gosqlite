# Independent post-DB SQL overlay review

Status: CLEAR_SOURCE_ONLY after the one-row correction below; no remaining blocking source findings. The bounded seven-file source review is complete; this is not execution authorization or a model/native PASS. This review covers the frozen, formatted bytes listed below, not a later merge or changed file. Model tests, guard execution, compile-only checks, and all physical/manual qualification remain NOT_RUN by this reviewer.

## Finding resolved before final freeze

The original retained `pre-DB uncertain accepted` mutant changed only the lifecycle constructor. The new assignment at `internal/testkit/rollbackfault/sql_ledger.go:420` overwrote that value for every failure case, so the mutant no longer exercised pre-DB IDs 5/6/7. This was a guard-test coverage regression, not a discovered caller/model acceptance defect.

The independently checked correction changes exactly one row at `internal/testscope/rollback_fault_sql_test.go:88`: it now mutates the actual assignment from `evidence.Case == SQLFinalDirectorySyncBefore` to `isSQLPreDBFailure(evidence.Case) || evidence.Case == SQLFinalDirectorySyncBefore`. That genuinely changes only old pre-DB IDs 5/6/7, preserves ID9's correct uncertainty, and retains the scenario name, assertion, and independent semantic diagnostic. All other six files are byte-identical to the prior reviewed freeze. The implementation's one-file formatter receipt records exit 0 and no additional byte changes. The finding is resolved in the final hashes below.

## Reviewed source properties

- IDs 0–7 and their selectors remain in order; only IDs 8/9 are appended. ID8 selects JournalUnlink/Journal owner12/occurrence1 at Calls[16], FailBefore, 17 calls and CommitRecoveryRequired. ID9 selects FinalDirectorySync/Directory owner13/occurrence1 at Calls[17], FailBefore, 18 calls and CommitUncertain. Both require the previously completed three-page writes and DB Sync; ID9 additionally requires the completed unlink. The unchanged generic receipt validates actual returned results, exact call order, target hit, error tokens, commit class, closed logical state, and owner retirement.
- Public Commit is immediately followed by Finish with its original error object. The caller checks exact `*sqlite.Error` identity and CodeIOErr. The bridge observes the actual target and retired Tx poison, binds the public wrapper's sole unwrap to that poison, and allows only the concrete errors.Join type plus exact injected, ErrPoisoned, and one case-specific sentinel leaf. It refuses custom Is/As before unwrap, unknown/nil children, extra cleanup causes, and bounded-work overflow. Unlink permits ErrRecoveryRequired; final-directory permits ErrCommitUncertain. Owner or Close errors cannot reach the observed terminal through this path.
- Both new cases execute a full seven-query SQL/index/scan/typed-row/EOF/Err/Close/Check NEW snapshot on retained journal-free `post-commit.input.db` before the recovery branch. The caller verifies all owners are retired and retained inputs unchanged. Before/PostCommit/After reader owners and iterator identities are independently checked. The model requires a canonical-zero PostCommit snapshot and false PostDBOnlyTargetCause for the old seven cases.
- ID8 retains the original trial and journal, creates distinct O_EXCL copies, and invokes the existing bounded recovery helper exactly once on replay.db. Its final full readback and full recovered-byte hash must be OLD. ID9 requires absent journal artifacts, zero recovery calls, unchanged NEW replay bytes, and a final full NEW readback. Original identities, lengths, hashes, retained copies, and final owner retirement remain required. NEW observation is never durable acknowledgement: PhysicalValidated and Pass remain constant false.
- Actual public classification fields are derived from report.CommitError and the bridge's actual cause observation; PostCommit comes from the actual readback result. Source control flow, rather than plan-derived result fabrication, binds the one-shot lifecycle and recovery decision. Existing baseline/read/open/recovery/registry helpers are unchanged byte-for-byte; Query and Readback helpers are also unchanged.
- All 21 prior model test roots remain; three focused post-DB roots are added. All nine guard roots remain. All 141 prior repinned mutant labels remain, with 67 additions (208 total). The two shared model negatives that would otherwise be no-ops on ID9 are properly changed: wrong-commit uses CommitRecoveryRequired for ID9, and the public uncertainty flag is inverted. The new tests cover exact prefix results, swapped classes, masked/native failures, false acknowledgement, independent NEW evidence, wrong recovery, artifacts, closure, and canonical-zero old-case fields.
- Four complete source snapshots and their four SHA-256 pins independently match their entire source files. The mutant harness repins both whole-file hashes and complete shapes, requires a named independent semantic diagnostic, and rejects reliance on supplemental snapshot diagnostics. All 206 directly encoded literal mutant anchors are present and differ from their replacements; the remaining two use the unchanged exact manual-build-tag constant. Source inspection of the semantic predicates, exact function comparisons, and call/order checks supports the intended bounded guard coverage, with the corrected retained mutant described above. No guard or mutant was executed, and no arbitrary-program proof is claimed.

## Baseline and integration boundary

The supplied published base identity is main `4dc3979aa8d503913b75e3c0c432c234d7bc0a40`, tree `4fbf9a52417eedcd7dcf6e08ecb31a4147915a9f`. No Git command was used to re-verify those supplied identifiers. Each of the six baseline Go files in W independently matches its baseline SHA in the evidence manifest. The manual's baseline matches W after removing only W's new first validation paragraph and following blank line. Preserve that paragraph when integrating; do not replace the current manual wholesale. No adapter/default/VFS/recovery/A5 source is included in this seven-path overlay.

The implementation's separately authorized fixed-gofmt receipts were read, not replayed. This reviewer performed only source reads, text/byte comparisons, SHA-256 calculations, and this report write. No Go/gofmt/parser/AST/scanner/test/compile/manual/native/fixture/oracle/recovery/controller execution, W write, Git operation, or network publication was performed.

## Bounded validation handoff

The affected normal/race closure remains exactly `./internal/testkit/rollbackfault` and `./internal/testscope`: pure value/model tests and source/scanner tests. Retain the existing six-step scope: fixed-tool formatting check, affected normal tests, affected race tests, existing all-source guard, compile-only sqlite image with `manual_fault_rollback,manual_restart_rollback`, and read-only nm inventory. The existing symbol contract remains 454 exact required sqlite test roots and 18 forbidden roots; no manual root was added or renamed in this slice. This handoff describes the parent's already bounded next stage, not results from this review. Do not execute the compiled image, any manual/native/fixture/recovery case, A5 controller, full-repository test sweep, or unrelated hypotheses. Rebind results to the actual integrated source identity and preserve the current 4dc manual header.

## Frozen review hashes

- docs/testing/rollback-fault-sql-manual-zh.md — 30,151 bytes — a6b2e10c7b2cbac7b57adcc5cd1bc38afe576eee1287f9ad6c7aad6d0f4cedee
- internal/pager/rollback_fault_sql_manual_linux.go — 8,391 bytes — ad9f263e2d15a496cc70b1d35038f5b0393e8b94c0923ea5db96bf4eff71c0f9
- internal/testkit/rollbackfault/sql_ledger.go — 25,282 bytes — 7d47e46b34204c12a4df50d02a24179bf22e081d45f2816e3c048b0bc7dd04dc
- internal/testkit/rollbackfault/sql_ledger_test.go — 83,124 bytes — 5bea133008de83e9c5e737582305034cf24e1b99a25d402cba61c596f52ef2a3
- internal/testscope/rollback_fault_sql.go — 285,272 bytes — ac577f657d71a52426c100150b89b067a4d19410569a430f5b789828c7ef243b
- internal/testscope/rollback_fault_sql_test.go — 65,601 bytes — 977605cd7e68411e1ccc6521003e028edc44969a42cb33d0f605d2d39a4eb525
- sqlite/rollback_fault_sql_manual_linux_test.go — 45,972 bytes — c368296059cbf29a4756cbb760b3836c580d8615e25a7330f76d83ba97427037

Evidence hashes at review:
- postdb-source-evidence/SEVEN_SOURCE_HASHES.json — b626dedb6cf66e4d9dfc883c1e3bd33147d3fad2d406dc43b58049be13693021
- postdb-source-evidence/POSTDB_SOURCE.diff — 5e46f9e904e397685a287b69439a6d64c8b7f5345178b9b31e15a6e4e0ab89c9
- postdb-source-evidence/GOFMT_SIX_RECEIPT.json — 111f3919278f77ee983ebfebf4d5374023ac93b7b4b06700381cd30efb40869f
- postdb-source-evidence/GOFMT_GUARD_RECEIPT.json — 7dd3e2906e5e264bb9c76e4817af2e686de36d724008d62f944d2f656e8fa1cc
- postdb-source-evidence/UNCERTAIN_MUTANT_FIX.diff — 92adb23b4f8141e36e449e31a0b825eb8b561e2da5819eb1db4f37b55c2c415a
- postdb-source-evidence/UNCERTAIN_MUTANT_FIX_RECEIPT.json — a514828db9d86a4ddf1dc7543bed105467cacc90e64ca95e30105ee0682cc31b
