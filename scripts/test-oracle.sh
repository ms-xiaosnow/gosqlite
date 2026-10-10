#!/bin/sh
set -eu
# Ordinary tests only. Keep manual physical-failure suites excluded.
: "${SQLITE_ORACLE:?Set SQLITE_ORACLE to the verified official SQLite 3.54.0 CLI}"
export CGO_ENABLED=0 GOTOOLCHAIN=local
test "$(go env GOVERSION)" = go1.27.2
case "$SQLITE_ORACLE" in
  /*) ;;
  *) echo 'SQLITE_ORACLE must be an absolute executable path.' >&2; exit 2 ;;
esac
if [ ! -f "$SQLITE_ORACLE" ] || [ ! -x "$SQLITE_ORACLE" ]; then
  echo 'SQLITE_ORACLE must identify an existing executable file.' >&2
  exit 2
fi
# The reference guard precedes the oracle invocation and all database tests.
# Behavioral review of test bodies and helpers is additionally required.
go run ./internal/testscope
expected='3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3'
actual=$("$SQLITE_ORACLE" -batch -bail -noheader -list :memory: 'SELECT sqlite_version(),sqlite_source_id();')
if [ "$actual" != "$expected" ]; then
  printf 'SQLite oracle identity mismatch: %s\n' "$actual" >&2
  exit 1
fi
# No cached acceptance, -run filter, package subset, or manual build tag.
exec go test -count=1 -timeout=30m -tags oracle ./...
