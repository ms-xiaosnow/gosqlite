#!/bin/sh
set -u
R=/workspace/scratch/5e06a74e5f65/durable-restart
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$R/go-cache" GOPATH="$R/go-path" SQLITE_ORACLE="$R/toolchains/sqlite-build-3540000/sqlite3" CGO_ENABLED=0
GO="$R/toolchains/go1.27.2/bin/go"
E="$R/gosqlite/docs/evidence/boolean-identifier-independent"
for version in baseline-v2 v2; do
 cd "$R/boolean-provenance-independent-$version"
 { "$GO" run ./internal/testscope && "$GO" test -tags oracle ./internal/testkit -run '^TestIndependentInheritedAliasBoundary$' -count=1 -v; } > "$E/inherited-$version.txt" 2>&1
 echo $? > "$E/inherited-$version.exit"
done
