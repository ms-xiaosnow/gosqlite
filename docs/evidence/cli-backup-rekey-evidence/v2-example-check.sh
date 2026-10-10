set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/cli-backup-rekey-freeze-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false
go run ./internal/testscope
go build -trimpath -o ../cli-backup-rekey-evidence/gosqlite-v2 ./cmd/gosqlite
sha256sum ../cli-backup-rekey-evidence/gosqlite-v2
go run ./examples/encrypted-cli ../cli-backup-rekey-evidence/gosqlite-v2
sha256sum -c ../cli-backup-rekey-evidence/v2-final-all-sha256.txt >/dev/null
echo 'v2 exact binary example and frozen source readback PASS'
