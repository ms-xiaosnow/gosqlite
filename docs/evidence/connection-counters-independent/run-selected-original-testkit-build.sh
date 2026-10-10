#!/bin/bash
set -eu
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOPATH="$PWD/../go-path" GOCACHE="$PWD/../go-cache" GOFLAGS=-buildvcs=false
export SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
date -u
# Audited, selected native/expression/driver tests only. No broad default suite.
go run ./internal/testscope
CGO_ENABLED=0 go test ./internal/sql ./sqlite ./driver -run '^(TestConnectionFunctionContextAndVolatility|TestConnectionCounters.*|TestDriverConnectionCounters.*|TestReviewConnectionCounters.*)$' -count=1 -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/sql ./sqlite ./driver -run '^(TestConnectionFunctionContextAndVolatility|TestConnectionCounters.*|TestDriverConnectionCounters.*|TestReviewConnectionCounters.*)$' -count=1 -v
go run ./internal/testscope
CGO_ENABLED=0 go test -tags=oracle ./internal/testkit -run '^(TestConnectionCountersOracle.*|TestReviewConnectionCountersTypedOracle)$' -count=1 -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race -tags=oracle ./internal/testkit -run '^(TestConnectionCountersOracle.*|TestReviewConnectionCountersTypedOracle)$' -count=1 -v
go run ./internal/testscope
CGO_ENABLED=0 go vet ./internal/sql ./sqlite ./driver
CGO_ENABLED=0 go vet -tags=oracle ./internal/testkit
go run ./internal/testscope
CGO_ENABLED=0 go build ./internal/sql ./sqlite ./driver
CGO_ENABLED=0 go build -tags=oracle ./internal/testkit
echo 'SELECTED INDEPENDENT CHECKS PASS'
date -u
