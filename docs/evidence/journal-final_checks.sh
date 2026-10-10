set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/journal-final-verify
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Exact combined source adds independently reviewed leading secondary inequalities and preserves final SQL plus plaintext ownership. Journal-final additionally broadens byte-only finite-count segment parsing; automatic Open recovery is absent. All new tests are audited intact ordinary SQL/index/ownership fixtures or pure-memory parser/model laws. No RecoverRollback or restricted physical/native-fault/namespace/crash/recovery workflow is executed.'
cat ../journal-final-tree.txt
date -u +%FT%TZ
go run ./internal/testscope
go test ./... -count=1 -timeout=30m
go vet ./...
go build -trimpath ./...
echo 'JOURNAL-FINAL COMBINED FULL CGO0 UNIT/VET/BUILD PASS'; date -u +%FT%TZ
./scripts/test-oracle.sh
echo 'JOURNAL-FINAL COMBINED FULL TAGGED ORACLE PASS'; date -u +%FT%TZ
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./... -count=1 -timeout=30m
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do GOOS=${target%/*} GOARCH=${target#*/} go build ./...; echo "cross-build $target PASS"; done
go mod verify
sha256sum -c ../journal-final-all-sha256.txt >/dev/null
date -u +%FT%TZ
echo 'JOURNAL-FINAL COMBINED FULL FROZEN AGGREGATE PASS'
