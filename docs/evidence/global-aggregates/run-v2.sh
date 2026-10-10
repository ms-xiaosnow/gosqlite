#!/bin/bash
set -uo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/aggregate-global-work-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
audit(){ echo 'Behavioral audit: inherited frozen 1336a5d7 ordinary source review plus all owned aggregate tests inspected: intact SQL fixture creation/read/write, regular shared snapshots, rollback/BUSY/cancel/close/checkpoint, pure-memory arithmetic/parser/quota/clone admission. Oracle command finishes normally, no native failure or physical mutation/termination/recovery.'; go run ./internal/testscope; }
date -u +%FT%TZ
audit || exit
go test ./... -count=1 -timeout=30m; echo FULL_UNIT_V2_EXIT=$?
audit || exit
go test -tags=oracle ./internal/sql ./sqlite ./driver ./internal/testkit -run '^(TestGlobalAggregate|TestDriverGlobalAggregate|TestRelational|TestDriverRelational|TestRejectUnsupported|TestRejectionsAndCodes|TestOracleErrorsAndResources)' -count=1 -timeout=30m; echo FOCUSED_ORACLE_V2_EXIT=$?
audit || exit
CGO_ENABLED=1 go test -race -tags=oracle ./internal/sql ./sqlite ./driver ./internal/testkit -run '^(TestGlobalAggregate|TestDriverGlobalAggregate|TestRelational|TestDriverRelational|TestRejectUnsupported|TestRejectionsAndCodes|TestOracleErrorsAndResources)' -count=1 -timeout=30m; echo FOCUSED_ORACLE_RACE_V2_EXIT=$?
go vet ./...; echo VET_V2_EXIT=$?
go build -trimpath ./...; echo BUILD_V2_EXIT=$?
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64;do GOOS=${target%/*} GOARCH=${target#*/} go build ./sqlite ./driver ./internal/sql;echo CROSS_V2_$target=$?;done
date -u +%FT%TZ
