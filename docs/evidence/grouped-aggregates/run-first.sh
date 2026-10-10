set -u
cd /workspace/scratch/5e06a74e5f65/durable-restart/grouped-aggregate-work-v1
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0 SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Reviewed selected tests: pure-memory parser/global arithmetic; grouped tests only intact owned ordinary SQL, normal close/cancel, quota and read-only oracle processes with normal EOF. No restricted physical/native-fault/namespace/crash workflows.'
go run ./internal/testscope || exit $?
go test ./internal/sql -run '^Test(GlobalAggregateGrammar|Unsupported|BooleanIdentifier|From)' -count=1; echo "legacy_grammar_exit=$?"
go test ./sqlite -run '^TestGroupedAggregate' -count=1; echo "native_exit=$?"
go test -tags=oracle ./internal/testkit -run '^TestGroupedAggregateOracleValues$' -count=1; echo "oracle_exit=$?"
