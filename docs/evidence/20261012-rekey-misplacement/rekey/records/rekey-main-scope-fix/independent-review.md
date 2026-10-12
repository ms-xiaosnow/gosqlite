# R5 main collector scope correction: narrow source review

**CLEAR for the two-file correction below.** No blocking defect found. This is source review of the actual correction, not a claim that the targeted rerun has passed.

The preserved first normal-run log shows 60 passing roots, one failing root (`TestRekeyGuardIntegrationDefault`), zero skipped roots and 786 named PASS events. Its two findings are consistent with `rekeyGuardMainSemantics` incorrectly applying the main collector's variadic-append rule to the distinct local `findings` collector inside `inspectSource`. That definite false rejection was missed by the earlier source review. Original failure evidence remains intact.

The fix restricts only assignment-state checks to nodes whose nearest enclosing `ast.FuncDecl` is the non-method `main`. Traversal crosses `ast.FuncLit`, so assignments in the real WalkDir callback and nested ordinary-source branch remain checked. Returning true for an out-of-scope assignment continues AST descent: the original full-file call inventory is preserved, including calls nested in such assignments. Main's append/reset checks, failure-bit checks, exact hook sequences and live placement checks are otherwise unchanged.

Independent byte comparison confirms that every byte outside `rekeyGuardMainSemantics` in the guard file is unchanged from the preserved before file. In particular, native `rekeyErr`/`capturedRekeyErr` assignment checks, process/Wait admissions, all other functions, independent critical bodies, pins and snapshots are unchanged.

The test diff adds only two double-repinned mutants in the existing `TestRekeyGuardIntegrationDefault` root: replacing main's R5 findings append with nil, and replacing the nested ordinary-source append with the ordinary result directly. Both remain within main's ownership under the corrected ancestor walk and must be rejected by the preserved state checks. All existing positives, negatives and test roots remain; no protection is removed or substituted.

Reviewed current SHA-256:

- `internal/testscope/rekey_interruption.go`: `02e4a370883e04f26a5d04678970a33056d3d08c20f74a96a14aa99a38e32a83`
- `internal/testscope/rekey_interruption_test.go`: `fdb0b19e44a966846963b498f62a8c70ac0d643e72726d0e013554cd87af3c7b`

The bounded continuation is the seven R5 guard roots normal, followed by the remaining nine original gates, preserving the first failure and reusing the 54 already-passing legacy normal roots. The later 61-root race gate still covers both old and new guard roots. The execution owner must bind the continuation to the updated source identity; the earlier pre-correction identity is not a final identity for these two files.

No Go, gofmt, parser/scanner executable, compiler, test, sourceguard or native operation was run by this reviewer. No source, pin or runner was edited. Only source/diff/hash and existing log reads plus this report were performed.
