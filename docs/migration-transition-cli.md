# Offline transition-bundle CLI

`gosqlite migrate pgembedding transition` generates a JSON collection of review
artifacts. It opens no database, connects to no PostgreSQL server, and never
executes generated SQL. Mutation and post-install SQL remains commented. All
approval, capability, dependency, source-fidelity and operational gates stay open.
A successful command means artifact generation succeeded, not migration approval.

From the repository root:

```sh
go run ./cmd/gosqlite migrate pgembedding transition \
  --catalog testdata/pgembedding-0.3.6/catalog.json \
  --options examples/migration-transition/options.json
```

Add `--report new-review.json` to create a new private-mode JSON artifact instead
of printing it. Existing paths are refused with `O_CREATE|O_EXCL`, never overwritten.
A write/close error can leave a partial report; atomic file publication, durable
report storage and automatic cleanup are not promised. Review the returned error
and do not treat a partial artifact as a complete bundle. No apply/resume flags
are accepted in this dispatch.

Options use strict JSON, capped at 64 KiB, 8192 tokens and depth 8. Every field in
`TransitionOptions` and every nested `TransitionIndex` is explicit, including
zero/empty inapplicable fields, `max_bytes:0` for the default generation cap, and
an explicit index list. Duplicate/unknown/missing keys reject. The checked-in
options example supplies a full valid retained-array/cast proposal with explicit
index retuning; copy and review it rather than assuming old pg_embedding defaults
are valid target settings. `DecodeTransitionOptions` validates JSON structure;
`GenerateTransition` separately checks the catalog-dependent semantics.

The catalog decoder has its own bounded version/identity contract. Input paths
are trusted local files; blocking/mutable special files or hostile namespace
substitution are outside this CLI contract. Context is checked before reading,
during generation and before report creation. Arbitrary filesystem stalls cannot
be interrupted by context alone.

Without the new flag, the core retains the original retained-array/cast and typed
shadow-column APIs and output unchanged. The explicit `--full-table-evidence`
route reads `FullTableOptions` plus a strict identity-bound PG16 supplement and
generates a bounded whole-table copy/swap with supported dependency recreation.
See [the full-table guide](migration-full-table.md) for runnable synthetic examples,
all fields/limits and the preservation/gating matrix. No online migration, actual
privilege change, old-service restore or live PostgreSQL validation is performed.
See migration-transition.md for exact phase and rollback boundaries. Source review
corrected identifier capture in float32 verification and refuses known inherited
source parents. A catalog's missing inventory cannot be treated as proof that no
external dependencies exist.
