#!/bin/sh
set -eu
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOPATH="$PWD/../go-path" GOCACHE="$PWD/../go-cache" GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
GO=../toolchains/go1.27.2/bin/go
scope() {
  test -s docs/evidence/affinity-substitution/scope-audit.md
  test -s docs/evidence/boolean-semantics/scope-audit.md
  "$GO" run ./internal/testscope
}
export CGO_ENABLED=0
scope
"$GO" test ./internal/sql -count=1
scope
"$GO" test ./sqlite ./driver -run 'Test(Affinity|Boolean|DriverBoolean|Relational|DriverRelational|OrdinaryIndex|Expression|SQLExpression|VectorReview|Grams)' -count=1
scope
"$GO" test -tags oracle ./internal/sql -count=1
scope
"$GO" test -tags oracle ./internal/testkit -run 'Test(Affinity|BooleanOracle|RelationalOracle|OracleExpression)' -count=1 -v
export CGO_ENABLED=1
scope
"$GO" test -race ./internal/sql ./sqlite ./driver -run 'Test(Affinity|Boolean|DriverBoolean|Relational|DriverRelational|OrdinaryIndex|Expression|SQLExpression|VectorReview)' -count=1
export CGO_ENABLED=0
scope
"$GO" vet ./internal/sql ./sqlite ./driver
scope
"$GO" vet -tags oracle ./internal/testkit
