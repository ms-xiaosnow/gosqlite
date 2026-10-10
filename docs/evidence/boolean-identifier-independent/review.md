# Independent Boolean identifier and LEFT-ON provenance review

Date: 2026-10-10 UTC. Verdict: the three-file production delta is accepted within its documented scope. No blocking production correctness issue found. One nonblocking evidence wording correction is recommended below. This is not an overall baseline or full-project PASS.

## Finding first

**P3: prepared-statement reuse was described as prepared-AST reuse.** Candidate `docs/evidence/boolean-identifiers/alias-on-verification.md:46` says “database/sql prepared AST reuse.” In `driver/driver.go:173–177`, Prepare parses for validation but stores only the SQL string; `stmt` at lines 253–257 has no parsed AST, and `QueryContext` at lines 271–277 forwards that string to DB.Query, which parses again (`sqlite/sqlite.go:420`). Therefore the supplied driver test proves prepared-statement and parameter reuse, not same-AST immutability. Change the wording to “prepared-statement reuse.” The independent `TestIndependentAliasSameASTAndBudget` added only in the review copy separately establishes same-AST immutability across six typed parameter plans, and passes normally and with race instrumentation. This closes the acceptance evidence gap without editing frozen bytes.

## Immutable inputs and ownership

- Baseline: `affinity-substitution-freeze-v1`, not main. Full manifest: 879 files; SHA-256 `8b060026749d944fc027840e834a840659566789668292a93902e6b1cc5728be`.
- Candidate: `boolean-identifier-freeze-v2`. Full manifest: 970 files; SHA-256 `2d65674d219f87c89191826bef43168e264509dbfd4433dd21df1940b4f7ae39`.
- Both complete file sets and every digest verified before and after; no extra/unlisted files, missing files, or mismatches. Before/after manifests are identical.
- The complete baseline/candidate difference exactly equals `combined-v2-owned-paths.json`, SHA-256 `bf9435ec10a2bf820149a4c7a908ea9cdf17d68776f875497b73adbafa32d1b6`.
- Patch `combined-v2.patch` SHA-256 `9b56e68bba211c9c1e0e883340941602fbbba6c6c82b0b8e2c4b86c9deb1636b`; `git apply --check` passes against an independent baseline copy.
- Only production files are `internal/sql/parser.go`, `sqlite/relational_bind.go`, and new `sqlite/relational_alias_on.go`. No planner/rank, storage, AST-layout or production dependency change is in this delta.
- Isolated candidate copy: `boolean-provenance-independent-v2`; isolated baseline copy: `boolean-provenance-independent-baseline-v2`. Only new independent tests were added. Candidate-copy inherited files remain byte-identical to the freeze. No production edits, staging, commits, uploads or publication occurred.

See `hash-verification-before.json`, `hash-verification-after.json`, and `verify-frozen-inputs.py`.

## Source-backed semantic review

The pinned oracle and official source files were independently rehashed, not trusted from the candidate report. Oracle binary SHA-256 is `470775f6ada29ccfb2d3046ac2b4c975df95a9c20b782332d4e8ba6b6233e6c6`; version/source ID is `3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`. All six source hashes match the supplied pins; exact values are in `independent-source-pin.json`.

1. TRUE/FALSE are not entries in `tool/mkkeywordhash.c`. The parser delta removes exactly those reserved-name exclusions and processes calls and qualified references before bare Boolean fallback (`internal/sql/parser.go:188–196, 745–788`). Quoted identifiers remain columns; unqualified unquoted values retain the existing Boolean-marker AST representation. Reserved NULL behavior and all other reserved exclusions are unchanged. Native and parser tests cover mixed case, qualified wildcard, calls, quoted spellings, DDL/DML target names, ambiguity and rejection boundaries.
2. Official `select.c:437–454, 653–659` annotates original ON trees with EP_OuterON before putting them into WHERE. `resolve.c:68–105` replaces a direct explicit result-alias reference by a copy of its projection expression; the original ON flags are not retained. The lookup at `resolve.c:653–695` requires an explicit result name and follows physical-column resolution. `where.c:6950` splits top-level AND, and its false-WHERE bypass at lines 7005–7047 treats unannotated terms as ordinary predicates. This supports the narrowly modeled behavior independently of anecdotes.
3. Candidate LEFT joins bind only the explicit-alias map (`relational_bind.go:201–203, 269–277, 352–356`). The first explicit duplicate alias wins, implicit result labels cannot preempt it, and `scope.bind` resolves physical/qualified columns first. Direct unqualified roots are extracted only through top-level AND (`relational_alias_on.go:20–63`). Operators such as unary plus, comparison, NOT and nested OR preserve ON placement. Parentheses alone do not create an operator. Plain FALSE/NULL and implicit SELECT labels are not promoted.
4. Original full ON binding and forward-source validation happen before extraction (`relational_bind.go:356–376`), preserving the declared unsupported-forward-reference boundary even with WHERE 0 or LIMIT 0. Physical ambiguities remain eager errors. Bound projection expressions and retained ON terms continue through validation/function registration and predicate cloning; promoted terms enter the existing WHERE/startup pipeline.
5. The extractor's Binary assertion is safe for the parsed supported AST: original AND nodes are retained by RewriteChecked; only alias leaves can expand. It neither modifies original nodes nor overwrites bound nodes. The planner starts with a shallow Select copy but constructs bound expression trees and new AND nodes. A direct same-AST probe verifies structural equality against a freshly parsed pristine AST after each plan.
6. Work is charged before each extraction visit; name resolution is charged; retained promoted terms and reconstructed AND nodes are memory-charged. Independent in-memory probes force work-limit, memory-limit and pre-canceled failures inside extraction itself. These are propagated, and the original native post-error reuse/cancellation tests also pass.
7. Literal-zero AND folding was specifically probed because official `parse.y:1354` invokes `sqlite3ExprAnd`, whose `expr.c:1164–1185` can remove an alias before ON annotation. The inherited native parse-time folding preserves that behavior; ON z AND 0 and nested variants were not incorrectly promoted.

## Results and exact command scope

All commands use the absolute pinned SQLite oracle and Go 1.27.2. Environment:

```
R=/workspace/scratch/5e06a74e5f65/durable-restart
GO=$R/toolchains/go1.27.2/bin/go
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
GOCACHE=$R/go-cache GOPATH=$R/go-path
SQLITE_ORACLE=$R/toolchains/sqlite-build-3540000/sqlite3
```

Each following command is immediately preceded by `$GO run ./internal/testscope`; the guard and behavioral audit both passed. Normal stages use CGO_ENABLED=0; race stages use CGO_ENABLED=1 for compiler instrumentation.

- `$GO test -tags oracle ./internal/testkit -run '^TestBooleanIdentifierOracle(Matrix|DML)$' -count=1 -v`: PASS; unchanged strict 2,021-case matrix plus intact DML lifecycle.
- `$GO test -tags oracle ./internal/testkit -run '^TestBooleanAliasOnOracleProvenance$' -count=1 -v`: PASS; unchanged 748-case matrix.
- `$GO test ./internal/sql ./sqlite ./driver -run '^Test(BooleanIdentifier|BooleanAliasOn|DriverBooleanIdentifier|DriverBooleanAliasOn)' -count=1 -v`: PASS.
- Same package/selection with `$GO test -race ... -count=1`: PASS.
- `$GO test -tags oracle ./internal/testkit -run '^TestIndependentAliasOn' -count=1 -v`: PASS, 1,957 additional supported cases, full values/storage classes/error outcomes; 1,086 root/operator/empty-right cases and 871 multi-source/parameter/name/error cases. The latter includes 540 multiple-LEFT cases, 288 bound parameter cases and eight binding value/type controls. Numeric, REAL, TEXT, BLOB and NULL are compared as distinct Go types, using quote-mode oracle parsing.
- `$GO test ./sqlite -run '^TestIndependentAliasSameASTAndBudget$' -count=1 -v`: PASS; six repeated same-AST plans and extraction-local work/memory/cancel failures. Same selection with `-race`: PASS.
- `$GO test -tags oracle ./internal/testkit -run '^TestIndependentInheritedAliasBoundary$' -count=1 -v`: candidate PASS, baseline intentionally FAILS three of four comparisons. Baseline returns a null-extended row for direct 0/NULL alias roots where SQLite returns no rows; the +alias control matches. Exact minimal SQL is in `inherited-baseline-v2.txt`. This independently confirms the bug existed for ordinary z and quoted true before the parser change.
- `$GO build ./...`: PASS, CGO_ENABLED=0.
- `$GO vet ./internal/sql ./sqlite ./driver`: PASS.
- `$GO vet -tags oracle ./internal/testkit`: PASS.

Reproduction scripts are `run-reviewed-suites.sh`, `run-independent-probes.sh`, and `run-baseline-comparison.sh`. Independent test sources are saved as `independent-matrix-supported.go.txt`, `independent-ast.go.txt`, and `independent-baseline-probe.go.txt`; copy them into their corresponding packages in a fresh review copy, never the frozen directories. Logs and exit codes are retained next to the scripts.

## Failure preservation and harness classification

- The original 40 mismatches, strict 2,021-case matrix source, initial oracle source/output and initial parser output remain byte-identical to parser-only freeze-v1. `historical-preservation.json` verifies this. No original failing case was removed or expectation weakened.
- The candidate's documented initial parameter-name assertion correction is a harness expectation correction, not a production fix. Its first DML inclusion of ordinary REINDEX is a pre-existing unsupported boundary; removal from the supported DML lifecycle does not establish ordinary REINDEX support. Initial failures remain in the frozen evidence.
- The expanded alias harness initially bound quoted-looking `0` as INTEGER rather than TEXT through shell token handling, producing 12 harness mismatches. Both initial and corrected evidence remain intact. My independent parameter checks avoid the shell quote heuristic by inserting explicit typed SQL values into temp.sqlite_parameters and verify `SELECT ?1,typeof(?1)` for all eight binding types/values.
- My first 1,146-case exploratory matrix returned 60 differences, all explicitly unsupported scalar COLLATE syntax, already documented in compatibility.md. The other 1,086 cases matched. Its full initial source, raw output and nonzero exit are preserved. The final supported matrix excludes only that unsupported construct; `independent-classification.json` accounts for all cases. This is not a zero-mismatch claim for the exploratory matrix.

## Limits

No broad `go test ./...` was run or claimed. The separately identified baseline ANN/rank false-WHERE regression remains outside this candidate; neither the baseline nor the whole project is declared green. No planner/rank change was included or accepted here. Full internal/sql and broader selected suites in the frozen author evidence were reviewed as supplied evidence, not represented as independently rerun in this review.

This acceptance is limited to the supported Boolean-identifier grammar and explicit LEFT-ON alias-root provenance behavior against pinned SQLite 3.54.0. It does not establish general optimizer error-order equivalence, arbitrary join reordering, forward-source support, scalar COLLATE, quoted-string name exceptions, derived/RIGHT/FULL/USING/NATURAL joins, ordinary REINDEX, full SQLite compatibility, durability or fault-injection behavior. All lifecycle work was ordinary intact correct-key success; the prohibited physical/lifecycle fault classes were not executed, including in CI.
