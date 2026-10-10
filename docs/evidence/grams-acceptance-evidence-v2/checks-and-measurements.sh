#!/usr/bin/env bash
set -euo pipefail
b=/workspace/scratch/5e06a74e5f65/durable-restart
e="$b/grams-acceptance-evidence-v2"
# Avoid overlapping this worker's timings with its earlier selected test run.
while [ ! -f "$b/grams-acceptance-evidence/checks.exit" ]; do sleep 10; done
cd "$b/grams-acceptance-freeze-v2"
export PATH="$b/toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$b/go-cache" GOPATH="$b/go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
printf 'Final source '; cat "$e/final-tree.txt"
printf 'v1/v2 exact inventory delta only benchmarks/grams/main.go and README.md; sqlite package and entire imported source closure are byte-identical. Selected sqlite CGO0/race evidence therefore retains exact executable source identity. Final changed harness rechecked below.\n'
cat "$b/grams-acceptance-evidence/checks.exit"
date -u +%FT%TZ
go run ./internal/testscope
go test ./benchmarks/grams -count=1 -v -timeout=10m
go vet ./benchmarks/grams
go build -trimpath -o "$e/grams-cli-a" ./benchmarks/grams
go build -trimpath -o "$e/grams-cli-b" ./benchmarks/grams
cmp "$e/grams-cli-a" "$e/grams-cli-b"
sha256sum "$e/grams-cli-a" "$e/grams-cli-b"
go version -m "$e/grams-cli-a" > "$e/executable-build-info.txt"
go list -deps -f '{{if .CgoFiles}}{{.ImportPath}}:{{.CgoFiles}}{{end}}' ./benchmarks/grams | sed '/^$/d' > "$e/cgo-dependencies.txt"
test ! -s "$e/cgo-dependencies.txt"
"$e/grams-cli-a" -rows 128 -queries 6 -repeats 2 -corpus-only > "$e/corpus-a.json"
"$e/grams-cli-b" -rows 128 -queries 6 -repeats 2 -corpus-only > "$e/corpus-b.json"
cmp "$e/corpus-a.json" "$e/corpus-b.json"
sha256sum "$e/corpus-a.json" "$e/corpus-b.json"
if "$e/grams-cli-a" -rows 63 -corpus-only > "$e/invalid-flags.stdout" 2> "$e/invalid-flags.stderr"; then echo 'invalid flags accepted'; exit 1; else echo 'invalid flags rejected before opening DB'; fi
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./benchmarks/grams -count=1 -v -timeout=15m
sha256sum -c "$e/final-sha256.txt" > "$e/source-check.log"
echo 'FINAL HARNESS CGO0 NORMAL/VET/BUILD/DETERMINISM AND CGO1 RACE PASS'
date -u +%FT%TZ
echo 0 > "$e/harness-checks.exit"
source_id="tree:$(cat "$e/final-tree.txt")"
hardware='shared-cloud Linux/amd64; visible AMD EPYC 9V74 80-Core host model, 9 CPUs allowed; GOMAXPROCS=2; overlay storage of unknown physical medium; uncontrolled co-tenant load and OS cache; details in environment.txt'
for profile in smoke medium; do
 if [ "$profile" = smoke ]; then rows=128; queries=6; repeats=2; else rows=512; queries=12; repeats=3; fi
 date -u +%FT%TZ
 echo "Starting $profile: rows=$rows queries-per-length=$queries independent-builds=$repeats"
 cat /proc/loadavg > "$e/$profile-load-before.txt"
 if "$e/grams-cli-a" -rows "$rows" -queries "$queries" -repeats "$repeats" -seed 2026101052 -page-size 4096 -max-pages 32768 -source-id "$source_id" -environment "$hardware" > "$e/$profile.json" 2> "$e/$profile.stderr"; then echo 0 > "$e/$profile.exit"; else code=$?; echo "$code" > "$e/$profile.exit"; echo "$profile FAILED; original JSON/stderr preserved"; exit "$code"; fi
 cat /proc/loadavg > "$e/$profile-load-after.txt"
 echo "$profile native standalone benchmark PASS"
 date -u +%FT%TZ
done
sha256sum -c "$e/final-sha256.txt" > "$e/source-check-after-measurement.log"
echo 'FINAL SOURCE AND BOTH DECLARED ORDINARY MEASUREMENT PROFILES PASS'
