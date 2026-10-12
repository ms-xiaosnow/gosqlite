# R2 final formatted composition and finite validation review

2026-10-12 UTC. **CLEAR for the final source composition and the exact 13-command validation template.** No blocking defect found. This is a static review, not an executed model/guard/compile result or native qualification. No additional review layer is requested.

## Bound identities

- Formatted manifest: `misplaced-write-format/final-path-sha256-manifest.json`, SHA-256 `fae80cd29f7a027839d7429e954056b0956c6d7a772a5ae22c5d80f50d949a0e`.
- Final integration identity: `misplaced-write-integration/SOURCE-IDENTITY.json`, SHA-256 `55200eba71c1b1ed412704a0a433ca0e59fc5f44dc35e651629e48fa6c0f6b27`.
- Candidate tree recorded by the lead/identity: `02e421a5c803175d5497ce2e3603b9ac2b6de724`; prior R5 tree `8751a3bed18341a299c060ff494c437d5176b024`; base `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`.
- Validation template: `run_misplaced_write_validation.py`, SHA-256 `0c78621e787e630cb0c9b4b03e1125e47d320ca2a390e123e170fd542cf13ab0`.
- Original semantic source review remains `misplaced-write-independent-review.md`, SHA-256 `facb10a83ac7f7007f0537e18179d4be71680a04d120f8dff71f706c8b11fcba`.

## Formatting and composition

Independently recomputed all 13 formatted overlay hashes; all match the formatted manifest. Compared original reviewed files retained under `misplaced-write-format/before` with final overlay text. After substituting only the seven explicitly identified pin/full-source-snapshot pairs with equal sentinels, every Go file has identical non-whitespace text and identical ordered literal/comment spans. Both manuals are byte-identical. Independently rechecked that all seven final complete-source snapshots contain exact final source text. The recorded formatting delta contains only alignment/line breaks; semantic literals, guards and mutant values are unchanged. The empty anchor-adaptation diff is consistent with the final text and unchanged mutant tables. The corrected recorded anchor inventory is 290 (33 existing generic, 24 added generic, 233 SQL), not the stale 270 in the earlier handoff; anchor presence is not a passing AST test.

The integrated W matches the final overlay on the eleven non-overlap paths. The only two differences are the pre-existing R5 independently reviewed whole-main literals. Each overlay contains exactly one old `6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2` value; replacing just that value with current main SHA-256 `1d9f41ed857bf64a3d2913f96fee3409a173f036df753efd81a7a9190ee2536a` gives the following independently calculated hashes, both of which match final W and its identity:

- `internal/testscope/rollback_fault_test.go`: `259a3060ddeea65e2933dde29fcb3d9539a0efc48f3e1249be43575321b28dfb`.
- `internal/testscope/rollback_fault_sql_test.go`: `4d3f75d9b7e09332b28d83e0faf317d59c916f7ef7afce9f79e8f6fa9b1c8981`.

Current `internal/testscope/main.go` independently hashes to that preserved R5 value. R2 does not change main or weaken its assertions. The R5 model-name correction uses `RekeyQueryPlan`/`RekeyQueryText`, avoiding the old rollback SQL guard's protected names; its existing reviewed correction remains separate evidence. The lead reports R5 focused gates complete and other R5 source preserved. This review does not reinterpret those results as new R2 results.

The original review's conclusions for actual full-payload misdirection, native error precedence, proof-before-recovery, retained observations, exact cause, generic/A5 bodies, OLD-only replay, and manual specificity continue to apply to these formatting-only/composition changes.

## Exact finite command closure

The template schedules exactly 13 commands:

1. One read-only gofmt `-l` over the eleven identity-listed Go files.
2. One normal selected test command.
3. One selected race command.
4. One ordinary sourceguard invocation.
5–7. Three `go list -json` source selections.
8–10. Three `go test -c` compile-only images.
11–13. Three `go tool nm` reads of those exact images.

Static declaration comparison, independent of the template's descriptive label, confirms its exact selector sets: 54 roots in `internal/testkit/rollbackfault`, 13 in `internal/testkit/rollbackrestart`, and 24 affected roots in `internal/testscope`, with no missing, extra or duplicate root names in the respective intended files. Thus each normal/race command selects 67 pure-model roots plus 24 affected AST roots. The selector is anchored; unrelated R5 test roots are not selected. Package compilation still includes the whole package. The selected packages contain no source `init` or TestMain declaration. The unchanged memory WriterAt root is deliberately reused from its existing evidence rather than rerun, as specified by the lead.

The template pins the same fixed GO/gofmt hashes, uses local/offline module/toolchain settings, one nonblocking queue lock, CGO_ENABLED=0 outside the reviewed race support, `-mod=readonly`, `-buildvcs=false`, `-p=1`, no vet, count=1, parallel=1, and timeout=0. It checks the final source identity and release identity before work, retains source snapshots/command/environment/output hashes, fails on source changes, and stops on a failed gate. Resource observation failures wait for the active process naturally rather than killing or orphaning it. No command launches a generated manual image.

The three source/compile/nm targets are closed:

- `fault-sqlite`: `./sqlite`, `manual_fault_rollback`; exactly one manual role, `TestManualRollbackFaultSQLLedger`.
- `fault-pager`: `./internal/pager`, `manual_fault_rollback`; exactly one manual role, `TestManualRollbackFaultRecovery`. Its source selection explicitly includes the manual bridge and excludes the default bridge.
- `restart-sqlite`: `./sqlite`, `manual_fault_rollback,manual_restart_rollback`; exactly four manual roles, the SQL ledger root plus `TestManualRollbackRestartController`, `TestManualRollbackRestartWriter`, and `TestManualRollbackRestartRecovery`.

Source selection rejects the R5 rekey manual external test file and requires the exact manual-role sets above. The output image paths are used only as compiler destinations, hash inputs and nm inputs, never argv[0]. The normal/sourceguard commands have no manual tags or manual enabling environment variables.

## Result and nm parsing

The JSON test parser requires package PASS for all three expected packages, nonempty root selections, no named FAIL/SKIP, no duplicate named terminal records, and exact per-package selected root sets. These are planned runtime checks, not observations made by this review.

The nm parser is anchored per line, accepts defined address/type/symbol rows and addressless U rows, preserves the full symbol spelling, and requires each source-selected Test/Fuzz/Example root symbol exactly once. It then compares the entire exact package/potential external-package root set against the source-selected set. The inventory is specifically Test/Fuzz/Example roots, not a claim to enumerate every symbol category or prove execution. The expected manual-role cardinalities are 1/1/4. Compilation and symbol presence confer no manual/physical correctness result.

## Status and stopping boundary

This reviewer read sources, diffs, manifests and existing review evidence, performed static text/hash comparisons, and wrote only this report. No W/overlay/source/runner/release file was changed. No Go, formatting, Go parser/AST runner, test, compiler, sourceguard, target image, Commit, recovery, process controller or CI was executed by this reviewer.

The lead may proceed through the already authorized identity-bound 13-command workflow after creating its exact execution release and acquiring the free queue. Preserve failures if any; do not broaden selectors or execute images to investigate by convenience. The finite facility endpoint remains these exact validation results and its directed checkpoint. Actual native miswrite/recovery, hardware/media/power-loss and physical durability remain NOT_RUN.
