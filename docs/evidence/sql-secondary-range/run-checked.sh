#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../../.."
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
export GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path"
export SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
export CGO_ENABLED=0
# Read behavioral-audit.md before execution. These selections exercise ordinary
# SQL and descriptor-free budget/parser paths only; no physical fault tags.
guard() { go run ./internal/testscope; }
date -u +%FT%TZ
go version
"$SQLITE_ORACLE" -batch -bail :memory: 'SELECT sqlite_version(),sqlite_source_id();'
guard
go test ./sqlite ./driver ./internal/sql -run '^(TestSecondaryRange|TestDriverSecondaryRange|TestOrdinaryIndex|TestRowIDRange|TestDriverRowIDRange|TestBoolean|TestDriverBoolean)' -count=1 -v -timeout=30m
guard
go test -tags oracle ./internal/testkit -run '^(TestOracleSecondaryRange|TestOracleOrdinaryIndex|TestOracleRowIDRangeSQL|TestBooleanIdentifier|TestBooleanAlias)' -count=1 -v -timeout=30m
guard
CGO_ENABLED=1 go test -race ./sqlite ./driver -run '^(TestSecondaryRange|TestDriverSecondaryRange)' -count=1 -v -timeout=30m
guard
CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^TestOracleSecondaryRange' -count=1 -v -timeout=30m
guard
go build ./...
guard
go vet ./...
date -u +%FT%TZ
echo 'SECONDARY RANGE SCOPED CHECKS PASS; independent review and lead full integration remain separate'
