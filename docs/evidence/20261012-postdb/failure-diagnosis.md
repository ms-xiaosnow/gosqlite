# Post-DB normal-gate source diagnosis

The actual normal gate reports one failed root, `TestRollbackFaultSQLSourceContracts`, with `post-DB root exact one-shot captured observations and recovery decisions`. This supersedes any inference that the earlier CLEAR_SOURCE_ONLY review established test success. The earlier frozen review remains historical source-only evidence.

## Exact mismatch

The actual manual root contains a three-line comment beginning “Retain the untouched public error text…” immediately before `var diagnosticTexts [rollbackfault.MaxEvents]string`. The source is parsed with `parser.ParseComments`, so that comment is attached to the nested variable declaration's `ast.GenDecl.Doc`. `rollbackFaultSQLExactFunction` clears only the outer `ast.FuncDecl.Doc`; it leaves this nested Doc intact.

The independent fixed root literal omits that comment and is parsed with flags `0`, yielding nil nested Doc. `readOnlyWALManualCompact` uses `printer.Fprint`, which prints attached Doc comments. Its whitespace/semicolon compaction does not remove comment text. The resulting comparison therefore fails despite matching executable nodes.

A complete source-text diff of the expected root literal and actual root shows only the retained-error comment, a non-declaration comment before the post-Commit readback assignment, and alignment whitespace. The attached variable-declaration comment is the differing AST payload; this is not a caller control-flow or classification mismatch.

## Minimal correction proposed

Keep the exact-function assertion and every negative assertion. In the guard only:
1. Parse the fixed expected function literal with `parser.ParseComments` instead of `0`.
2. Include the exact three retained-error comment lines before `var diagnosticTexts` in the fixed root semantic literal.

Both independently constructed nested GenDecl.Doc nodes then have identical fixed content. No actual caller/model/bridge change, supplemental four-source repin, broad comment stripping, or relaxed semantic predicate is needed. Future actual gate execution must verify the correction; this diagnosis does not claim execution success.

## Evidence

- postdb-validation/02_normal.stdout — SHA-256 336f404538a003fd1c7071472a413f6d82f76e1fd73607d07088575c5eb20539
- postdb-overlay/internal/testscope/rollback_fault_sql.go — SHA-256 ac577f657d71a52426c100150b89b067a4d19410569a430f5b789828c7ef243b
- postdb-overlay/sqlite/rollback_fault_sql_manual_linux_test.go — SHA-256 c368296059cbf29a4756cbb760b3836c580d8615e25a7330f76d83ba97427037

This diagnostic pass used source/log reads and text comparison only. No Go, parser/AST runner, test, compiler, native/manual/recovery execution, W edit, Git operation, or publication was performed.
