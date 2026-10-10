# Manual verification and release-blocking gates

This file records checks that require a separately approved native environment or independent reviewer. No command here is evidence that it was executed.

## Current state

M0 ordinary unit/build/vet/race checks are executable here. Database crash/fault/process-exit workflow execution is prohibited by the current platform boundary; it must not be renamed, obscured, moved to remote CI, or executed elsewhere as a workaround. Such test source may be reviewed and documented; its results remain BLOCKED / NOT_RUN until independently supplied authorized evidence is available.

Later milestones will add exact ordinary manual commands for implemented crash/recovery, cross-process lock, native OS, security and legal gates. Do not execute a placeholder command or claim a nonexistent test passed.

## Release requirements

- Linux/amd64, Linux/arm64, Darwin/arm64 and Windows/amd64 native file-lock/durability verification, distinct from cross-compilation.
- Long-running concurrent soak with fixed workload/hardware/seed and real recorded duration.
- Independent data-safety, cryptography/threat-model, dependency/license and API reviews.
- Data-format upgrade/rollback and preservation evidence.

All are NOT_RUN at M0 and block stable/production acceptance.
