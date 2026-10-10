#!/bin/sh
set -eu
: "${SQLITE_ORACLE:?Set SQLITE_ORACLE to the verified official SQLite 3.54.0 CLI}"
export CGO_ENABLED=0 GOTOOLCHAIN=local
go run ./internal/testscope
go test -tags oracle ./...
