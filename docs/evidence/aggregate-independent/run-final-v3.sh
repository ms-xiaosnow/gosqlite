#!/bin/bash
set -uo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/aggregate-independent-copy-v3
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Review the selected aggregate tests and helpers behaviorally before executing. The review covers intact ordinary SQL/native/driver lifecycles and pure-memory arithmetic/parser/quota tests only.'
go run ./internal/testscope && go test -tags=oracle ./internal/sql ./sqlite ./driver ./internal/testkit -run '^(TestGlobalAggregate|TestDriverGlobalAggregate|TestIndependentAggregate|TestReviewerAggregate|TestRejectUnsupported|TestRejectionsAndCodes)' -count=1 -timeout=10m
echo FOCUSED_ORACLE_V3_EXIT=$?
go run ./internal/testscope && CGO_ENABLED=1 go test -race -tags=oracle ./internal/sql ./sqlite ./driver ./internal/testkit -run '^(TestGlobalAggregate|TestDriverGlobalAggregate|TestIndependentAggregate|TestReviewerAggregate|TestRejectUnsupported|TestRejectionsAndCodes)' -count=1 -timeout=10m
echo FOCUSED_ORACLE_RACE_V3_EXIT=$?
go vet ./...; echo VET_V3_EXIT=$?
go build -trimpath ./...; echo BUILD_V3_EXIT=$?
go mod verify; echo MOD_VERIFY_V3_EXIT=$?
go list -deps -f '{{if .CgoFiles}}{{.ImportPath}}:{{.CgoFiles}}{{end}}' ./sqlite ./driver; echo PRODUCTION_CGO_CLOSURE_V3_EXIT=$?
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} go build ./sqlite ./driver ./internal/sql
  echo CROSS_V3_${target}=$?
done
