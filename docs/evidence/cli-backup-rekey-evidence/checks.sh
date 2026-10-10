set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/cli-backup-rekey-work
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false
selected='^(TestCLIProviderMemory.*|TestCLIMaintenance.*|TestVersion|TestUnsupported|TestSQLWriteAndQuery|TestSQLRejectedBeforeCreation|TestCLIExclusiveWAL|TestCLISharedWAL|TestCLIGramsQueryAndCheck|TestQueryReadonlyFixture|TestInspectReadonlyFixture|TestQueryCanceled)$'
go version
go run ./internal/testscope
go test ./cmd/gosqlite -run "$selected" -count=1 -timeout=5m -v
go vet ./cmd/gosqlite ./examples/encrypted-cli
go build -trimpath ./...
go build -trimpath -o ../cli-backup-rekey-evidence/gosqlite ./cmd/gosqlite
go run ./examples/encrypted-cli ../cli-backup-rekey-evidence/gosqlite
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./cmd/gosqlite -run "$selected" -count=1 -timeout=5m
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} go build ./...
  echo "CGO0 cross-build $target PASS (compile only)"
done
go mod verify
echo 'CLI ENCRYPTED BACKUP/REKEY SELECTED CHECKS PASS'
