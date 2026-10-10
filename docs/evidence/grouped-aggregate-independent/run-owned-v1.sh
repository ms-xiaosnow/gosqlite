set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/grouped-review-copy-v1
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0 SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Behavior audit: new grouped tests are pure AST/state/quota laws or ordinary intact SQL fixtures, typed-value errors, normal close/cancel, prepared statements, shared snapshots and transaction BUSY/rollback/checkpoint. Reviewed helper transitive behavior. CLI completes ordinarily. No restricted workflow.'
go run ./internal/testscope
go test ./internal/sql ./sqlite ./driver -run '^Test(GroupedAggregate|DriverGroupedAggregate)' -count=1 -timeout=5m
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run '^TestGroupedAggregateOracle' -count=1 -timeout=5m
sha256sum -c ../grouped-aggregate-evidence/freeze-v1-all-sha256.txt >/dev/null
git diff --exit-code
