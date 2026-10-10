# Full-table transition candidate handoff

Status: corrected bounded ordinary implementation awaiting independent re-review. Not live
PostgreSQL/Neon qualified; not a completed M9/release certification.

Base: `69008bf658611318154a112178913c693b679a2b`
Frozen tree: `../full-table-review-freeze.zC6p1M` (relative to the working repository)
Owned paths: `owned-paths.txt` (33 paths)
Overlay: `owned-overlay.tar`
Overlay SHA-256: `ecd01b2650c9eb54e013fb7e2c40ef15aa72c4056071d62a3c8ddbb9af1349dc`
Individual source hashes: `source-sha256.txt`; verified in `freeze-identity.txt`.
Reconstruction: `git archive <base>` into a new directory, then extract the owned
overlay there. No unrelated shared-worktree worker source is included.

## Delivered

A separately versioned PG16 full-table evidence API and strict CLI route, genuine
all-column typed table creation/copy/binary verification, retained backup/name
swap, supported immediate PK/UNIQUE/scalar CHECK/default/btree/ANN/owner/ACL/comment
restoration, explicit same-sequence ownership transfer, and exact internal row-type,
TOAST and legacy operator-family support-row dependency classification. All mutation
SQL stays commented; unknown or unsupported facts withhold every mutation proposal.
No original retained-array/shadow input/API/artifact schema changed, and all their
existing golden artifacts passed unchanged. No database/server connection or SQL
execution was performed. No staging, commit, CI change or publication by this worker.

Guide/matrix: `docs/migration-full-table.md` in the candidate; original transition
and CLI guides link the new route. Fixed synthetic example and all 14 artifact
goldens are under `examples/migration-full-table`.

## Final verification

Pinned Go 1.27.2; GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off, shared existing
GOCACHE/GOPATH. Testscope and a behavioral audit preceded every suite.

- CGO_ENABLED=0 filtered unit tests: PASS, 26 top-level tests plus subtests.
- CGO_ENABLED=1 same filtered `-race` suite: PASS. CGO is only race instrumentation.
- CGO_ENABLED=0 focused `go vet -buildvcs=false` and `go build -buildvcs=false`: PASS.
- Standalone example/CLI generation: PASS, output byte-identical.
- Existing retained/shadow full golden comparisons: PASS.
- `git diff --check` for owned changed paths: PASS.

Exact package selection:
`./migration/postgres ./cmd/gosqlite ./examples/migration-full-table`
Exact test filter:
`^(TestTransition|TestFullTable|TestMigrateTransition|TestMigrateFullTable)`

Logs: `unit.txt`, `race.txt`, `vet-build.txt`, `cli-example.txt`, `example.json`,
`cli.json`. The frozen git archive has no Git metadata, so static and go-run
commands explicitly disable VCS stamping; source identity is separately recorded.
`audit.txt` explains audited behavior and the retained earlier failures.

## Still gated / not exercised

No full-table online/batched/concurrent swap; no FK/exclusion/deferrable-trigger,
view/function/user-trigger/rule/RLS/partition/publication/security-label/extended-
statistics recreation, custom collation/opclass/storage options, arbitrary
expressions, identity/generated/dropped/typmod columns, non-owner ACL chains or
shared/external sequences. Exact supported and gated contracts are in the guide.
Source/target server availability, all live results, application cached OID cleanup,
role privileges, source completeness, capacity, downtime, and restore feasibility
remain unresolved. Backup sequence defaults are explicitly removed and OWNED BY
moves to the new table. Post-COMMIT backup is stale; no automatic reverse migration
or cleanup is emitted. No restricted physical workflows were executed anywhere.

Parent/reviewer should inspect the entire final family/TOAST classification and
full-table SQL ordering, not rely on passing results from before that refinement.

## Independent review corrections in this freeze

The previous CJbRPe freeze and full-table-evidence directory are preserved. Its
passing tests were insufficient to establish PG16 semantic correctness. Review
found and this candidate corrects:

- Typed integer literals now cast quoted signed text, covering int2/int4/int8
  minima without PostgreSQL unary-minus/cast precedence overflow.
- Constraint-backed archive index names cannot collide with an existing source
  CHECK/other constraint; source indexes must be in their owning table namespace.
- Both row-type internal ownership links are mandatory even when omitted from
  both base and supplement. Standalone PK/UNIQUE requires connoinherit=true;
  primary indexes reject NULLS NOT DISTINCT.
- Explicit namespace OID/name records and a finite exact mandatory dependency
  address set cover source table/owned sequences, old extension and legacy
  members, method-handler, operator-function, family-method, ANN-opclass and
  support-row implementation links. Membership edges are captured for every old
  extension-owned AM/function/operator/opclass/family. Pinned builtin float_ops
  identity is 1970; its ordinary dependency row is not invented in the fixture.
- Database-wide event-trigger coverage is mandatory and the readonly collection
  reference queries pg_event_trigger. Presence, including disabled presence,
  remains unsupported and blocks all mutation proposals.
- Required dependency addresses are built once into a map reused for membership
  and mandatory presence checks, avoiding repeated closure allocations.

New regressions remove each captured namespace/implementation/family-method edge
from both inputs and bind the changed hash; all such omissions block. Source
namespace mismatch, constraint-name collision, missing individual/both row-type
links, event-trigger facts, invalid p/u connoinherit, primary NULLS NOT DISTINCT,
wrong float_ops OID and all signed minima are covered. Existing retained/shadow
goldens remain unchanged and pass.

Final unit/race/static/example logs cover the final address-map source. Earlier
passing corrected runs are retained with before-address-cache suffixes. The
temporary synthetic fixture writer passed testscope and its one focused test,
then was removed before final source hashes. No restricted workflow or database
execution occurred. No further source edits are pending; reviewer has final bytes.
