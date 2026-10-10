set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/restore-library-maintenance-verification-tools
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
echo 'Exact Library-restored maintenance/dataset checkpoint. Previously audited source/memory and intact ordinary acquisition, correct-key CLI lifecycle, dataset numeric/Top-K and persisted grams measurement. No native failure/namespace/crash/hot recovery/interrupted rekey. Complete frozen aggregate including NFC membership and full oracle/race already passed; this restore independently exercises focused changed scopes.'
git rev-parse HEAD; date -u +%FT%TZ
go run ./internal/testscope
go test ./internal/pager ./internal/vfs ./cmd/gosqlite ./migration/postgres ./benchmarks/grams -run 'ReaderAcquisition|SharedAbsence|PathAbsence|CLIProvider|CLIMaintenance|Import.*Distance|MigrateImportDatasetDistances|CorpusDeterministic|EmpiricalDistribution|BoundedConfiguration|NativePersistedGramsBenchmarkSmoke' -count=1 -timeout=0
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/pager ./internal/vfs ./cmd/gosqlite ./migration/postgres ./benchmarks/grams -run 'ReaderAcquisition|SharedAbsence|PathAbsence|CLIProvider|CLIMaintenance|Import.*Distance|MigrateImportDatasetDistances|CorpusDeterministic|EmpiricalDistribution|BoundedConfiguration|NativePersistedGramsBenchmarkSmoke' -count=1 -timeout=0
go vet ./...; go build -trimpath ./...
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do GOOS=${target%/*} GOARCH=${target#*/} go build ./...; echo "restored cross-build $target PASS"; done
go mod verify; test -z "$(git status --porcelain)"; date -u +%FT%TZ
echo 'EXACT RESTORED MAINTENANCE DATASET TOOLS CHECKPOINT PASS'
