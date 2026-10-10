set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/sql-connection-counters-freeze-v1
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Behavior audit: selected existing tests reviewed: normal SQL transactional defaults, statement failure, active Rows/BUSY, quota/cancel/read-only paths; transaction control/driver tests intact normal files only; SQL expression tests descriptor-free. New tests use normal SQL, logical errors, cancellation after a row boundary, ordinary CLI EOF, and no forbidden physical/namespace/recovery/native-failure workflows.'
date -u +%FT%TZ
go run ./internal/testscope
go test ./sqlite ./driver ./internal/sql -run 'Test(ConnectionCounters|ConnectionFunction|DriverConnectionCounters|NativeDefaultsAffinityAndRowIDs|NativeStatementRollbackAndTransactionalDDL|NativeSnapshotsBusyAndCleanup|NativeBudgetsCancellationAndReadOnly|TransactionDeferredSnapshotTiming|TransactionSavepointNamesDDLAndStatementFailure|TransactionImmediateExclusiveAndFailedOuterRelease|TransactionSavepointLimitsAndRowsRestrictions|TransactionLazySavepointQuotaFailureState|DriverSavepointsPoolResetAndOuterRelease|DriverFailedOuterReleaseRemainsActive|DriverDeferredBeginSeesFirstAccessState|SQLExpressionFeaturesDMLBindingDefaults|BooleanScalarVersusPredicate|BooleanThreeValuedTruthTables)$|^TestConnectionCounters|^TestDriverConnectionCounters|^TestConnectionFunction' -count=1 -timeout=5m -v
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run '^TestConnectionCountersOracle' -count=1 -timeout=5m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./sqlite ./driver ./internal/sql -run '^TestConnectionCounters|^TestDriverConnectionCounters|^TestConnectionFunction' -count=1 -timeout=5m -v
CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^TestConnectionCountersOracle' -count=1 -timeout=5m -v
go vet ./sqlite ./driver ./internal/sql
go vet -tags oracle ./internal/testkit
go build ./sqlite ./driver ./internal/sql
printf 'SELECTED NORMAL/RACE/ORACLE/VET/BUILD PASS\n'
date -u +%FT%TZ
