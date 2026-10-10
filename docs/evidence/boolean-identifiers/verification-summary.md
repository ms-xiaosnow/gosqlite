# Boolean identifier parser slice: frozen, not independently accepted

2026-10-10 UTC. Base: immutable sibling affinity-substitution-freeze-v1.
Production ownership: internal/sql/parser.go only. Public AST layouts, binders,
planners, storage and function registration remain unchanged.

TRUE and FALSE are ordinary SQLite ID spellings, absent from the pinned keyword
hash table. The parser now accepts them wherever its ordinary names are accepted,
including result/table aliases, schema names, index names and DML target columns.
Function calls and qualified names take precedence over Boolean fallback; bare
unquoted forms keep the existing Literal.Boolean AST representation and binder
column/alias precedence. Quoted forms remain columns; NULL stays reserved and
retains literal semantics. No other keyword fallback has been broadened.

Executed from this exact source: full internal/sql normal and tagged oracle
suites, selected native Boolean/relational/ordinary-index/expression/affinity and
driver suites, corresponding existing ordinary oracle cases and new DML oracle,
selected race suite, vet and CGO_ENABLED=0 build ./... all passed. These are
scoped checks, not a full ./... test pass. Every suite starts with testscope.
Exact reproducible commands are in run-checks.sh; each final result has its own
exit file. Baseline parser regression is red; candidate parser tests pass.

The unchanged strict new 2,021-case oracle matrix FAILED with 40 variants of one
inherited LEFT JOIN alias-root boundary: zero or NULL result aliases referenced
as an entire ON term eliminate all rows in SQLite but leave null-extended rows
natively. Independent diagnostic on the immutable baseline reproduces it for
ordinary alias z and quoted true/false. The exact failing cases and logs remain.
This is a real wrong-result boundary; no nominal complete oracle PASS is claimed.
The original two AS true/AS false WHERE cases now agree with SQLite. A separately
owned follow-on correction is required before treating this as publishable.

The first parser probe had an incorrect expected parameter-name assertion for
?1/?2; fixed to existing public Parameter representation. The initial DML probe
included ordinary REINDEX, which the engine already supports only for extension
indexes. That unsupported ordinary-index maintenance action was removed from
the supported DML lifecycle check, not implemented or relabeled. Initial probe
source and failing output are preserved. CREATE INDEX and parser REINDEX-name
acceptance are covered; ordinary REINDEX execution is still outside scope.

Behavioral safety audit: scope-audit.md. Only pure-memory syntax/scalar checks,
intact normal SQL fixtures and readonly/:memory: pinned oracle processes were
executed. No prohibited physical fault or lifecycle test was run. No main-tree
edits, commits or publication occurred.
