#!/bin/sh
# Run from the immutable source root; only source-audited ordinary test subsets.
set -u
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOPATH="$PWD/../go-path" GOCACHE="$PWD/../go-cache" GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
GO=../toolchains/go1.27.2/bin/go
E=docs/evidence/boolean-identifiers
scope() { test -s "$E/scope-audit.md" && "$GO" run ./internal/testscope; }
run() {
 label=alias-on-final-$1; shift
 { printf 'CGO_ENABLED=%s ' "$CGO_ENABLED"; printf '%s ' "$@"; printf '\n'; scope && "$@"; } > "$E/$label.txt" 2>&1
 status=$?; echo "$status" > "$E/$label.exit"; printf '%s: exit %s\n' "$label" "$status"
}
export CGO_ENABLED=0
run internal-sql "$GO" test ./internal/sql -count=1
run native-focused "$GO" test ./sqlite ./driver -run '^Test(Boolean|DriverBoolean|Relational|DriverRelational|OrdinaryIndex|Expression|SQLExpression|Affinity)' -count=1
run scalar-oracle "$GO" test -tags oracle ./internal/sql -count=1
run ordinary-oracle "$GO" test -tags oracle ./internal/testkit -run '^Test(BooleanOracleContexts|BooleanAliasOnOracleProvenance|Affinity|RelationalOracle|OracleExpression|BooleanIdentifierOracleDML)' -count=1 -v
# Unchanged strict parser matrix: all 2021 cases must now pass.
run identifier-oracle-strict "$GO" test -tags oracle ./internal/testkit -run '^TestBooleanIdentifierOracleMatrix$' -count=1 -v
export CGO_ENABLED=1
run native-race "$GO" test -race ./internal/sql ./sqlite ./driver -run '^Test(Boolean|DriverBoolean|Relational|DriverRelational|OrdinaryIndex|Expression|SQLExpression|Affinity)' -count=1
export CGO_ENABLED=0
run vet "$GO" vet ./internal/sql ./sqlite ./driver
run oracle-vet "$GO" vet -tags oracle ./internal/testkit
run build "$GO" build ./...
