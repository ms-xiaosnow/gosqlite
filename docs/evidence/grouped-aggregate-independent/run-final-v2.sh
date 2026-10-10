set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/grouped-review-copy-v2
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" GOFLAGS=-buildvcs=false CGO_ENABLED=0 SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Exact v2 candidate, independently inventoried and reconstructed from reviewed baseline patch. Behavioral audit: all selected tests are pure parser/AST/arithmetic/resource laws or ordinary intact SQL fixtures, typed values, normal cancellation/close, shared snapshots, transaction BUSY/rollback and checkpoint. Pure-memory candidate refusal checks inject no storage/native failure. Oracle CLI completes normally. No restricted physical/native-fault/namespace/crash/hot-recovery/interrupted publication/rekey workflows.'
date -u +%FT%TZ
test "$(git write-tree)" = 6774495e7289aa1cf7a652df673ba2610117900e
sha256sum -c ../grouped-aggregate-evidence/freeze-v2-all-sha256.txt >/dev/null
selection='^Test(GroupedAggregate|DriverGroupedAggregate|GlobalAggregateGrammar|RejectUnsupported|BooleanIdentifierRetainedRejections)'
go run ./internal/testscope
go test ./internal/sql ./sqlite ./driver -run "$selection" -count=1 -timeout=5m
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run '^TestGrouped(AggregateOracle|Reviewer)' -count=1 -timeout=5m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/sql ./sqlite ./driver -run "$selection" -count=1 -timeout=5m
go run ./internal/testscope
CGO_ENABLED=1 go test -race -tags oracle ./internal/testkit -run '^TestGrouped(AggregateOracle|Reviewer)' -count=1 -timeout=5m
go vet ./...
go build -trimpath ./...
go mod verify
closure=$(go list -deps -f '{{if .CgoFiles}}{{.ImportPath}} {{.CgoFiles}}{{end}}' ./sqlite ./driver)
test -z "$closure"
echo 'CGO-disabled sqlite/driver production dependency closure contains no CgoFiles'
sha256sum -c ../grouped-aggregate-evidence/freeze-v2-all-sha256.txt >/dev/null
git diff --exit-code
date -u +%FT%TZ
echo 'Selected final v2 independent normal/race/oracle/vet/build checks PASS; no full default/oracle gate claimed'
