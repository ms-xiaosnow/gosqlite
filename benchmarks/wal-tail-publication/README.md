# Finite public WAL tail-publication elapsed profile

SOURCE_ONLY / proposed pins HOLD. No parser, formatter, compiler, source guard,
pure test, benchmark, physical publication or native qualification was run for
this candidate. This directory is a measurement index, not a runnable command.

The only physical root remains `TestManualPublicWALTailPublication` in
`sqlite/wal_tail_publication_manual_linux_test.go`, behind the existing Linux
manual build tag and first-statement authorization environment gate. Keep its
exact four sequential cases and one invocation/count=1, with verbose=true; no wrapper, `go run`,
new benchmark entry, automatic repetition, subprocess or controller is supplied.

See [the retained manual procedure](../../docs/testing/wal-tail-publication-manual.md)
for explicit independent review, exact source/binary identity, hardware/CPU/OS/
ext-family FS/4096-sector/Go 1.27.2 profile, bounded metadata, complete logs,
retirement rules, qualification and separate manual cleanup.

## Measurement contract

- Prepare options, context, connector, fixture, preflight and hashes outside the
  interval. Place `time.Now` immediately before the actual `Open`, `OpenContext`
  or `Connector.Connect`; `time.Since` is the next statement after return.
- `public_open_with_tail_publication_elapsed_ns` is total public call latency,
  including admission, lock/scan, publication and constructor retirement. It
  cannot establish separate scan/publish/fsync/recovery-only latency. Ordinary
  writes/reopens are correctness checks and never recovery timing samples.
- The deterministic fixture is two 4096-byte main pages, two committed WAL frames
  and one complete uncommitted frame, little-endian checksum, table rowid/IPK only.
  Record every byte/ledger/manifest digest and `seed=not_applicable_deterministic_fixture_v1`.
- `input_provenance=synthetic_encoded_committed_prefix`;
  `pre_crash_commit_ack=not_present`. This is not an actual interrupted/crashed
  business database or a real pre-crash successful Commit receipt.
- Fixture writes and verification warm caches: warm/unspecified OS cache, no
  eviction. Each public entrance has n=1. Do not pool entrances, claim cold cache,
  calculate P95, claim a regression, or use these values to close all WAL recovery
  performance requirements.
- Refusal controls have separate `refusal_attempt_elapsed_ns` and zero qualified
  samples. Case/invocation end-to-end timings are labeled separately.

Raw `WAL_ELAPSED_ATTEMPT` records never qualify. Only after all old assertions,
unchanged retained inputs, all four cases and one-time root owner retirement may
the in-memory reducer emit qualified publication candidates. Close errors,
pending owners, failures, identity mixing, incomplete or invalid timing fail
closed. The six pure scalar test roots remain NOT_RUN.

The final `WAL_ELAPSED_REPORT` is only a
`QUALIFIED_CANDIDATE_EXTERNAL_TERMINAL_REQUIRED`. Acceptance additionally requires
natural exit=0, complete outer stdout/stderr, exactly one final report, exactly
six raw attempts, top-level plus exactly four subcase PASS, and independently
bound source/binary/profile/retained evidence. Missing, truncated, partial,
hanging or non-natural output is UNKNOWN. Never extract successes from it.

All original artifacts survive success and failure. No physical/native result
is claimed. Actual crash acknowledgement and interrupted/restart recovery
fixtures/controllers/receipts and other recovery-profile contracts remain
separate gaps; this source slice does not complete M3/M10 or all of §11.3.
