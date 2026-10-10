# Follow-on LEFT-ON explicit-alias provenance correction

2026-10-10 UTC. Independent acceptance is pending. Parser-only v1 remains intact
in sibling boolean-identifier-freeze-v1. Its strict 40-case failure and the
baseline ordinary-alias z reproduction are preserved, not replaced with new
expectations. This follow-on adds only sqlite/relational_alias_on.go and a small
sqlite/relational_bind.go delta to production.

## Source-backed behavior and bounded implementation

Pinned select.c marks original ON expressions with EP_OuterON. resolve.c's
resolveAlias copies the already-resolved projection expression over a direct
alias reference; the replacement lacks that ON annotation. where.c treats these
unannotated terms as ordinary WHERE terms. The VDBE evidence shows a startup
bypass for constant false/NULL aliases but null extension for literal ON 0 and
operator-wrapped ON +alias. Row-dependent direct aliases likewise filter rows,
whereas ON +alias remains a join condition. This is observed SQLite 3.54 behavior,
not a claim about standard SQL or a new interpretation of all LEFT joins.

The helper visits only top-level AND conjuncts of the original ON. For a whole
unqualified reference to an explicit result alias, after full binding and
physical-column precedence, it removes that bound term from ON and appends it
to the existing WHERE pipeline. The existing startup compiler handles constants;
ordinary row predicates run after the join. A containing operator (+alias,
alias=0, alias OR 0, etc.) retains its ON position. Parentheses alone introduce
no operator. Literal FALSE/NULL, physical/qualified columns and implicit labels
such as SELECT FALSE are never promoted. An explicit-only LEFT-ON alias map
also prevents implicit SELECT labels from overriding a later explicit alias.

Original ON validation and forward-reference rejection happen before extraction.
Extraction and retained metadata are charged to the relational work/memory
budget. Existing predicate cloning keeps per-occurrence column identities; the
public AST layout is unchanged. Function registration, binding validation,
projection eagerness, normal Close ownership and literal LEFT-null extension
are not bypassed. No general join reordering is implemented.

## Evidence

- Unchanged original 2,021-case identifier matrix: PASS, 0 mismatches. All 40
  historical failures are resolved without editing the matrix source.
- Additional 748 strict provenance cases: PASS. Includes 0/NULL/TRUE/FALSE,
  integer/NULL/text prepared bindings, empty left source, operator parents,
  same-name physical precedence, right-side references, duplicate/implicit
  result labels, row-dependent aliases, eager projection errors, startup error
  suppression, LIMIT 0 and multiple LEFT joins.
- Native repeated parameter calls, database/sql prepared-statement reuse,
  pre-cancellation, work quota, post-error reuse and forward-reference validation:
  PASS. These are ordinary intact fixtures and pure-memory limits.
- Full internal/sql normal and tagged scalar oracle, existing selected
  Boolean/relational/ordinary-index/expression/affinity and driver suites,
  selected ordinary oracle, race, vet, and CGO_ENABLED=0 build: final commands
  are in run-alias-on-checks.sh; all final exit files must be zero for acceptance.

The first expanded parameter probe passed '0' to shell .parameter set without
protecting the SQL quotes. The shell therefore bound INTEGER 0 rather than TEXT
'0', producing 12 harness-only mismatches. The initial source/output remain as
alias-on-initial-probe.go.txt and alias-on-expanded-first.txt; corrected quoting
preserves the real TEXT binding and all 748 cases then pass.

## Remaining limits

This does not expand any keyword besides TRUE/FALSE. Unrelated keyword fallbacks,
quoted-string name exceptions, unknown functions, schema-qualified FROM, derived
relations, USING/NATURAL/RIGHT/FULL joins and ordinary REINDEX execution retain
their existing supported/unsupported boundaries. General optimizer error-order
or left-WHERE pushdown differences remain separate. No broad SQLite equivalence,
full ./... test pass, durability or fault-injection acceptance is claimed.
The three unrelated rank false-WHERE failures reported by the parent are outside
this candidate and are being corrected separately.

Final exact-source aggregate completed: all nine stage exit files are zero.
The final pass includes the last prepared-AST/forward-reference additions.
Independent review remains required before integration/publication.
