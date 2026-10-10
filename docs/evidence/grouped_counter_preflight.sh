set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/grouped-counters-final-verify
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Exact composition13f53e4. Selected checks use descriptor-free cache/quota/AST laws, intact SQL/Tx/cooperating locks and ordinary official EOF. Promoted original GROUP BY query retained as a typed native and official positive; strict cached ordinary BUSY outcome now matches original official control. Obsolete imported review expectations were identified before execution; no failed run is fabricated.'
cat ../grouped-counters-final-tree.txt; date -u +%FT%TZ
go run ./internal/testscope
go test ./sqlite ./driver ./internal/sql -run 'ConnectionCounters|Preparation|Grouped' -count=1 -timeout=20m
CGO_ENABLED=1 go test -race ./sqlite ./driver ./internal/sql -run 'ConnectionCounters|Preparation|Grouped' -count=1 -timeout=20m
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run 'ConnectionCounters|Preparation|Grouped' -count=1 -timeout=20m
CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run 'ConnectionCounters|Preparation|Grouped' -count=1 -timeout=20m
go vet ./...; go build -trimpath ./...
sha256sum -c ../grouped-counters-final-all-sha256.txt >/dev/null
date -u +%FT%TZ
echo 'COMPOSED GROUPING/COUNTERS/CACHED PREPARATION SELECTED GATE PASS'
