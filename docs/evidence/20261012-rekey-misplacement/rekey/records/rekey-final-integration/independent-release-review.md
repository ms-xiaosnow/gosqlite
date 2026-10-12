# R5 final integration: narrow independent release review

## Verdict

**CLEAR for the exact final source identity and bounded validation runner identified below.** No blocking defect was found in the formatting, exact pin/snapshot release, mutant-anchor synchronization, or the eleven-command validation plan. This supplements `rekey-guard-independent-review.md`; its finite-proof and native/physical limitations remain in force.

- Base commit: `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`
- Candidate tree: `165bd1e232e41f57b5f6a22e0657b86104e4901e`
- `SOURCE-IDENTITY.json` SHA-256: `019b42af2cbf11ff4ae654948af2f793726db028dcc1f647042e90136462f3be`
- `run_rekey_guard_validation.py` SHA-256: `7f4db59c9ddb5bf1492704013e789b50688fd24faa79840ca70e5ee3f0a25de9`

All 20 identity entries were independently hashed from the current worktree and compared with the corresponding stored candidate-tree blobs. All matched. The staged diff contains the expected 20 files, and the worktree has no unstaged source changes. This review changed no source, pin, index entry, or execution-release file.

## Final delta reviewed

The preserved before/after-format sources, format-only diff, authorized one-pass gofmt receipt, token-spelling comparison report, independent-body comparison report and final synchronization diff were read. The formatting transition is consistent with whitespace, layout and separator changes to the already reviewed source. The two models retain the parent's already validated identities:

- contract.go: `b9a179e2ca3f82678842f86a3e330b52cc916097408ccc1716fa74ebf08a0e02`
- contract_test.go: `ac32dd1977e8283aa66dec82f66f2635125ef0d16ef52c321303ee3d9e847159`

The reported earlier eight-model normal PASS is reused by the plan; it was not rerun or independently executed by this reviewer.

Independent text/literal comparisons established:

1. The final guard differs from the preserved post-format guard only in its two supplemental maps, `rekeyGuardPins` and `rekeyGuardShapes`.
2. All nine released pins equal the actual final source hashes, and all nine decoded source snapshots equal those complete final source files byte-for-byte.
3. All 24 native and 12 bridge independent body literals remain unchanged from the reviewed guard freeze.
4. All 43 mutation anchors are present. The only test-source change after formatting is the paired false/true `Result.Pass()` anchor's whitespace alignment. The rejection still flips false to true; no test, semantic check, root or mutant was removed.
5. Each of the six old whole-main pin owners differs from its preserved input only by one exact old-main-digest replacement. Other pins and the R1 WAL-model pin `465813a5c6c4b1324ad6b4ed4850585c24d145e5e3a7a4f7e22ca1d178cd0902` are retained.
6. Main remains the previously reviewed `1d9f41ed857bf64a3d2913f96fee3409a173f036df753efd81a7a9190ee2536a`. The complete `inspectSource` declaration is byte-identical to the base declaration and hashes to `98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199`.
7. The prior exact Wait-goroutine correction, two main hooks, single startup OR predicate, and 14 × 14 read-only-WAL propagation matrix remain in the reviewed source. The final release adds no new native permission or blanket exception.

## Eleven-command recorder review

The runner was read as source, not imported or executed. Its fixed plan is:

1. Read-only gofmt listing of the scoped Go files.
2. Exactly 61 named source/AST guard roots, normal.
3. The same 61 guard roots, race.
4. Exactly eight pure value-model roots, race.
5. Exactly two previously reviewed correct-key healthy default sqlite roots, normal.
6. The ordinary source-reference guard command.
7. Default Linux source selection for encstore/model/sqlite.
8. Manual-tag Linux source selection for the same packages.
9. Default Windows source selection for the same packages.
10. Linux manual-tag sqlite `test -c` compilation to the evidence image.
11. `go tool nm` reading that image.

The three selectors have 61, 8 and 2 distinct exact names respectively. They match the finite reviewed proposal; the five R5 manual role roots are never included in an execution selector. Only compilation and nm receive the manual image. There is no invocation of that image, shell command, manual authorization environment marker, wrong-key experiment, process-exit scenario, archive/restore scenario or unrelated test campaign in this runner. The two healthy roots remain ordinary test execution with correct credentials and default observation stubs; that execution is distinct from the source-only/model checks.

The runner requires an identity-bound explicit execution-release receipt; checks the fixed Go/gofmt identities; takes the nonblocking coordinated queue lock; uses fixed environments with network/module discovery disabled; preserves command/stdout/stderr/result evidence; checks tracked source identity before and after each command; and stops before later commands after a failed gate or latched resource issue. It waits/reaps a started command if resource observation fails rather than discarding ownership. A failing test, missing root, skip, source-selection mismatch or malformed nm output cannot become a PASS.

Source-selection checks distinguish default and manual encstore implementations, require the correct external-test file selection, and retain the exact two-file pure model package. The nm reader preserves the entire symbol tail, supports the addressless U form, uses full-root matching and requires the exact selected test symbol set plus absence of unrelated manual test roots. It does not execute symbols or infer a physical result from symbol presence.

## Limits and handoff

CLEAR is a source-review conclusion for these exact bytes and this bounded plan. The lead owns the execution release and actual run. This reviewer ran no Go, gofmt, parser/scanner executable, compiler, tests, source-selection command, sourceguard or native image. Actual validation results remain pending at this handoff. Any runner/source mutation after these identities needs its relevant delta checked; no wider proof framework, additional campaign or native physical acceptance is implied.
