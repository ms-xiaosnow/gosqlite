#!/bin/bash
set -uo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/aggregate-global-work-v3
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
audit(){ echo 'Behavioral audit: review probes read fully; all added aggregate tests use intact SQL-created fixtures, ordinary reads/shared snapshots/rollback/BUSY/cancel/close/checkpoint and pure-memory arithmetic/parser/budget admission. NUL bytes are SQL values, never file-byte mutation. Oracle commands finish normally. No physical/native failures, namespace mutation, termination, hot recovery or interrupted rekey.'; go run ./internal/testscope; }
date -u +%FT%TZ
audit || exit
go test -tags=oracle ./internal/sql ./sqlite ./driver ./internal/testkit -run '^(TestGlobalAggregate|TestDriverGlobalAggregate|TestIndependentAggregate|TestRelational|TestDriverRelational|TestRejectUnsupported|TestRejectionsAndCodes|TestOracleErrorsAndResources)' -count=1 -timeout=30m;echo FOCUSED_ORACLE_V3_EXIT=$?
audit || exit
CGO_ENABLED=1 go test -race -tags=oracle ./internal/sql ./sqlite ./driver ./internal/testkit -run '^(TestGlobalAggregate|TestDriverGlobalAggregate|TestIndependentAggregate|TestRelational|TestDriverRelational|TestRejectUnsupported|TestRejectionsAndCodes|TestOracleErrorsAndResources)' -count=1 -timeout=30m;echo FOCUSED_ORACLE_RACE_V3_EXIT=$?
go vet ./...;echo VET_V3_EXIT=$?
go build -trimpath ./...;echo BUILD_V3_EXIT=$?
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64;do GOOS=${target%/*} GOARCH=${target#*/} go build ./sqlite ./driver ./internal/sql;echo CROSS_V3_$target=$?;done
date -u +%FT%TZ
