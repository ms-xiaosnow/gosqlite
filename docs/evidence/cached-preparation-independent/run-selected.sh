#!/bin/bash
set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/sql-preparation-independent-final-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOPATH="$PWD/../go-path" GOCACHE="$PWD/../go-cache" GOFLAGS=-buildvcs=false CGO_ENABLED=0 SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
audit() {
 echo 'Selected-source behavioral review: intact ordinary SQL databases, cooperative locks, SQL-only peer DDL, supported expressions/counters, logical errors, explicit/automatic transaction and savepoint rollback, binding/context/quota, bounded copied metadata, read-only/active Rows and driver pool reset. Pure error-tree tests perform no native failure operation. Oracle subprocesses synchronize startup and finish with EOF/.quit and Wait. No restricted physical/namespace/native-failure/crash/recovery work; no broad suite or manual tags.'
 go run ./internal/testscope
}
date -u +%FT%TZ
go version
"$SQLITE_ORACLE" -version
sha256sum "$PWD/../toolchains/go1.27.2/bin/go" "$SQLITE_ORACLE"
audit
go test ./internal/sql ./sqlite ./driver -run '^(TestConnectionFunctionContextAndVolatility|TestConnectionCounters.*|TestDriverConnectionCounters.*|TestCachedPreparation.*|TestDriverCachedPreparation.*|TestReviewPreparation.*)$' -count=1 -v
audit
CGO_ENABLED=1 go test -race ./internal/sql ./sqlite ./driver -run '^(TestConnectionFunctionContextAndVolatility|TestConnectionCounters.*|TestDriverConnectionCounters.*|TestCachedPreparation.*|TestDriverCachedPreparation.*|TestReviewPreparation.*)$' -count=1 -v
audit
go test -tags=oracle ./internal/testkit -run '^(TestConnectionCountersOracle.*|TestCachedPreparationOracle.*|TestReviewPreparation.*)$' -count=1 -v
audit
CGO_ENABLED=1 go test -race -tags=oracle ./internal/testkit -run '^(TestConnectionCountersOracle.*|TestCachedPreparationOracle.*|TestReviewPreparation.*)$' -count=1 -v
audit
go vet ./internal/sql ./sqlite ./driver
go vet -tags=oracle ./internal/testkit
audit
go build ./internal/sql ./sqlite ./driver
echo 'SELECTED INDEPENDENT NORMAL/RACE/ORACLE/VET/BUILD PASS'
date -u +%FT%TZ
