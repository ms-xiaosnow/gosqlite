#!/bin/sh
set -eu
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOPATH=$PWD/../go-path GOCACHE=$PWD/../go-cache GOFLAGS=-buildvcs=false SQLITE_ORACLE=$PWD/../toolchains/sqlite-build-3540000/sqlite3
export CGO_ENABLED=0
../toolchains/go1.27.2/bin/go run ./internal/testscope
../toolchains/go1.27.2/bin/go test ./internal/sql -count=1
../toolchains/go1.27.2/bin/go run ./internal/testscope
../toolchains/go1.27.2/bin/go test ./sqlite -run 'Test(Relational|OrdinaryIndex|Expression)' -count=1
