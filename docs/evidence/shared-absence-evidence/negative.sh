#!/usr/bin/env bash
set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/shared-absence-negative-v1
export PATH="$(realpath ../toolchains/go1.27.2/bin):$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$(realpath ../go-cache)" GOPATH="$(realpath ../go-path)" CGO_ENABLED=0 GOMAXPROCS=2
set -x
go run ./internal/testscope
set +e
go test -p=1 ./internal/pager -run '^TestSharedAbsenceSource' -count=1 -v
result=$?
set -e
test "$result" = 1
go test -p=1 ./internal/pager ./internal/vfs -run '^Test(SharedAbsenceMemory|PathAbsenceMemory|CleanupOwnersMemory|PlainCleanupMemory.*Absence|Shared(Open|Close)CleanupMemory)' -count=1 -v
