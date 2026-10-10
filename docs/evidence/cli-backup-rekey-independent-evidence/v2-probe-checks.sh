set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/cli-backup-rekey-independent-probes-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false
export GOSQLITE_REVIEW_BINARY="$PWD/../cli-backup-rekey-independent-evidence/gosqlite-v2"
go run ./internal/testscope
go test ./cmd/gosqlite -run '^TestReview' -count=1 -timeout=5m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./cmd/gosqlite -run '^TestReview' -count=1 -timeout=5m -v
echo 'REVIEWER V2 MEMORY AND ACTUAL CLI PROBES PASS'
