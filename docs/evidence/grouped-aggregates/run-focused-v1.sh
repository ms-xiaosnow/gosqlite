set -euo pipefail
cd "${GROUPED_SOURCE:-/workspace/scratch/5e06a74e5f65/durable-restart/grouped-aggregate-work-v1}"
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0 SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Grouped scope: reviewed intact ordinary SQL/typed-value/parser errors, normal shared snapshots/Tx/BUSY/rollback/cancel/close/checkpoint, pure-memory quotas/math only. No restricted physical/namespace/native-fault/crash/interrupted publication/rekey workflows. No manual tags, no test-process kills, no cache clearing.'
date -u +%FT%TZ
go version
"$SQLITE_ORACLE" -batch -bail -list :memory: 'SELECT sqlite_version(),sqlite_source_id();'
selection='^Test(GroupedAggregate|GlobalAggregate|DriverGroupedAggregate|DriverGlobalAggregate|RejectUnsupported|Relational|DriverRelational|BooleanIdentifier)'
go run ./internal/testscope
go test ./internal/sql ./sqlite ./driver -run "$selection" -count=1 -timeout=10m
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run '^TestGroupedAggregateOracle' -count=1 -timeout=10m
go vet ./...
go build -trimpath ./...
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/sql ./sqlite ./driver -run "$selection" -count=1 -timeout=10m
go run ./internal/testscope
CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^TestGroupedAggregateOracle' -count=1 -timeout=10m
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do GOOS=${target%/*} GOARCH=${target#*/} go build ./sqlite ./driver ./internal/sql; echo "core cross-build $target PASS"; done
go mod verify
closure=$(go list -deps -f '{{if .CgoFiles}}{{.ImportPath}} {{.CgoFiles}}{{end}}' ./sqlite ./driver)
if test -n "$closure"; then echo "$closure"; exit 1; fi
echo 'Production sqlite/driver CgoFiles closure empty'
date -u +%FT%TZ
echo 'FOCUSED GROUPED CHECKS PASS; no complete repository test pass claimed'
