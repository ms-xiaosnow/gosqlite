#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/5e06a74e5f65/durable-restart
EVIDENCE="$ROOT/public-open-combined-independent-v1"
PREFIX="$EVIDENCE/checks"
CONFIG="$EVIDENCE/approved-config.json"
test -f "$CONFIG"
test -f "$EVIDENCE/SOURCE_REVIEW.md"
test -s "$EVIDENCE/compiler-release.txt"
test ! -e "$PREFIX.exit"
test ! -e "$PREFIX.stages.tsv"
set -o noclobber
export PATH="$ROOT/toolchains/go1.27.2/bin:$PATH"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
export GOCACHE="$ROOT/go-cache" GOPATH="$ROOT/go-path"
export CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
export SQLITE_ORACLE="$ROOT/toolchains/sqlite-build-3540000/sqlite3"
export EVIDENCE CONFIG
stage=initialization
trap 'rc=$?; printf "AGGREGATE EXIT %s AT %s LAST_STAGE %s\n" "$rc" "$(date -u +%FT%TZ)" "$stage"; printf "%s\n" "$rc" > "$PREFIX.exit"' EXIT
run_stage() {
    stage=$1
    shift
    local start end rc suffix
    for suffix in started command stdout stderr finished exit; do
        test ! -e "$PREFIX.$stage.$suffix"
    done
    start=$(date -u +%FT%TZ)
    printf '%s\n' "$start" > "$PREFIX.$stage.started"
    { printf '%q ' "$@"; printf '\n'; } > "$PREFIX.$stage.command"
    printf '\nSTAGE %s START %s\nCOMMAND ' "$stage" "$start"
    printf '%q ' "$@"
    printf '\n'
    set +e
    "$@" > "$PREFIX.$stage.stdout" 2> "$PREFIX.$stage.stderr"
    rc=$?
    set -e
    end=$(date -u +%FT%TZ)
    printf '%s\n' "$end" > "$PREFIX.$stage.finished"
    printf '%s\n' "$rc" > "$PREFIX.$stage.exit"
    cat "$PREFIX.$stage.stdout"
    cat "$PREFIX.$stage.stderr" >&2
    printf 'STAGE %s EXIT %s AT %s\n' "$stage" "$rc" "$end"
    printf '%s\t%s\t%s\t%s\n' "$stage" "$start" "$end" "$rc" >> "$PREFIX.stages.tsv"
    if [ "$rc" -ne 0 ]; then exit "$rc"; fi
}
printf 'PUBLIC OPEN COMBINED EXISTING ORDINARY AGGREGATE START %s\n' "$(date -u +%FT%TZ)"
printf 'Exact approved source only. No source changes, Git, remote, CI, new campaigns, manual tags, kill/cancel/watchdog, cleanup, relocation or retries.\n'
run_stage resource-precheck python3 "$EVIDENCE/precheck.py"
run_stage resource-snapshot bash -c 'df -B1 /tmp "$1"; df -i /tmp "$1"; free -b; printf "TMPDIR=%s\nGOTMPDIR=%s\n" "${TMPDIR:-/tmp (default)}" "${GOTMPDIR:-unset}"' _ "$ROOT"
run_stage source-identity-before python3 "$EVIDENCE/identity.py" before
VERIFY=$(python3 -c 'import json,os; print(json.load(open(os.environ["CONFIG"]))["verify"])')
cd "$VERIFY"
run_stage environment bash -c 'set -e; go version; test "$(go env GOVERSION)" = go1.27.2; test "$(go env GOOS)/$(go env GOARCH)" = linux/amd64; go env GOVERSION GOOS GOARCH GOTOOLCHAIN GOPROXY GOSUMDB GOPATH GOCACHE CGO_ENABLED GOFLAGS; printf "GOMAXPROCS=%s\nSQLITE_ORACLE=%s\n" "$GOMAXPROCS" "$SQLITE_ORACLE"; sha256sum "$1/toolchains/go1.27.2/bin/go" "$SQLITE_ORACLE" "$EVIDENCE/run.sh" "$EVIDENCE/identity.py" "$EVIDENCE/precheck.py" "$CONFIG" "$EVIDENCE/SOURCE_REVIEW.md"' _ "$ROOT"
run_stage sourceguard-before go run -p=1 ./internal/testscope
run_stage cgo0-unit go test -p=1 ./... -count=1 -timeout=0
run_stage cgo0-vet go vet -p=1 ./...
run_stage cgo0-build go build -p=1 -trimpath ./...
run_stage full-ordinary-oracle ./scripts/test-oracle.sh
run_stage sourceguard-before-race go run -p=1 ./internal/testscope
run_stage cgo1-race env CGO_ENABLED=1 go test -p=1 -race ./... -count=1 -timeout=0
for target in linux/arm64 darwin/arm64 darwin/amd64 windows/amd64 windows/arm64; do
    run_stage "cross-build-${target//\//-}" env CGO_ENABLED=0 GOOS="${target%/*}" GOARCH="${target#*/}" go build -p=1 ./...
done
run_stage module-integrity go mod verify
run_stage source-identity-after python3 "$EVIDENCE/identity.py" after
stage=complete
printf 'PUBLIC OPEN COMBINED FULL FROZEN ORDINARY AGGREGATE PASS %s\n' "$(date -u +%FT%TZ)"
