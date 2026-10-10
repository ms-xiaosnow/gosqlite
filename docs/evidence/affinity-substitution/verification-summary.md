# Candidate verification and handoff

Base: accepted Boolean v2, full manifest digest
`be6ece6401dca41264cc1846ba4674db0977626724af25798d26351aa6bdc96a`.

The root cause and bounded correction are described in `../../affinity-comparisons.md`.
Only `internal/sql/eval.go` production code changes: declared BLOB is removed
from the two TEXT-to-no-affinity coercion conditions. Formatter, predicates,
planner, budget/context contracts, public APIs and file/type layouts are unchanged.

Final corrected aggregate (`aggregate-checks-final.txt`, exit 0):
- CGO0 all internal/sql normal and oracle tests passed.
- CGO0 scoped sqlite/driver Boolean, affinity, relational, ordinary-index,
  expression, vector-review and grams checks passed.
- CGO0 scoped testkit affinity/Boolean/relational/expression oracle checks passed.
- Separate CGO1 race selection passed for internal/sql, sqlite and driver.
- Ordinary vet passed, and oracle-tagged testkit vet passed separately.
- The source-reference guard ran before each suite, with behavioral audit.

The full-value/class affinity results are 2160/2160 historical cases plus
7968/7968 expanded matrix queries. The final expanded test uses real SQLite
parameter bindings, retaining parameter storage class in projected checks.
The earlier exploratory matrix used literal reference SQL; final acceptance
does not rely on that substitution. Normal write/rollback/reopen checks pass.
Pinned source and VDBE evidence distinguish BLOB affinity from no affinity.

Historical evidence is retained. The unpatched accepted-Boolean evaluator
overlay intentionally fails 18 scalar assertions. The initial aggregate
passed every test stage but exited 1 because untagged vet included the
oracle-only testkit package. The corrected full aggregate exits 0; no engine
change or weakened expectation was needed for that harness correction.

Independent affinity review is pending; the lead will assign a dedicated
reviewer after this worker returns. No shared/main SQL or Boolean frozen
source was changed. No integration, commit, upload or publication performed.

Remaining boundaries: no full optimizer/index-order/error-order equivalence,
no unsupported SQL expansion, no non-Linux runtime or physical-fault acceptance.
The existing planner boundaries in the Boolean guide remain explicit.
