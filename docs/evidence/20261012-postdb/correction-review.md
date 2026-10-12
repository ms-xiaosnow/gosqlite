# Post-DB four-line correction review

Status: CLEAR_SOURCE_ONLY for the exact correction below. No remaining blocking source finding in this narrow correction. The original failed normal gate and its earlier source review remain preserved; this does not claim a successful rerun.

Independent byte reconstruction confirms W's guard equals the retained old guard plus exactly two edits: expected-literal parsing now uses parser.ParseComments, and the fixed root literal includes the actual three-line retained-error comment before var diagnosticTexts. Both replacement anchors occur exactly once. This aligns the previously mismatched nested GenDecl.Doc without changing any executable caller/model/bridge behavior or relaxing the exact-function comparison.

The outer FuncDecl.Doc shallow-copy normalization remains unchanged. All four complete source snapshots and pins are untouched. All negative assertions are untouched. The other six integrated slice files, including the preserved current documentation header, independently match the failed gate's source-before.json hashes. No unrelated source changes were reviewed or proposed.

Exact identities:
- Old guard SHA-256: ac577f657d71a52426c100150b89b067a4d19410569a430f5b789828c7ef243b
- Corrected guard SHA-256: 0647edf67ae831b8d99b3caddf02fbf8e65380fc391405effe433145c5374969
- CORRECTION.diff SHA-256: 9657b46e931085ace33170183fa42ae2e87f708142af550ab2c7d11963b7848f
- Parent-supplied integrated tree: 449d420ca57e9ab7a454dcfdd33bc2d9aec8871a; no Git operation used to re-verify it.

The next bounded stage remains six steps: formatting check of this changed guard, normal testscope only, race on the same two pure/source packages, existing sourceguard, dual-tag sqlite compile-only, and read-only nm with the unchanged 454-required/18-forbidden root set. The unchanged rollbackfault normal result of 1,565 named passes may be reused under its original source identity; it is not a rerun. Race/sourceguard/compile/nm were previously NOT_RUN and need actual results. No manual image, native/fixture/Commit/recovery case, A5 controller, broad test sweep, or refactor is included.

This review performed source/evidence reads, exact text reconstruction and hashes, and this separate report write only. No source edit, formatter, Go/parser/test/compile/native execution, Git operation, or publication occurred. Physical qualification remains NOT_RUN.
