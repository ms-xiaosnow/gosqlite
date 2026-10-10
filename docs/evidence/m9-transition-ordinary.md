# M9 D: ordinary offline transition evidence

Recorded 2026-10-10 UTC. This is the review-only PostgreSQL transition slice,
not full M9 qualification and not PostgreSQL execution evidence.

## Implemented boundary

- `migration/postgres/transition.go`: explicit options, source identity/subset
  checks, pinned index mapping/range checks, catalog/dependency/plan identities,
  bounded generation, blocked versus unresolved human gates, JSON renderer
- `migration/postgres/transition_render.go`: deterministic structured artifact
  assembly, full catalog evidence isolation, fully commented phase rendering,
  transaction metadata, content manifest, operator guide
- `migration/postgres/transition_sql.go`: read-only preflight, exact extension
  transition reference, typed shadow setup, atomic single-worker keyset batches,
  HNSW/eligible-IVF references, verification and application/rollback examples
- `migration/postgres/transition_test.go`: memory/text-only source-model tests
- `examples/migration-transition/`: compiling synthetic example and both golden
  phase bundles; no shared `cmd/gosqlite` or importer paths changed
- `docs/migration-transition.md`: executable local example commands, public API,
  supported/unsupported matrix, approvals and precise rollback boundaries

Both retained-real-array/cast and nullable-vector-shadow-copy approaches are
implemented. Both single-transaction and multi-transaction modes are generated.
Full-table typed copy is explicitly unsupported. No automatic approval,
CASCADE, source-column/table deletion/rename, cleanup, live writer or arbitrary
SQL body interpolation is provided.

## Source and safety audit before tests

The focused suite was manually read before invocation. It loads only the
synthetic catalog/golden text, modifies owned in-memory catalog/options structs,
generates strings, scans SQL tokens, checks bytes/hashes, marshals JSON and
checks ordinary canceled contexts. It never opens a database or executes the
generated SQL. The context-cancellation case is canceled before generation;
there is no interrupted migration, storage operation or publication test.

No test subprocess, crash, forced exit, injected I/O failure, namespace mutation,
physical tampering, wrong-key disk operation, interrupted rekey, recovery,
fault/restart or restricted qualification was run. The standalone example's
normal artifact generation used new directories and stdout; no output-failure
or interrupted-publication behavior was induced. The example explicitly reports
that failed artifact writes can leave partial output and claims no atomic
publication protocol.

`go run ./internal/testscope` ran successfully immediately before each focused
test invocation. That static source-reference guard is additional evidence,
not a substitute for this behavioral review.

## Exact environment and commands

Working directory: `gosqlite` repository root. Existing pinned toolchain and
already populated dependency cache, no network/module downloads:

```sh
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
export GOCACHE="$PWD/../gocache" GOPATH="$PWD/../go-path" CGO_ENABLED=0
GO=../toolchains/go1.27.2/bin/go
```

Final source-boundary commands and actual results:

```text
go run ./internal/testscope
Ordinary test source-reference guard PASS; behavioral review is still required.

go test ./migration/postgres -run '^TestTransition' -count=1
ok github.com/ms-xiaosnow/gosqlite/migration/postgres 0.194s

go vet ./migration/postgres ./examples/migration-transition
PASS (exit 0, no diagnostics)

go build ./migration/postgres ./examples/migration-transition
PASS (exit 0, CGO_ENABLED=0)

go run ./internal/testscope
Ordinary test source-reference guard PASS; behavioral review is still required.

CGO_ENABLED=1 go test -race ./migration/postgres -run '^TestTransition' -count=1
ok github.com/ms-xiaosnow/gosqlite/migration/postgres 2.432s
```

The race detector is a separate CGO-enabled test build. The ordinary production
package/example build remained CGO-disabled; the race command does not establish
a production CGO dependency. Full-repository aggregate testing is owned by the
integration lead and is not claimed by this focused record.

Initial build environment probes encountered an unwritable default home build
cache and an empty guessed module-cache directory. After selecting established
writable caches and offline dependencies, builds succeeded. These were build
environment errors, not exercised database/native-I/O failure scenarios.

The standalone example was actually executed for both:

```sh
"$GO" run ./examples/migration-transition -approach shadow -mode batches \
  -out-dir examples/migration-transition/golden/shadow-batches
"$GO" run ./examples/migration-transition -approach retain -mode single \
  -out-dir examples/migration-transition/golden/retain-single
```

Both initial runs reported `Generated 13 review artifacts;
status=review_required; no PostgreSQL SQL executed.` Following ordinary review
fixes, final JSON stdout runs regenerated the owned golden artifacts and the
focused suite above byte-compared all SQL phases. Those golden files include
the final full-catalog inventory and batch-row-count result.

## What the tests establish

- All four approach/mode combinations generate deterministic structured review
  artifacts. Reordering explicit index choices does not change plan identity.
- Every mutation-phase file tokenizes as trivia/comments only. The active
  preflight has no unquoted mutating SQL token and declares READ ONLY.
- Two complete golden phase sets match exactly: retained-array single transaction
  and typed-shadow multi-transaction batch migration.
- Read-only preflight includes available/installed versions, exact AM/dependency
  ownership, transitive dependency inventory, privileges, RLS/policies and full
  array-shape/NULL/nonfinite/zero-profile queries.
- Batch state uses SQL NULL initial cursor, includes all source NULL rows,
  compares prior progress and commits state with data. No MinInt64 sentinel,
  OFFSET, SKIP LOCKED or shadow-NULL progress filter is generated.
- Concurrent indexes are emitted outside explicit transaction wrappers and
  rejected with single-transaction mode. HNSW/IVF option bounds are enforced;
  IVFFlat L1 is blocked. Modern Hamming is never substituted for Manhattan.
- Missing data/coverage/version, unplanned legacy indexes, unhandled or forged
  dependencies, invalid old defaults, source identity drift and name collisions
  produce explicit blockers instead of proposed mutation bodies.
- Catalog definition/dependency changes change the bound hashes. Control-bearing
  SQL identifiers are rejected; quotes/backslashes/Unicode remain correctly
  delimited. Source definitions with CR/newline and arbitrary commands remain
  evidence-only JSON. CR cannot escape a review-only comment.
- Output caps, option validation, canceled/nil contexts and changed artifact
  hashes fail closed without external operations.

These are source/text/lexical checks, not a full PostgreSQL parser, PostgreSQL
server syntax check, live capability test, executed transaction or operational
rollback demonstration. Runtime expectations remain explicit review gates.

## Pinned source facts checked locally

The established upstream-reference tree was read, never compiled or executed.
No upstream implementation code was copied into production. The target SQL
defines real[]/vector casts, vector_norm and HNSW vector_l1_ops; target IVF source
and header establish lists bounds and per-query probes. The existing legacy
source fixture/pin is reused. All references use target commit
`f37c13f68b57d2c3472b2214fbcff699d6d34876` and source commit
`5d48508aeaeb86b9e9c630468ada7a124b905795`.

Locally calculated SHA-256 for the extra reviewed target files:

```text
a4ceca5931cb614b40bec09ce6994a57058513d9af612c15b1458a338134609d  sql/vector.sql
652309192fdd909048371ed24f5026ae0ebd1eecf253ca6512820ce114ea71e0  src/ivfflat.c
be00dd3383cc4921fa3e252c16b7bf1f6e418f32b8c8b08f2a0070cb76af399b  src/ivfflat.h
```

These are local reference-content hashes, not upstream-published checksum claims.
The current Neon migration chapter remains absent at the 2026-10-10 snapshot;
historical full-table/CASCADE guidance is not asserted to be current or implemented.

## Unsupported and remaining gates

No live PostgreSQL/Neon execution, actual server capability/privilege evidence,
real staging plans, operational extension restore, application deployment,
business/ANN validation or database rollback experiment was performed. Exact
legacy-distance comparisons must be captured before extension removal and use
declared tolerances; generated value/norm/NULL queries do not replace those gates.

The one-column/single-worker subset rejects unplanned affected indexes and
dependent objects. Full-table typed copy, generic dependency recreation,
online/dual-write maintenance, parallel batch scheduling, state adoption,
arbitrary source SQL rewriting and versions outside the pinned profiles remain
unsupported. Source inspector errors cannot be waived by target options.

Restricted physical durability/security/native qualification and independent
release review remain open and separate. This slice makes no production-ready,
whole-M9-verified or whole-goal-complete claim. Integration/publication requires
the parent's separate exact source review and durable-write authorization.
