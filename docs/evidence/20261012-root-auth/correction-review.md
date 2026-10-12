# Supplemental source review: encrypted-root wiring integration

Date: 2026-10-12 UTC. Scope: only the two-file readonly-WAL source-guard correction under `root-auth-wiring-correction/internal/testscope/`. No checkout edits, Go/gofmt/parser/test/compiler executions, native/manual/database operations, network actions, or Git writes were performed by this reviewer. The implementer's separately approved formatting pass is checked as a source delta below.

## Preserved first failure and corrected conclusion

The original `root-auth-independent-review.md` is preserved unchanged (SHA-256 `a992637f944e4738430212a3fceb1521bcfdeaa16cdd3a51ce5db16e8301a4e6`). Its review missed one existing guard-integration requirement: readonly WAL's closed findings-append inventory did not know the new encrypted-root source hook. The new main's source/error/findings blocks themselves remain correctly bound; changing whole-main pins did not integrate that independent closed inventory.

Actual first bounded validation, session 17139, `02_normal` on the parent-identified tree cae45848, exited 1 at 2026-10-12 21:40:43 UTC. Independently parsed original JSON events show:

- `internal/testkit/encryptedroot`: 36 named PASS, zero FAIL/SKIP.
- `internal/testscope`: 8 named PASS, one FAIL, zero SKIP.
- Sole failed root: `TestRollbackRestartWiringPreservesAllPoliciesAndReviewHold`.
- Diagnostic: `readonly WAL manual previous guard wiring weakened: direct placement and checked findings/error propagation required for findings reset/rebinding`.
- `root-auth-validation/02_normal.stdout` SHA-256: `c3de4c2c14e865e78312c3a6b614d0b51bbf4d696399a31f0eaf74896f9e4027`.
- `root-auth-validation/02_normal.result.json` SHA-256: `d30eb3386d75ec157779c9d37b4e98c38823297e2b47aacc6bb419ff007d826c`.
- Original STOP record states later commands NOT_RUN and physical_manual_executed=false. This is a real source-guard integration FAIL, not an authentication, physical fixture, or native failure. It is not rewritten as an expected PASS.

## Finding W1 and bounded correction

Baseline `readonly_wal_manual.go:1361–1371` lists every exact permitted findings append, and lines 1383–1389 reject any other rebinding. The added main statement `findings = append(findings, rootNegative...)` therefore correctly hit the independent closed-inventory refusal. Accepting all appends or deleting the old assertion would weaken its intended guarantee; neither is part of this correction.

Reviewed correction:

1. `readonly_wal_manual.go:1259–1262` requires the exact adjacent source assignment, exact `rootNegativeErr` check/return, and exact `findings` append in the direct WalkDir callback body, using the existing duplicate-refusing AST sequence matcher.
2. Line 1311 requires this new triplet before formatUpgrade and retains all old source-filter order comparisons. Missing formatUpgrade still fails because its index cannot exceed a nonnegative rootNegative index.
3. Lines 1315–1338 add only the exact encrypted-root contract row after readonlyContracts. The existing matcher still requires direct main-body assignment, checked diagnostic/error propagation, findings output, and increasing order. The final failure exit must follow this now-last contract too.
4. Line 1367 adds only the exact rootNegative append spelling to the count-one inventory. All previous ten append entries, findings/failed reset rejection, address-escape rejection and failure-initialization rules are unchanged.
5. Line 1539 adds only the exact source and repository-contract calls to the existing once-only inventory. This complements direct placement and binding; a correctly spelled string alone is not sufficient.

Source-review disposition: the pre-format correction is CLEAR within this scope. It requires no changes to main, model, native harness, new root guard, pin literals, production code, or original failure evidence. Final formatted identities are recorded below before integration.

## Existing counterexamples preserved and extended

`readonly_wal_manual_test.go:623–631,744–752` adds only the new source and contract rows to the existing combined-policy mutation table. Removing those exact two rows recovers the original ten rows byte-for-byte. No new Test root or named child was added; the existing `TestReadOnlyWALManualWholeRootInventory` calls this helper unchanged.

The existing positive baseline requires zero findings before mutations. The unchanged mutation generator applies fourteen inert source counterexamples per row: omission, wrong helper, wrong result capture, wrong result propagation, wrong checked error, wrong propagated error, suppressed error block, dead placement, duplication, state reset, result reset before propagation, address escape, result/error reordering, and intact triplet moved to the wrong direct position. The two new rows use the actual main binding names, exact block text, correct state variable, and a genuinely wrong move destination. Each negative still requires a dedicated semantic `previous guard wiring weakened` diagnostic; a pin mismatch or unrelated error cannot satisfy it.

The explicit inventory changes from ten/140 to twelve/168. The original ten rows and all fourteen transformations are unchanged. Execution of these additional variants is NOT_RUN in this source review; their expected rejection is supported by the direct-sequence, ordering, call-count and result-state checks reviewed above.

## Identities and validation boundary

Baseline readonly guard SHA-256: `506a8cc3e815a9f953b4534aeda4a1d9894ce668ee21df7e9d8790d61f368b17`.
Baseline readonly test SHA-256: `b97f66a2ddd7e2ea481bfb8615a6264d40cb0c7bdec41649280f026f791107e1`.
Reviewed preformat guard SHA-256: `e4e2a49f461185dafc5124c9c02749b11b33a06b1694134114bd1628be5fc5e8`.
Reviewed preformat test SHA-256: `3187dbca3556f0331baaca6d906778a9c0d257c85070eae228e9af61df27bab1`.

The baseline hashes, candidate hashes, and complete textual diff were independently checked against `wiring-correction-preformat-manifest.json` and `wiring-correction-preformat.diff`. Whole-main identity remains `6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2`; no further main repinning is needed. This report only approves the bounded source correction, not a future test result.

A later bounded normal/race rerun should include the previously failed rollback-restart wiring root and the existing readonly whole-root inventory root so the two new rows actually execute, alongside the parent's separately authorized affected checks. Full-repository testscope must still pass normally; no broad skip or allowlist relaxation is justified by this finding. All six physical cases, including control, remain NOT_RUN; running the manual binary is still prohibited on this platform.

## Final formatted source disposition, 21:45 UTC

Final source-review disposition: CLEAR for the exact two-file correction, with no unresolved blocking finding in this bounded delta.

- Final `internal/testscope/readonly_wal_manual.go` SHA-256: `cfe6597ed00b9bec1b86727aec720258a2b872c952a75049d2ab381f5f8e9478`.
- Final `internal/testscope/readonly_wal_manual_test.go` SHA-256: `3187dbca3556f0331baaca6d906778a9c0d257c85070eae228e9af61df27bab1`.
- Preserved preformat snapshot SHA-256: `dca7667806a05ed7d639d93fe8a2f578bfda62d5820c040298f8a4bd8e7264c7`.
- Implementer's correction archive SHA-256: `0d0e0f7b34d0f6e55595d94b8a1cffe1f1523ef3ab8cde54769d54660b62fbf6`.

The final formatting delta was independently read: one extra alignment space on the added allowedAppends entry; the test file was byte-unchanged. The baseline and candidate hashes and complete final textual diff were independently verified. Original independent report, original normal FAIL stdout/result, and the unchanged whole-main hash were rechecked and remain exactly the identities recorded above.

No corrected test outcome is claimed here. The actual subsequent affected checks, source-selection checks and compile-only result must be recorded separately by the parent, with the original first FAIL retained. This source-only correction does not change physical/manual execution authority.
