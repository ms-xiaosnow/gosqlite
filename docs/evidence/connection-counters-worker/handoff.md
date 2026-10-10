# Bounded SQL connection counters: candidate handoff

## Result

Implemented native SQL `changes()`, `total_changes()` and
`last_insert_rowid()` with connection-owned state, explicit DML completion
outcomes, per-row last-insert tracking, no disk-format changes and unchanged
statement-specific native/driver Result values. Independent review is assigned separately; this candidate is ready for its verdict.

- Baseline: plain-final-verify, exact tree
  `7d3184f87165e11e12560884ec6e4f92c9bfd039`.
- Freeze: sql-connection-counters-freeze-v1, exact tree
  `7c8f7a266a63a713b22709268fdd4d25f1b875d1`.
- Exact owned baseline patch: source.patch, SHA-256
  `7a09d38be5ccb2fd18b11e3f74a1d57ba8b98e04ef7bd6a56491dd5b8ec0099c`.
- Full 1259-file byte inventory: all-sha256.txt, SHA-256
  `f05142b0c8e291c41614596e45013e3f87dc5ee4389792b9ea1671c14414710f`.
- manifest.json records every owned old/new hash, exact trees and isolated object
  store. owned-sha256.txt lists final owned hashes. patch-apply.txt verifies a
  fresh baseline+patch matches the complete freeze; work also matches.

## Semantic findings and bounded differences

Pinned public SQLite3.54.0 validates the distinctions: a failed multirow INSERT
can leave last_insert_rowid at the last inserted row even though its rows are
rolled back, changes resets0 and total does not increase; successful explicit
transaction/savepoint rollback preserves all counters; UPDATE counts unchanged
matched rows but never sets insert-ID; DDL/control/index maintenance do not
contribute. Automatic DML counts publish after successful commit; explicit Tx
statements publish before COMMIT. Validated writer-promotion BUSY resets changes;
automatic commit BUSY does not add total but retains an inserted rowid. Explicit
COMMIT BUSY leaves previous completed-statement counters alone.

There is an explicit reviewed boundary: native first-snapshot BUSY occurs before
catalog validation, whereas SQLite can sometimes prepare from cached schema.
Lead explicitly accepted this bounded behavior without unknown-state poisoning.
Native preserves previous counters on that admission failure, rather than
guessing whether valid DML or missing table/column SQL executed. The oracle test
asserts this cached-schema divergence separately; it is not counted as parity.
Existing active-Rows/session lease restrictions and statement-local native context
cancellation remain, with no claim of full SQLite cursor/interrupt semantics.

Private DB layout changed by three int64 counters. Exported configuration/result
fields and Result values are unchanged. Result.LastInsertId remains this INSERT's
final ID, or zero for another statement, while SQL last_insert_rowid is persistent
connection state. No native Counter getter API was added. Functions return INTEGER,
are non-deterministic, and share the operation/Rows context under the existing DB
lock without recursive locking or a global side table. One explicit fixed context
node per operation carries the pointer; no per-call registry/closure is built.

Unsupported parser/catalog profiles, notably general triggers/FK/conflict policies,
views, virtual tables, WITHOUT ROWID, INSERT SELECT and RETURNING, remain unsupported.
Their accounting is not claimed. Engine-owned extension index maintenance is
excluded from direct user-row counts. Counter function DEFAULT expressions remain
under existing default-expression rejection.

## Verification

Run environment: exact ../toolchains/go1.27.2/bin/go; GOTOOLCHAIN=local,
GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false;
GOCACHE=../go-cache, GOPATH=../go-path; CGO_ENABLED=0 for production checks and1
for separate race checks. SQLITE_ORACLE=../toolchains/sqlite-build-3540000/sqlite3.

- selected-checks-v2.txt/.exit: selected native+driver+SQL tests, new oracle
  histories, focused race, new oracle-race, affected vet and build PASS, exit0.
- frozen-checks.txt/.exit repeats the same selected batch on the exact frozen
  source, PASS exit0; full byte inventory still matches afterward.
- run-frozen.sh records every exact command. Before each execution batch, the
  source behavior audit and internal/testscope guard passed.
- Oracle history matrices: 512/4096/65536-byte pages × rollback/shared WAL;
  complete labels, values/storage classes, native primary error codes and
  official error categories. Live ordinary promotion/commit/EXCLUSIVE BUSY
  tests use merged output ordering, normal SQL and .quit/EOF completion only.
- Native tests include prepared repeated use/pinned separate connections,
  abandoned pool transaction reset, active Rows, cancellation after first row,
  lazy transaction counter reads, ordinary reopen and extension maintenance.
- tree-whitespace-check.txt: exact-tree diff whitespace check exit0.

Original selected-checks.txt/.exit is preserved: all selected tests passed, but
an untagged vet command selected oracle-only testkit and exited1. The corrected
v2 command vets ordinary packages normally and testkit with -tags oracle.
Original and corrected official probe scripts/output are retained; the first
BUSY script placed a counter SELECT on the same line as failed DML, so the shell
skipped that SELECT. Corrected v2 separates lines and records values. The v3 preparation probe
retains exact official valid/invalid DELETE SQL under a peer EXCLUSIVE lock: the
missing-table SQL preserves2/6/3; valid DELETE BUSY yields0/6/3. Both fail before
catalog admission natively and preserve prior counters, an explicit code gap.

No broad default or broad oracle suite ran. No manual tags, cache clearing,
process killing, physical corruption/mutation, namespace changes, native I/O/
lock/close/unmap/sync failures, diskfull/permissions, crash/hot recovery,
interrupted publication or rekey work was performed. No main repository edits,
staging or commits were made.

## Lead-owned documentation integration

Only the new docs/sql-connection-counters.md was added. Do not overwrite central
README, compatibility, progress, transactions or backlog from this worker tree.
The lead should update current compatibility/transaction scope prose that says
changes()/last_insert_rowid() is unsupported, link the new bounded profile and
record the exact early snapshot-admission BUSY distinction and unrun physical
gates. Preserve historical evidence rather than rewriting it. No pre-existing
negative test assertion was altered or removed by this slice.
