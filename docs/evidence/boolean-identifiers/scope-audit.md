# Boolean identifier scope audit

2026-10-10 UTC. Production ownership is only internal/sql/parser.go. The immutable
base is sibling affinity-substitution-freeze-v1. No planner/binder/public AST
layout or storage code is changed.

Every suite invocation first runs internal/testscope. The existing internal/sql
suite and its opt-in oracle tests are parser/scalar/math/valid memory tests;
runDiff uses the pinned CLI only against :memory: with normal process completion.
This inherited scope was also audited in the accepted Boolean/affinity evidence.
New internal/sql/boolean_identifier_test.go uses only pure-memory parser/AST,
custom evaluator function and malformed SQL/depth inputs. It has no file handles.

New sqlite and driver Boolean identifier tests use intact temporary databases
created by normal SQL, ordinary writes/indexes, queries, prepared bindings,
transaction rollback and normal close/reopen. New testkit oracle cases query the
same intact SQL-created fixtures readonly or use independent :memory: schemas.
The oracle is pinned by exact version and source ID; children always finish
normally. Unknown-function and invalid-column cases are SQL errors, not storage
faults. Helpers reviewVectorDB/Exec/Query and confOracle/confQuoted/writeExec are
normal owned temporary fixtures, quote-mode parsing and ordinary transactions.
The selected existing Boolean/Relational/OrdinaryIndex/Expression suites retain
the accepted behavior audit in docs/evidence/boolean-semantics/scope-audit.md.

No manual tags, physical tampering/corruption, missing-selected-file/wrong-key
probes, namespace replacement, crash/kill/process-exit/hot recovery, native
I/O/lock/unmap/close/Sync failure, short write, disk full or interrupted rekey
workflow is run. Compiler race instrumentation is not race injection.

Follow-on alias-ON correction: new boolean_alias_on tests add intact SQL fixtures,
readonly parameterized oracle children, canceled in-memory contexts and finite
relational work limits. They do not inject races or touch descriptor behavior.
The helper modifies only bound predicate placement for explicit result aliases.
No new source or lifecycle class is introduced by the follow-on suite.
