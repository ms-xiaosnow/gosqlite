set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/grouped-review-copy-v1
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0 SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Behavior audit: extra tests use ordinary SQL-created intact private fixtures and read-only pinned CLI normal completion, scalar error expectations and bounded result comparison only. No excluded workflow.'
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run '^TestGroupedReviewer(TopKDetails|RowidStream)$' -count=1 -timeout=5m -v
