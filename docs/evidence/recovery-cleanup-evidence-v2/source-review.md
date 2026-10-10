# Source review: recovery owner repair

Baseline tree: 7d3184f87165e11e12560884ec6e4f92c9bfd039.
Final tree: 201dca235ee217594d231f2f583d3ccf65aa8093.

Reviewed `internal/pager/recovery.go`, new `plain_recovery_cleanup.go`, existing
plain cleanup/transaction/session owner helpers, native file bridges, VFS cleanup
and initial-absence helpers. This is static review, not native recovery evidence.

1. Root admission precedes filepath.Abs, native path resolution and main open. An
   admission error returns without attempted metadata/native acquisition. A defer
   is installed immediately after reservation, so pre-main constructor failures
   still adopt nested obligations and enter the independent pending registry.
2. Non-nil main is adopted immediately before inspecting its error or doing stat.
   The journal temporary slot reserves before metadata; open result is adopted
   immediately before error/stat/link/identity observations. No raw direct close
   remains in recovery.go or recoveryOpenJournal.
3. All exits flow to beginClass, which adopts operation errors before its existing
   nested -> temporary -> main stages. closeTemporary first adopts the candidate
   read cause; consumed diagnostics remain errors. An incomplete explicit child
   close returns before any subsequent logical action. finishPlainRecovery checks
   childClosePending and requests no same-operation second attempt.
4. Existing RecoverRollback logical order is retained: SHARED, RESERVED query,
   candidate inspection, EXCLUSIVE without RESERVED, rechecks, bounded parsing,
   extent/header validation, journal+directory sync, page replay, truncation,
   database sync, identity rechecks, managed journal close, unlink, directory sync,
   managed main retirement. New absence decisions require initial no-handle probe
   provenance. Wrapped/joined candidate EOF errors remain diagnostics.
5. The retry surface is existing bounded plain ownership only. The new finalizer
   helper receives a group, cause and two booleans; it calls only errors.Join,
   childClosePending and beginClass. It stores no replay plan/pages/DB/Tx/Rows or
   callbacks. Existing managed owners may hold normal pathname metadata and opaque
   errors may retain arbitrary payload, as documented.
6. Existing 4096-node single-pass owner discovery, no-effects-until-all-token-
   discovery, immutable historical tokens, pending snapshot and 1024-root admission
   behavior are reused. Unbounded input gets an opaque terminal barrier. Registry
   admission/snapshot do not call native status/retirement or wait on scope locks.
7. Operation and cleanup errors after touched preserve RecoveryRequired before
   unlink or CommitUncertain after unlink; the class enters pending tokens and
   results are cleared on any error. Retirement-only retries cannot repeat parse,
   replay/truncate, Sync, unlink, lock acquisition or path acquisition.
8. Exactly four source paths differ. Public structures, shared/encrypted flows,
   journal parser, automatic Open integration, main entrypoints, goal and central
   docs are byte-preserved. No recovery function was called and no physical failure
   was simulated. Physical recovery/release qualification remains NOT_RUN.

Final v2 differs from v1 only by removing an inaccurate literal no-path-retention
comment; executable statements and tests are unchanged. Initial VCS stamping
failure is preserved in v1 evidence. Final commands use GOFLAGS=-buildvcs=false,
which disables unavailable checkout metadata stamping without changing test scope.
