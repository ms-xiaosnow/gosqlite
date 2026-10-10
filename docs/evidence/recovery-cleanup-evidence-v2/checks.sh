#!/bin/bash
set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/recovery-cleanup-freeze-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0
export GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path"
export GOFLAGS=-buildvcs=false
set -x
go version
go run ./internal/testscope
go build ./...
go test ./... -run '^$' -count=1 -timeout=60s
go run ./internal/testscope
go test ./internal/pager -run '^Test(PlainRecoveryCleanup(Memory|Source)|PlainCleanupMemory|PlainTxCleanupMemory)' -count=1 -timeout=60s
go run ./internal/testscope
go test ./internal/vfs -run '^Test(CleanupOwnersBoundedMemory|PathAbsenceMemory)' -count=1 -timeout=60s
go run ./internal/testscope
go test ./internal/journal -run '^TestReplay.*InMemory$' -count=1 -timeout=60s
go run ./internal/testscope
go test ./internal/pager -run '^TestTxCreatePageSizesAndReadSnapshot$' -count=1 -timeout=60s
go run ./internal/testscope
go test ./sqlite -run '^TestPlaintextCleanupOrdinary(ReaderRowsAndReopen|DatabaseSQLZeroIdle)$' -count=1 -timeout=60s
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/pager -run '^Test(PlainRecoveryCleanup(Memory|Source)|PlainCleanupMemory|PlainTxCleanupMemory|TxCreatePageSizesAndReadSnapshot$)' -count=1 -timeout=90s
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/vfs -run '^Test(CleanupOwnersBoundedMemory|PathAbsenceMemory)' -count=1 -timeout=90s
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./sqlite -run '^TestPlaintextCleanupOrdinary(ReaderRowsAndReopen|DatabaseSQLZeroIdle)$' -count=1 -timeout=90s
for target in linux/arm64 darwin/arm64 windows/amd64; do
  export GOOS="${target%/*}" GOARCH="${target#*/}"
  go build ./...
  go test ./... -run '^$' -exec=true -count=1 -timeout=60s
done
