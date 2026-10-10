#!/bin/sh
# Run in a fresh copy of boolean-identifier-freeze-v2 after copying the three
# saved .go.txt sources to their package paths; never write into the freeze.
set -u
R=/workspace/scratch/5e06a74e5f65/durable-restart
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$R/go-cache" GOPATH="$R/go-path" SQLITE_ORACLE="$R/toolchains/sqlite-build-3540000/sqlite3" CGO_ENABLED=0
GO="$R/toolchains/go1.27.2/bin/go"
E="$R/gosqlite/docs/evidence/boolean-identifier-independent"
run() { label=$1; shift; { printf '%s\n' "$*"; "$GO" run ./internal/testscope && "$@"; } > "$E/$label.txt" 2>&1; result=$?; echo "$result" > "$E/$label.exit"; echo "$label: $result"; }
run independent-matrix-supported "$GO" test -tags oracle ./internal/testkit -run '^TestIndependentAliasOn' -count=1 -v
run independent-ast "$GO" test ./sqlite -run '^TestIndependentAliasSameASTAndBudget$' -count=1 -v
export CGO_ENABLED=1
run independent-ast-race "$GO" test -race ./sqlite -run '^TestIndependentAliasSameASTAndBudget$' -count=1 -v
export CGO_ENABLED=0
run independent-oracle-vet "$GO" vet -tags oracle ./internal/testkit
