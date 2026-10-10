set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/recovery-combined-verify
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Exact checked composition preserves passing SQL/plain code, reviewed secondary ranges and byte-only replay parsing. Source-only recovery ownership adds only memory/AST laws and normal unrelated lifecycle checks; RecoverRollback is not invoked. Grouped-counters-final additionally includes reviewed GROUP BY, counters and cached ordinary preparation, with strict official BUSY and original GROUP BY positive controls. No planner-error candidate, physical/native-fault/namespace/crash/hot-recovery or interrupted-publication/rekey test is included.'
cat ../recovery-combined-tree.txt
date -u +%FT%TZ
go run ./internal/testscope
go test ./... -count=1 -timeout=30m
go vet ./...
go build -trimpath ./...
echo 'RECOVERY-COMBINED FULL CGO0 UNIT/VET/BUILD PASS'; date -u +%FT%TZ
./scripts/test-oracle.sh
echo 'RECOVERY-COMBINED FULL TAGGED ORACLE PASS'; date -u +%FT%TZ
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./... -count=1 -timeout=30m
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do GOOS=${target%/*} GOARCH=${target#*/} go build ./...; echo "cross-build $target PASS"; done
go mod verify
sha256sum -c ../recovery-combined-all-sha256.txt >/dev/null
date -u +%FT%TZ
echo 'RECOVERY-COMBINED FULL FROZEN AGGREGATE PASS'
