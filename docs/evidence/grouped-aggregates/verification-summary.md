# Grouped aggregate owner verification, v1

2026-10-10 UTC. Base immutable aggregate-combined-lead-verify tree
8d7a0fcc5a2e3a83850ea780b64b403047677568. All base files were verified from
aggregate-combined-lead-all-sha256.txt before copying to grouped-aggregate-work-v1.
No applicable AGENTS.md was present in the task/ancestor source directories.
The complete goal.md and SQL/Boolean/binding/budget source contracts were read.

Selected normal aggregate/grouped/relational/Boolean-identifier/parser and driver
checks PASS; new grouped pinned-oracle checks PASS; the same selected normal and
new-oracle checks under race PASS. Whole-tree vet/build PASS; core Linux arm64,
Darwin arm64/amd64 and Windows arm64/amd64 cross-builds PASS. go mod verify PASS;
production sqlite/driver CgoFiles closure is empty. All 45 existing exported
sqlite/driver struct definitions are text-identical to base; go.mod/go.sum remain
byte-identical. Full command/output/exit evidence: run-focused-v1.sh,
focused-checks-v1.txt and focused-checks-v1.exit, ending 20:09:49Z.

Independent review and lead combined acceptance are still required. The base's
unrelated rowid/grams fallback reason assertions are known to fail in the full
suite and are being corrected separately. Neither those tests nor their owned
source files were edited. No complete default/oracle/race repository test pass
is claimed from this isolated owner check.

## Preserved initial findings and corrections

- Original parser capability-negative failures were recorded unchanged, then the
  exact SELECT sum(x) FROM t GROUP BY x, SELECT count(*) FROM t HAVING count(*)>0,
  SELECT x FROM t GROUP BY x, and SELECT true FROM false GROUP BY true statements
  passed complete pinned label/value/class comparisons. They now have positive
  parser assertions; unrelated negatives, including grouped wildcard, remain.
- The first grouping implementation kept only its first numeric key. Oracle
  exposed MIN/MAX selecting a different INTEGER/REAL representative. Dedicated
  source-guided retention now distinguishes canonical computed keys, the needed
  source cells of one representative, and nested key-expression evaluation.
  Exact mixed-class/multiple MIN/MAX/CASE/root-versus-nested probes remain.
- The initial new alias spelling AS key was outside the established reserved-
  identifier profile. It was changed to AS gkey; no parser keyword boundary was
  broadened for this test.
- Initial NUL tests incorrectly used raw CLI quote-mode text as an oracle for
  complete TEXT bytes. SQLite's documented shell behavior truncates this display.
  NUL cases remain tested through hex/typeof with the original values and group
  counts; non-NUL representation probes still compare the full native values.
- An initial new error-timing expectation assumed ORDER BY k DESC required a
  second sorter. The pinned source/CLI showed full matching group-key order can
  directly emit the last key and stop before a later erroneous group. Native
  key directions/sorter elision now follow that rule; the exact successful query
  is retained, and ORDER BY k+0 keeps the distinct real-sort error assertion.
- The new ROLLUP grammar test initially caught acceptance as an arbitrary scalar
  function spelling. The parser now explicitly rejects grouping-set ROLLUP/CUBE
  entry syntax; no implementation or compatibility claim is made for them.

All failed outputs are retained, including initial harness-expectation mistakes
rather than portraying them as engine failures. No existing unrelated assertion
was weakened. Behavioral restrictions and exact new-test source audit are in
scope-audit.md. Central documentation additions are provided as append fragments
for reconciliation with the lead's newer tree, not stale whole-file overwrites.

## Corrected v2, 2026-10-10 20:22:15Z

Independent v1 review found two genuine supported-query error-timing gaps:
finite general ORDER/LIMIT projected later losing candidates, and rowid-unique
grouping scanned later input predicates that SQLite's stream never visited.
The original 508-case matrix had 9 top-K failures; the expanded 316 checks had
84 further top-K and 8 rowid failures, with required row-2 lookahead and DESC
controls retained. Those original logs remain in the review evidence; unchanged
expanded source copies are retained here as text.

V2 adds grouped-only stable LIMIT+OFFSET top-K admission before remaining scalar
projection, and proved single-table physical-rowid grouping with required
qualifying-row lookahead, including native reverse cursor reuse. It does not
alter or claim to fix older general-sort projection timing. Physical-rowid
plans explicitly report grouped_rowid and source stats reflect early stopping.
An additional owner 140-case rowid matrix caught an overbroad experimental NULL
placement elision: SQLite still sorts ORDER BY id NULLS LAST. That failing run
is preserved as grouped-v2-expanded.txt; the natural-NULL-only proof is restored
and all cases now pass. Mismatched ordering remains a real sorter.

The unchanged independent expanded 824 comparisons pass on the corrected owner
worktree. The new owner quota/cancel tests cover losing candidate admission,
ORDER-key failures, replacement lifetime admission, no partial answers, normal
shared/ordinary source lifetime and exact two-row lookahead. The full selected
owner normal/race/oracle/build/vet/cross-build/module/CGO checks pass again;
commands are run-focused-v2.sh and output focused-checks-v2.txt (exit 0).
All 45 exported sqlite/driver structs and both module files remain identical to
base, confirmed again by api-and-module-layout-check-v2.txt.

Only docs/evidence were updated after the checked source. Immutable v2 complete
hashes, owned paths and full-base/v1-to-v2 patches live beside the freeze for
independent review and lead composition. Independent v2 acceptance is still a
required gate; this owner verification is not a full repository acceptance.
