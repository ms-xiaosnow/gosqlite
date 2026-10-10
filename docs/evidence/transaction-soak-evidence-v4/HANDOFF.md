# Transaction/soak V4 final handoff

## Identity and checked integration

Root: `/workspace/scratch/5e06a74e5f65/durable-restart`.

- Immutable harness: `transaction-soak-freeze-v4`
- Harness tree: `65f0d9bece6732499899db84f6d4fc008b6810e3`
- Base: `plain-final-verify`, tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`
- All 1252 base files/modes unchanged; exactly 12 additions under `benchmarks/transactions`.
- Full baseline patch: `transaction-soak-evidence-v4/source-v4.patch`
- Full patch SHA256: `1e5d86e5b5f1643f3e20245048deda7c068630fade57d1d5f36d90e3ca33df4f`
- V3 delta: `transaction-soak-evidence-v4/v3-to-v4.patch`
- Delta SHA256: `e96e1ed7d518d56a51c9340604be3f416bbe4e4d06469d69a7bb453d09f1cd71`
- Full and delta checked replays: `transaction-soak-replay-v4`, `transaction-soak-delta-replay-v4`; both full file/mode inventories reproduce the exact V4 tree.

The runtime-only reader-fix composition is `transaction-soak-fixed-freeze-v4`, tree `73d79d0e5cacf82c55777ad9756214a1859e6e1b`. It applies the separately accepted reader patch SHA256 `ccc9bf728b3df6b0daa65562955ae54cc84fcdf690a9d8985d17888d740c3659`. The harness patch contains no native-engine changes. Full source/mode inventories and SHA256 manifests for both freezes and both replays are in the V4 evidence directory. V3 frozen hashes were rechecked unchanged.

No main/remote/goal/central-document/CI mutation, commit or upload occurred.

## V4 ownership/report correction

The strict V3 all-branch BUSY/cancellation classifier remains in place. V4 fixes the separate incomplete retirement reporting:

- Bounded discovery recognizes all exact public pending markers: plaintext, shared Open, shared Close and encrypted cleanup. Any SQLite CodeIOErr, poison/uncertain marker, malformed/excessive graph or opaque ownership envelope conservatively remains unverified. No custom Is/As/Error/Unwrap callback is used for this discovery.
- `recordFailure` immediately latches failed status plus `close_error_unverified_retirement` and retains the original error/token graph, even if no DB was returned.
- The ownership ledger and finalizer are installed before the first database Open. No-handle results have an explicit `no_database_handle_returned` label when genuinely benign; pending retirement instead keeps its unsafe label.
- `recordOpenFailure` only skips a wholly understood benign unsupported graph. Pending/IO/unknown joined diagnostics cannot become an unsupported skip. Later successful writer closure, pass or rejection cannot erase an unsafe latch.
- Failure rendering is independently bounded and avoids unknown callbacks, typed-nil panics and cyclic SQLite Error recursion. Original graphs remain retained through JSON emission.
- No automatic retry or second-nil Close retirement inference was added. References through report emission and ordinary process completion are explicitly not evidence that exceptional native obligations retired.

Every report now includes `harness_revision: "ordinary-transactions-v4"`. The performance comparator must require that exact revision, in addition to its existing status/invariant/identity conditions. V1–V3 schema version, workload version or zero closure fields cannot qualify them as corrected reports. Workload math remains unchanged.

## Preserved regressions and tests

The original four failed V3 pure-memory pending-marker controls remain unchanged at:

`transaction-soak-independent-review-v3/pure-memory-negative/benchmarks/transactions/independent_retirement_test.go`

Original probe SHA256: `519541f9c255f8eebe6c2f7457a7c7c34015fe0152b7ecf5a3ad20f55c964ada`.
Original failed log: `transaction-soak-independent-review-v3/negative.txt`, SHA256 `7d679b9592e49346f42fb866ffe6668b75fa2c985b083e68afda84700d6f3173`; `negative.exit` remains 1. V4 includes the same four semantic cases in a formatted test copy. The seven earlier classifier failures and V1 rollback ErrChanged evidence also retain their original identities. The V3 WAL 16 MiB duration-guard failure remains failed.

V4 selected controls cover the four pending markers, all earlier classifier negatives, nested retirement after clean writer Close, pre-handle IO/pending plus unsupported, guarded later skip/pass/rejection, strict benign unsupported graphs, original-token retention, opaque and typed-nil rendering, bounded cycles, and AST proof that finalization precedes both initial and reopening sqlite.Open calls. All exceptional cases are descriptor-free memory/error/report/AST tests. No native close/unmap/lock/I/O/Sync failure or other restricted workflow was induced.

## Exact verification

After behavioral audit `AUDIT.md` and `go run ./internal/testscope`, the frozen accepted-reader composition passed:

- Focused CGO0 normal tests, including ordinary lifecycle and cooperative cancellation
- Focused CGO1 race tests
- Focused vet and native CGO0 build
- Empty CGO0 CgoFiles dependency closure
- Full source hashes before and after execution
- Three predeclared fixed-64-attempt ordinary smokes, one per profile
- Independent Python reconstruction of every committed transfer and final model digest/generation

Pinned sibling Go1.27.2 and offline caches were used. Go tests used timeout0; runtime cancellation/closure was cooperative. No broad old-base suite, manual tags, unfixed-reader rerun, long or large profile. No new trial failure occurred.

All three smoke reports used rows 16 / readers 1 / batch 1 / seed 20261010, default 1ms reader pause and snapshot hold, and a 16 MiB data ceiling. These are fixed-count smoke measurements, not long-soak evidence or duration substitutes for earlier failed runs.

| Report | Active seconds | Commits | Writer BUSY events | Status |
|---|---:|---:|---:|---|
| `throughput-v4-rollback.json` |0.108698444|48|56|passed|
| `throughput-v4-wal-shared.json` |0.062026606|48|0|passed|
| `throughput-v4-encrypted-shared.json` |0.468003002|48|2|passed|

Each report has the V4 revision, zero close failures/unverified handles, unverified_retirement=false and normal verified closure. Raw clocks and counts remain in JSON. Shared-host timing is descriptive, not a confidence-qualified comparison.

Evidence in `transaction-soak-evidence-v4`:

- `checks.txt` / `checks.exit` (0); transcript SHA256 `35983601a735afcef25826894ae7bc1658c8ba81e5a9f116020e7e7d09ca75db`
- `report-model-verification.json`, SHA256 `ff1fcce506d167cfef664c0b402a6309287f6f2128a8ce9406b2f87b2dc02aa2`
- `freeze-inventory.json`, SHA256 `118d6ba6fa07ab2ed4f448472020c5e50fc480fc439bf7a0c66827c3f3d081fc`
- `fixed-composition-inventory.json`, SHA256 `0122c941d901323b8a36cb39e07b247411ec203a781256976969fc1842e3ba77`

All jobs finished normally at 21:36:39 UTC. The independent reviewer and performance owner have both exact patches and source identities; performance owner was told to wait for reviewer acceptance before another real paired trial. Independent review and whole-project acceptance remain separate from these focused gates.
