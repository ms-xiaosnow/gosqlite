#!/bin/sh
set -eu
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOPATH=$PWD/../go-path GOCACHE=$PWD/../go-cache GOFLAGS=-buildvcs=false SQLITE_ORACLE=$PWD/../toolchains/sqlite-build-3540000/sqlite3
GO=../toolchains/go1.27.2/bin/go
export CGO_ENABLED=0
$GO run ./internal/testscope
$GO test ./internal/sql -count=1
$GO run ./internal/testscope
$GO test ./sqlite ./driver -run 'Test(Boolean|DriverBoolean|Relational|DriverRelational|OrdinaryIndex|Expression|SQLExpression|VectorReview|Grams)' -count=1
$GO run ./internal/testscope
$GO test -tags oracle ./internal/sql -count=1
$GO run ./internal/testscope
$GO test -tags oracle ./internal/testkit -run 'Test(BooleanOracle|RelationalOracle|OracleExpression)' -count=1 -v
export CGO_ENABLED=1
$GO run ./internal/testscope
$GO test -race ./internal/sql ./sqlite ./driver -run 'Test(Boolean|DriverBoolean|Relational|DriverRelational|OrdinaryIndex|Expression|SQLExpression|VectorReview)' -count=1
export CGO_ENABLED=0
$GO vet ./internal/sql ./sqlite ./driver
