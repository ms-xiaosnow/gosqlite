set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/grouped-review-sort-foundation
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Audited selected test and called helpers: owned ordinary SQLite-created fixtures, read-only queries, SQL errors, quota/cancel/Close and byte-hash unchanged checks. CLI runs ordinary EOF. No excluded workflow.'
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run '^TestOracleErrorsAndResources$' -count=1 -timeout=3m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^TestOracleErrorsAndResources$' -count=1 -timeout=3m
sha256sum -c ../sql-final-all-sha256.txt >/dev/null
[ "$(git write-tree)" = 48370bbe2dc585c1b45821fbc360c525d156a9f5 ]
git diff --exit-code
printf 'Sort foundation checks PASS\n'
