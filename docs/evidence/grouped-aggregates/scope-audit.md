# Grouped aggregate ordinary-scope audit

Reviewed behavior before executions: all new internal/sql tests use in-memory
SQL/AST expressions, bounded arrays and arithmetic only. sqlite tests create
owned intact fixtures through normal SQL, perform queries, close/cancel, enforce
work/memory limits, inspect accounting, exercise ordinary shared snapshots,
normal transaction BUSY and rollback, and normal checkpoint. Driver tests use
normal prepared statements, repeated named bindings, own transaction writes,
rollback and close in ordinary and shared modes.

The new tagged oracle file uses aggregateOracleFixture/confCLI/confNative and
confCompare from the previously audited global/readonly fixtures. Each official
CLI command creates valid schemas/rows or reads an intact file, runs to normal
EOF, and never uses CommandContext, kill or signal. Its VECTOR malformed-value
case is an ordinary SQL value outside the engine's typed declaration contract,
not database corruption. NUL is normal TEXT data, compared through hex/typeof
because the official CLI quote display truncates embedded-NUL text.

No restricted physical corruption, wrong-key, missing selected files, namespace
mutation/races, physical fault, crash/recovery, interrupted publication/rekey,
permissions or disk-full test is included. No manual tags are used. Testscope
is run before every suite; it supplements this behavioral review. Whole-tree
build/vet and cross-builds compile code without executing such workflows. Race
runs use CGO only for Go's race detector, never production SQL.

Existing aggregate, relational, Boolean identifier, parser and driver selected
checks retain their previously reviewed ordinary behavior. The four promoted
parser statements have exact new positive pinned comparisons; unrelated legacy
rejections are untouched. Baseline reason-regression tests are not edited or
run as an asserted pass. Lead owns the complete combined suite.

V2 corrections retain that scope. The unchanged independent 824 comparisons
were read before execution: only intact ordinary SQL fixtures, read-only CLI
queries, expected scalar integer overflows, and normal EOF/Close. New owner
rowid controls use the same fixture helpers. Top-K admission tests are
file-descriptor-free expression/array/context/quota tests; rowid forward/reverse
checks use ordinary SQL-created intact tables in ordinary/shared mode. No
physical fault or process control is introduced by reusing the native bounded
rowid cursor. New plan/stats assertions inspect one root source and required
lookahead only. The original v1 and expanded failure logs remain immutable.
