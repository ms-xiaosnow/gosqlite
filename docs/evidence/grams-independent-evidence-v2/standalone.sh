#!/bin/bash
set -euo pipefail
b=/workspace/scratch/5e06a74e5f65/durable-restart
e="$b/grams-independent-evidence-v2"
cd "$b/grams-independent-review-v2"
export PATH="$b/toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$b/go-cache" GOPATH="$b/go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
trap 's=$?; echo "$s" > "$e/standalone.exit"' EXIT
{
 date -u +%FT%TZ
 grep 'model name' /proc/cpuinfo | head -1
 grep -E 'Cpus_allowed_list|Mems_allowed_list' /proc/self/status
 df -T "$e"
 free -h
 cat /proc/loadavg
 printf '%s\n' 'Shared container; uncontrolled concurrent workload including independent correctness/race checks; storage physical medium unknown; no OS cache eviction; not fixed-hardware release qualification.'
} > "$e/environment.txt"
go run ./internal/testscope
"$e/grams-independent" -rows 128 -queries 6 -repeats 2 -corpus-only > "$e/smoke-corpus-a.json"
"$e/grams-independent" -rows 128 -queries 6 -repeats 2 -corpus-only > "$e/smoke-corpus-b.json"
cmp "$e/smoke-corpus-a.json" "$e/smoke-corpus-b.json"
sha256sum "$e/smoke-corpus-a.json" "$e/smoke-corpus-b.json" > "$e/corpus-sha256.txt"
"$e/grams-independent" -rows 128 -queries 6 -repeats 2 -seed 2026101052 -page-size 4096 -max-pages 32768 -source-id tree:762a9e9dd8105cf620c68a8398922ddf254a9d5f -environment 'Independent smoke; shared Linux/amd64 container, AMD EPYC 9V74 host model, 9 allowed CPUs, GOMAXPROCS=2; overlay storage physical medium unknown; uncontrolled co-tenant load including independent test/race checks; no OS cache eviction' > "$e/smoke.json" 2> "$e/smoke.stderr"
python3 "$e/audit_report.py" "$e/smoke.json" "$e/smoke-corpus-a.json" > "$e/smoke-independent-audit.json"
sha256sum -c "$b/grams-acceptance-evidence-v2/final-sha256.txt" > "$e/source-check-after.log"
echo 'Independent standalone native smoke and independent result/counter/statistics audit PASS'
