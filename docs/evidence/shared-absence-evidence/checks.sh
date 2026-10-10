#!/usr/bin/env bash
set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/shared-absence-freeze-v1
export PATH="$(realpath ../toolchains/go1.27.2/bin):$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$(realpath ../go-cache)" GOPATH="$(realpath ../go-path)" CGO_ENABLED=0 GOMAXPROCS=2
E=$(realpath ../shared-absence-evidence)
set -x
go version
go run ./internal/testscope
go test -p=1 ./internal/pager ./internal/vfs -run '^Test(SharedAbsence|PathAbsenceMemory|CleanupOwnersMemory|PlainCleanupMemory.*Absence|Shared(Open|Close)CleanupMemory)' -count=1 -v
go test -p=1 ./internal/pager -run '^Test(SharedWAL|SharedOpenCleanupSuccessfulSessionsDoNotConsumeCapacity)' -count=1 -v
go test -p=1 ./sqlite ./driver -run '^Test(SharedWAL|CloseCleanupOrdinarySharedRowsTxAndReopen|SharedCloseCleanupConnectorCancellationAndOrdinaryDisposal)' -count=1 -v
go vet -p=1 ./internal/pager ./internal/vfs ./sqlite ./driver
CGO_ENABLED=1 go test -race -p=1 ./internal/pager ./internal/vfs ./sqlite ./driver -run '^Test(SharedAbsence|PathAbsenceMemory|CleanupOwnersMemory|PlainCleanupMemory.*Absence|Shared(Open|Close)CleanupMemory|SharedWAL|CloseCleanupOrdinarySharedRowsTxAndReopen|SharedCloseCleanupConnectorCancellationAndOrdinaryDisposal)' -count=1
for target in linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  export GOOS=${target%/*} GOARCH=${target#*/}
  go build -p=1 ./internal/pager ./internal/vfs ./sqlite ./driver
  go test -p=1 -c -o "$E/${GOOS}-${GOARCH}-pager.test" ./internal/pager
done
