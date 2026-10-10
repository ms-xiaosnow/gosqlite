set -euo pipefail
cd "${SOURCE_TREE:-/workspace/scratch/5e06a74e5f65/durable-restart/sql-cached-preparation-work-v1}"
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
audit() {
 echo 'Behavior audit: selected owner/new sources use intact SQL-created databases, ordinary SQL DML/DDL/transactions/savepoints, cooperative locks, logical errors, quota/context rejection, EOF/.quit/Wait oracle completion. Existing counters cover partial source-row insertion/ABORT, extension maintenance and driver pool reset. Pure error-tree tests invoke no native operation. No physical tamper/namespace/native failure/recovery/crash/kill/interrupted publication test is selected.'
 go run ./internal/testscope
}
date -u +%FT%TZ
go version
"$SQLITE_ORACLE" -version
audit
go test ./sqlite ./driver ./internal/sql -run '^TestCachedPreparation|^TestDriverCachedPreparation|^TestConnectionCounters|^TestDriverConnectionCounters|^TestConnectionFunction|^Test(NativeDefaultsAffinityAndRowIDs|NativeStatementRollbackAndTransactionalDDL|NativeSnapshotsBusyAndCleanup|NativeBudgetsCancellationAndReadOnly|TransactionDeferredSnapshotTiming|TransactionSavepointNamesDDLAndStatementFailure|TransactionImmediateExclusiveAndFailedOuterRelease|TransactionSavepointLimitsAndRowsRestrictions|TransactionLazySavepointQuotaFailureState|DriverSavepointsPoolResetAndOuterRelease|DriverFailedOuterReleaseRemainsActive|DriverDeferredBeginSeesFirstAccessState|SQLExpressionFeaturesDMLBindingDefaults|BooleanScalarVersusPredicate|BooleanThreeValuedTruthTables)$' -count=1 -timeout=5m -v
audit
go test -tags oracle ./internal/testkit -run '^TestCachedPreparationOracle|^TestConnectionCountersOracle' -count=1 -timeout=5m -v
audit
CGO_ENABLED=1 go test -race ./sqlite ./driver ./internal/sql -run '^TestCachedPreparation|^TestDriverCachedPreparation|^TestConnectionCounters|^TestDriverConnectionCounters|^TestConnectionFunction' -count=1 -timeout=5m -v
audit
CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^TestCachedPreparationOracle|^TestConnectionCountersOracle' -count=1 -timeout=5m -v
audit
go vet ./sqlite ./driver ./internal/sql
go vet -tags oracle ./internal/testkit
go build ./sqlite ./driver ./internal/sql
printf 'SELECTED NORMAL/RACE/ORACLE/VET/BUILD PASS\n'
date -u +%FT%TZ
