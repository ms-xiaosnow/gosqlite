set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/grouped-review-copy-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0
printf 'Reviewed ordinary intact SQL fixture, normal row/BLOB lifetime and pure work/memory quota enforcement. No excluded workflows.\n'
go run ./internal/testscope
go test ./sqlite -run '^TestGroupedReviewerRetentionAdmission$' -count=1 -timeout=3m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./sqlite -run '^TestGroupedReviewerRetentionAdmission$' -count=1 -timeout=3m
sha256sum -c ../grouped-aggregate-evidence/freeze-v2-all-sha256.txt >/dev/null
git diff --exit-code
