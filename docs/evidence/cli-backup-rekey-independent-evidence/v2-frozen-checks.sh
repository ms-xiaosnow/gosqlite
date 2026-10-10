set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/cli-backup-rekey-independent-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false
selected='^(TestCLIProviderMemory.*|TestCLIMaintenance.*)$'
go version
go run ./internal/testscope
go test ./cmd/gosqlite -run "$selected" -count=1 -timeout=5m -v
go vet ./cmd/gosqlite ./examples/encrypted-cli
go build -trimpath -o ../cli-backup-rekey-independent-evidence/gosqlite-v2 ./cmd/gosqlite
go run ./examples/encrypted-cli ../cli-backup-rekey-independent-evidence/gosqlite-v2
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./cmd/gosqlite -run "$selected" -count=1 -timeout=5m -v
go mod verify
echo 'FROZEN CLI INDEPENDENT SELECTED CHECKS PASS'
