#!/bin/bash
set -euxo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/recovery-cleanup-independent-final-v2
export PATH=/workspace/scratch/5e06a74e5f65/durable-restart/toolchains/go1.27.2/bin:$PATH
export GOCACHE=/workspace/scratch/5e06a74e5f65/durable-restart/go-cache
export GOPATH=/workspace/scratch/5e06a74e5f65/durable-restart/go-path
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false CGO_ENABLED=0
go version
go run ./internal/testscope
go test -count=1 -run '^TestPlainRecoveryCleanup(Memory|Source)' ./internal/pager
go test -count=1 -run '^(TestTxCreatePageSizesAndReadSnapshot|TestTxCommitPreservesUnrelatedHeaderAndPages)$' ./internal/pager
CGO_ENABLED=1 go test -race -count=1 -run '^TestPlainRecoveryCleanup(Memory|Source)' ./internal/pager
CGO_ENABLED=1 go test -race -count=1 -run '^(TestTxCreatePageSizesAndReadSnapshot|TestTxCommitPreservesUnrelatedHeaderAndPages)$' ./internal/pager
go vet ./internal/pager
go build ./...
for target in 'linux arm64' 'darwin amd64' 'darwin arm64' 'windows amd64'; do
 set -- $target
 GOOS=$1 GOARCH=$2 go test -c -o /workspace/scratch/5e06a74e5f65/durable-restart/recovery-cleanup-independent-evidence-v2/pager-$1-$2.test ./internal/pager
done
