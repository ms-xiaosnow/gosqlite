#!/usr/bin/env bash
set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/boolean-rank-filter-freeze-v1
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
cat ../boolean-rank-filter-evidence/scope-audit.md
date -u +%FT%TZ
sha256sum -c ../boolean-rank-filter-evidence/frozen-all-sha256.txt >/dev/null
go run ./internal/testscope
go test ./sqlite ./internal/sql -count=1 -timeout=20m
go run ./internal/testscope
go test ./driver ./extension/vector/... -run '(Boolean|Vector|ANN|Half|Sparse|Bit|IVF|Similarity|Expression)' -count=1 -timeout=15m
go vet ./sqlite ./internal/sql ./driver
go build ./...
go run ./internal/testscope
go test -tags oracle ./internal/testkit ./internal/sql ./driver -run '(Boolean|Vector|ANN|Half|Sparse|Bit|IVF|Similarity|Affinity)' -count=1 -timeout=20m -v
sha256sum -c ../boolean-rank-filter-evidence/frozen-all-sha256.txt >/dev/null
date -u +%FT%TZ
echo 'FROZEN FULL SQLITE/INTERNAL-SQL + SELECTED DRIVER/VECTOR + SELECTED ORACLE + VET/BUILD PASS'
