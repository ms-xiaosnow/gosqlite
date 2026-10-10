set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/transaction-soak-fixed-freeze-v4
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false
E="$PWD/../transaction-soak-evidence-v4"
date -u +%FT%TZ
cat "$E/fixed-composition-tree.txt"
echo 'Scope AUDIT.md: pure-memory exceptional ownership controls, accepted ordinary reader fix, selected normal/race and one fixed64-attempt run/profile. No native failure/namespace/crash/recovery/long-profile execution.'
sha256sum -c "$E/fixed-composition-sha256.txt" > "$E/pre-hashes.txt"
go run ./internal/testscope
go test ./benchmarks/transactions -count=1 -timeout=0 -v
go vet ./benchmarks/transactions
go build -trimpath -o "$E/transactions-v4" ./benchmarks/transactions
CGO_ENABLED=1 go test -race ./benchmarks/transactions -count=1 -timeout=0 -v
go list -deps -f '{{if .CgoFiles}}{{.ImportPath}} {{.CgoFiles}}{{end}}' ./benchmarks/transactions > "$E/cgo0-closure.txt"
SOURCE=$(cat "$E/fixed-composition-tree.txt")
for profile in rollback wal-shared encrypted-shared; do
 "$E/transactions-v4" -mode throughput -transactions 64 -profile "$profile" -rows 16 -readers 1 -batch 1 -max-data-bytes 16777216 -source-id "$SOURCE" -hardware-id shared-cloud-linux-amd64-epyc-9v74 > "$E/throughput-v4-$profile.json"
 echo "$profile fixed64-attempt ordinary smoke PASS"
done
sha256sum -c "$E/fixed-composition-sha256.txt" > "$E/post-hashes.txt"
date -u +%FT%TZ
echo 'V4 focused fixed-composition gates PASS; no native exceptional retirement or long-soak acceptance claim'
