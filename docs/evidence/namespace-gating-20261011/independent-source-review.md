# Independent source review: current-P0 namespace gating integration

## Verdict and limits

SOURCE REVIEW PASS. No blocking source finding in the authorized six-path delta against the current P0 base. This is a fresh review of tree `01918609a55cc149c5155451da40f7389854db06`, not a reuse of the older candidate's PASS.

No Go command was invoked. Actual Go AST execution, unit/race tests, the current candidate's target-selection matrix, and foreign compilation/symbol checks are **NOT_RUN by this reviewer**, pending separate compiler authorization and release. All raw namespace/native physical runtime and every restricted workflow remain **NOT_RUN**. The source verdict is not a dynamic, full-suite, native, or physical-safety PASS.

## Independently established identity

- Base: `public-open-combined-freeze-v1`, tree `442a7104dc780ddb20826e5ee4f1250b291f2b24`, 3,004 files.
- Candidate: `namespace-gating-integration-freeze-v1`, tree `01918609a55cc149c5155451da40f7389854db06`, 3,007 files and 143 directories excluding the root.
- Patch SHA-256: `18ba34e90283080cbc0a1ab1ab1e213945d2b9a05168acb33221498faaba4ee0`.
- Independent Git-object SHA-1 computation reconstructed both tree identities from filesystem entries and blob/tree object headers. No Git command, index, or repository write was used.
- An independently written strict unified-hunk parser checked every original line and every hunk offset/count. Applying those outputs to a reviewer-owned copy of the current base reproduced the candidate exactly, including all file SHA-256 values, complete bytes, paths, Git modes, file POSIX modes, and root/subdirectory POSIX modes.
- Both input inventories were rechecked unchanged after review.

Exactly three existing paths changed:

1. `internal/encstore/native_darwin_test.go`
2. `internal/encstore/native_windows_test.go`
3. `internal/testscope/main.go`

Exactly three paths were added:

1. `internal/encstore/namespace_manual_darwin_test.go`
2. `internal/encstore/namespace_manual_windows_amd64_test.go`
3. `internal/testscope/native_namespace_test.go`

The other 3,001 existing files remain byte/mode identical. Of the base's 366 test files, 364 remain byte/mode identical. No other current source, test, documentation, runner, dependency, or CI byte changed. These complete-current-base comparisons rule out replacement with the historical full tree.

## Complete moved functions and retained tests

The independent checker masks Go literals/comments, balances function-body braces, and compares entire source spans. This is a byte-preservation check, not an executed Go AST test.

- `TestWindowsEncryptedRelativePublicationAndRetainedRoot`: original and normalized moved-function SHA-256 `017622ac62e4b69e0396fb85f9404ef71372fc9aceb506e1f6c29ea8a05fd88b`.
- `TestDarwinEncryptedRelativePublicationAndParentLifetime`: original and normalized moved-function SHA-256 `140d779476126902745a89299b992a2d82d3a011d27ace0bdf945b055b50d245`.

Each moved function has exactly one declaration in the candidate encstore source. Removing only the inserted first-statement environment gate yields the complete original function byte-for-byte. No assertion, cleanup, operation, error check, or comment was weakened or omitted. The gate is exactly:

    if os.Getenv("GOSQLITE_MANUAL_NAMESPACE") != "independently_authorized" {
        t.Skip("restricted raw namespace publication test: excluded from ordinary CI")
    }

It precedes `t.TempDir()` and every native namespace operation. Neither new file introduces a package initializer, helper workflow, or additional test. The sentinel value is a source safeguard, never authorization to run restricted work.

The retained native files equal the originals after only removing the named complete function and these now-unused import lines:

- Windows: `bytes` only.
- Darwin: `bytes`, `context`, and `github.com/ms-xiaosnow/gosqlite/encryption`.

The resulting separating blank line is the direct consequence of deleting the function source span. Every other existing function is byte-identical; per-function hashes are recorded in `body-equivalence.json`. Windows retains `context`, which the current `windowsForbiddenProvider.Acquire`, `Prepare`, and `BeginMaintenance` references still use. This avoids the historical patch's incompatible import-removal hunk.

## Build selection: source conclusion only

Both new files begin with the exact constraint `//go:build manual_namespace`, followed by a blank line.

- Windows: old `native_windows_test.go` has `windows && amd64`; new `namespace_manual_windows_amd64_test.go` carries the same OS/architecture restrictions through its filename and additionally requires the manual tag.
- Darwin: old `native_darwin_test.go` has the Darwin tag and suffix; new `namespace_manual_darwin_test.go` retains that suffix and additionally requires the manual tag. The existing Go iOS-to-Darwin match is retained, not newly broadened.
- Default and oracle source selection therefore lose exactly the two moved tests on their applicable platforms.
- With `manual_namespace` or `oracle,manual_namespace`, the new declarations have the same platform conditions as before, plus the manual requirement.

This analysis was checked against the installed pinned toolchain's primary source, `go/build/build.go`, specifically `matchTag` and `goodOSArchFile`. The excerpt and source SHA-256 are saved in `go-selection-primary-source.txt`. No `go list`, Go binary, or toolchain command was executed. The fresh actual 47-target/four-tag-set matrix remains NOT_RUN by this reviewer.

## Current recovery guard is retained

Reversing only the addition of `"Rename", "rename"` to the generic selector-name switch reproduces every byte of current-P0 `internal/testscope/main.go`. Consequently the current recovery-name references, same-file option-type propagation, aliases, unkeyed option checks, dot-import logic, strict manual first-line exemptions, and existing regression tests are retained.

The new rule inspects every selector AST node, independently of receiver spelling and call context. Source inspection therefore supports detection of direct calls, captured method values, method expressions, chained receivers, and selectors within defer/go calls. Existing alias-aware import handling is untouched. OR constraints do not equal either exempt first line and remain scanned.

The added `native_namespace_test.go` operates on inert source strings. Its real calls are the parser/checker and ordinary testing operations; the shown namespace/rename expressions are never executed. It checks direct lowercase rename, a captured lowercase method value, a generic uppercase Rename selector, and the exact manual-tag exemption. The three added files also match the historical accepted additions byte-for-byte, independently checked after validating historical patch SHA-256 `65d746d7db22a9a2abfc3450af5220d571a953bd84679ac0c3ab9f0eb581de65`.

A separate Python lexical scan found no remaining nonliteral `Rename`/`rename` selectors in non-exempt candidate test files. This is supplemental source evidence only and is explicitly not presented as execution of the Go guard.

Known limits remain material:

- Generic selector names can conservatively flag unrelated Rename methods.
- This is not complete type/call-graph analysis, a sandbox, or proof of physical safety.
- Bare helper calls, differently named wrappers, dynamic/reflection calls, and future APIs still require behavioral review.
- Exact manual-tag exemption does not prove a body has an environment gate or is safe. The two moved bodies/gates were independently checked here.
- Historical native-scope review results support provenance only. They do not establish tests or compilation on this current tree.

## Executed work and boundaries

Executed: source reads, independently authored Python byte/hash/inventory checks, independent exact-hunk replay into this evidence directory, and source-only selection/guard analysis.

Not executed: any Go command; manual binaries; native namespace/fault/recovery/crash/kill/tamper/wrong-key/interrupted-publication workflows; ANN V4; the new 41-target fuzz workflow; any unknown interrupted chain; any new OS runner. No main/frozen source, Git state, remote, CI, Library, or publication was modified. Reviewer writes were limited to this owned evidence directory and its replay copy.

## Evidence

- `identity-result.json`: exact current-tree scope and status.
- `base-inventory.json`, `candidate-inventory.json`: independent complete file/path/mode inventories.
- `patch-replay.json`, `replay/`: exact-hunk replay results and reproduced source tree.
- `body-equivalence.json`: entire moved-function equality and retained-function hashes.
- `accepted-additions.json`: historical addition byte hashes, provenance only.
- `source-guard-and-selection.json`: source findings and explicit execution limits.
- `go-selection-primary-source.txt`: installed toolchain selection-source excerpt.
- `review_source.py`, `source-review.log`, `source-review.exit`: independently authored checking code and successful source-check run, exit 0.

Any later approved runtime-free Go checks require a separate report/addendum with their actual invocation, result, and unchanged-tree identity. They must not silently turn this source-only result into a runtime claim.
