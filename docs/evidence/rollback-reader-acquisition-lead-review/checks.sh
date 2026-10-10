set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/rollback-reader-acquisition-lead-review/frozen
export PATH="$PWD/../../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../../go-cache" GOPATH="$PWD/../../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0
echo 'Independent lead review of exact ad066d04: source/AST and memory classifiers plus ordinary cooperating reader/writer. Constructor and createEmpty helper audited. No injected native errors, physical mutation, hot recovery, abrupt process exit or manual tags.'
date -u +%FT%TZ
go run ./internal/testscope
go test ./internal/pager -run '^TestReaderAcquisition' -count=5 -timeout=10m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/pager -run '^TestReaderAcquisition' -count=3 -timeout=10m -v
go vet ./internal/pager; go build ./...
sha256sum -c ../../rollback-reader-acquisition-evidence/freeze-v1-sha256.txt >/dev/null
date -u +%FT%TZ
echo 'INDEPENDENT READER ACQUISITION FOCUSED PASS'
