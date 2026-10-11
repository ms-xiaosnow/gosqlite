# Public rollback Open integration on current published source

Result: fresh source/model/compile slice PASS; frozen for independent review.
Physical recovery / enabled real Open / production bridge remain BLOCKED / NOT_RUN.
No full ordinary/oracle regression or release/production acceptance is claimed.

## Candidate and reproduction

- Published baseline: `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`, tree `d735409ebd1c43d90ee1650c8a7c2a084e270674`.
- Frozen candidate: `public-open-integration-freeze-v1`, tree `75c074548e8b5208beb06e6ead314d835cd16f4f`.
- Checked fresh-baseline replay: `public-open-integration-evidence/replay`, identical tree.
- Source patch SHA-256: `4285b5bae09bcdb515a484b777eeb858f8ad5572d3f808917a89a17dba68080f`.
- Exactly 16 owned paths; baseline 2962 files, final 2969 files.
- Full inventories cover every file byte hash, size and POSIX mode. `goal.md`,
  go.mod and go.sum remain unchanged. Donor V2 identity remains unchanged.
- No main, Git index/object/commit, remote, CI or Library change by this worker.
  Tree identities are computed using in-memory Git blob/tree serialization.

Files: identity.json, owned-paths.json, source.patch, base/frozen/replay inventories
and SHA lists. Reproduction scripts: identity.py, freeze.py. Guard and production
hunks were individually reviewed and applied in that order, with exact patch and
command logs retained. No historical whole-file source overwrite was performed.

## Current-source correctness review

See behavioral-audit.md for the complete selected/helper chain audit. The V1 full
production review and V2 five-file guard-only review were read independently;
neither was treated as the integrated-tree result. Thirteen relevant current
ownership/parser/path/error dependencies are byte-identical to their V1 source
base (dependency-source-comparison.json), and were reassessed in this integration.

The nil-default optional policy is copied before Open admission/canonicalization
and connection storage. Connector URI restrictions precede its captured copy.
Three scalar recovery bounds retain defaults 64 MiB / 65536 records / 1 TiB,
without altering connection budgets or default/encrypted behavior. Only writable
plaintext rollback on Linux is eligible. No CLI/DSN switch was added.

Exact private resolved-path journal presence plus fully completed, diagnostic-
free initial Reader retirement is required. Joined/wrapped/generic errors and
pending owners cannot authorize replay. The public admission transfers once and
cannot be reused. Recovery would reacquire its own ownership and validation; only
success permits one fresh, separately charged, no-create read-only probe. All
errors retain original and subsequent causes/tokens and return zero metadata.
Historical ErrRecoveryRequired causes CodeIOErr precedence, even if physical
recovery already succeeded before later cancellation/probe failure. Later SQL
cannot invoke recovery, and no automatic cleanup/replay retry was introduced.

The recovery implementation is unchanged except its reachability comment. Broader
journal formats, WAL publisher/tail reconstruction, retained-directory, backup
and backends were not included. Cooperative path assumptions remain explicit.

## Test changes and guard limits

Existing main_test.go adds six inert AST assertions only. The sole changed old
reader_acquisition_test.go expectation now names finishReaderProbe rather than
cleanup.begin in its exact defer string; every ordering/ownership assertion is
retained, with the bridge's begin-first/call-free-checker boundary asserted by new
source tests. No old assertion was weakened or skipped. All other old tests match
the baseline bytes. New donor memory/source tests receive only additive typed-nil,
no-Is/As/Unwrap traversal and option-copy ordering assertions.

The prevention guard was integrated and audited FIRST. It detects recovery
function values, enabled field references, and recognized unkeyed Options via
same-file aliases/defined types/dot imports. It is not a complete scope/type or
call-graph proof: inferred element and cross-file alias forms can be missed,
shadowed names can false-positive. All selected helpers were reviewed and do not
use these omissions. The separate Rename/rename selector expansion and OS raw-
namespace gate correction are absent from 63c2 and were not silently imported.

## Actual check results

Pinned Go 1.27.2; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off,
GOFLAGS=-buildvcs=false; task-local GOPATH/GOCACHE; GOMAXPROCS=2 and -p=1.
Default /tmp with GOTMPDIR unset. Before each command, original >=1GiB /tmp,
>=2GiB workspace, >=10000 free inodes and writable checks passed. The lead
released compiler capacity after its separate authorized standard cache
maintenance; this worker did not clean, relocate or alter resource thresholds.

13 command stages naturally exited 0; last selected race finished
2026-10-11T07:42:37.528973+00:00. checks.exit=0; checks.finished records final
post-check inventory completion. Exact UTC/argv/cwd/CGO/resource/exit evidence is
commands.json and individual stage logs. No test was skipped or unmatched.

- Pinned version and gofmt.
- Ordinary source-reference guard PASS before tests.
- CGO0: 3 exact guard + 4 pager + 4 sqlite source/model tests PASS.
- CGO0 scoped vet: testscope, pager, sqlite, driver PASS.
- CGO0 full-package build PASS (compilation only).
- CGO0 production dependency closure: empty CgoFiles PASS.
- Source guard rerun PASS before race.
- CGO1 race: same exact 11 source/model tests PASS.

No Go source changed during or after checks. Complete pre/post-check inventories
are equal. Two documentation files subsequently received actual result wording;
identity.json records that exact docs-only difference before final freeze.
Fresh base archive + git apply --check + apply reproduces the full frozen tree.
Independent final-tree review and lead combined ordinary gates remain separate.

## User documentation and outstanding qualification

Topical docs/rollback-recovery-open.md and additive Chinese manual section 18
name the actual public API, flags/options, budgets, URI behavior, failure codes,
restricted combinations and future review steps. docs/progress.md records this
bounded result. The public physical harness is still missing; existing internal
manual_recovery && linux / GOSQLITE_MANUAL_RECOVERY fixture does not exercise the
public bridge. No invented runnable public recovery test is supplied.

Existing historical matrix/manual statements outside these three owned docs
must be reconciled by the lead when composing its concurrent documentation slice;
they are not overwritten from the old donor. All physical/native-failure/crash/
kill/process-exit/namespace/missing-selected-file/wrong-key/tamper/interrupted-
publication/rekey gates remain NOT_RUN. No native OS or CI runner, ANN V4, 41-target
fuzz or unknown blocked operation was attempted, repackaged or routed elsewhere.
No concrete edit/check was denied in this source integration.
