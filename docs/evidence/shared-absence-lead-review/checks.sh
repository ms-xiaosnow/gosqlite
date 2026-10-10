set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/shared-absence-lead-review/frozen
export PATH="$PWD/../../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../../go-cache" GOPATH="$PWD/../../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0 GOMAXPROCS=2
echo 'Independent exact b983d28 shared-absence source/descriptor-free memory laws and ordinary pinned shared snapshots/commit/rollback/checkpoint/normal reopen at three page sizes. Test bodies and helper paths audited. No physical missing-file/namespace/native-fault/crash/recovery workflow.'
date -u +%FT%TZ
go run ./internal/testscope
go test -p=1 ./internal/pager ./internal/vfs -run '^TestSharedAbsence' -count=3 -v
go test -p=1 ./internal/pager -run '^TestSharedWALSnapshotsPromotionAndCheckpoint$' -count=2 -v
go run ./internal/testscope
CGO_ENABLED=1 go test -p=1 -race ./internal/pager ./internal/vfs -run '^Test(SharedAbsence|SharedWALSnapshotsPromotionAndCheckpoint$)' -count=2
go vet -p=1 ./internal/pager ./internal/vfs; go build -p=1 ./...
sha256sum -c ../../shared-absence-evidence/full-sha256.txt >/dev/null
date -u +%FT%TZ
echo 'INDEPENDENT SHARED ABSENCE FOCUSED PASS'
