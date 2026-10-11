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
# Only the SQLite child receives a clean environment. The shell, compiler,
# downloader, and trusted /usr/bin utilities retain their existing bootstrap.
# This reviewed launch contract is Linux-only; it does not prove OS isolation.
if [ "$(/usr/bin/uname -s)" != Linux ]; then
  echo 'The isolated oracle launch contract is reviewed only for Linux.' >&2
  exit 2
fi
oracle_absolute_path() {
  case "$1" in
    /*) ;;
    *) return 1 ;;
  esac
  case "$1" in
    *//*|*/./*|*/../*|*/.|*/..|*\\*) return 1 ;;
    /) ;;
    */) return 1 ;;
  esac
}
oracle_temp_root=${GOTMPDIR:-${TMPDIR:-/tmp}}
oracle_work_dir=$(pwd -P)
oracle_exe="$SQLITE_ORACLE"
for oracle_path in "$oracle_exe" "$oracle_temp_root" "$oracle_work_dir"; do
  if ! oracle_absolute_path "$oracle_path"; then
    echo 'Oracle executable, temp root, and CWD must be clean absolute Linux paths.' >&2
    exit 2
  fi
done
case "$oracle_temp_root" in
  /tmp|/tmp/*) ;;
  *) echo 'Oracle temp root must be /tmp or a descendant.' >&2; exit 2 ;;
esac
# Directory usability is a preflight check, not a proof against later changes or
# SQLite fallback. A separately approved execution must retain this namespace.
if [ ! -d "$oracle_temp_root" ] || [ ! -w "$oracle_temp_root" ] || [ ! -x "$oracle_temp_root" ]; then
  echo 'Oracle temp root must be an existing writable searchable directory.' >&2
  exit 2
fi
if [ ! -f "$oracle_exe" ] || [ ! -x "$oracle_exe" ]; then
  echo 'Oracle executable must be an existing executable file.' >&2
  exit 2
fi
cd "$oracle_work_dir"
expected='3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3'
actual=$(/usr/bin/env -i LC_ALL=C LANG=C TZ=UTC "SQLITE_TMPDIR=$oracle_temp_root" "TMPDIR=$oracle_temp_root" "$oracle_exe" -noinit -batch -bail -noheader -list :memory: 'SELECT sqlite_version(),sqlite_source_id();')
if [ "$actual" != "$expected" ]; then
  printf 'SQLite oracle identity mismatch: %s\n' "$actual" >&2
  exit 1
fi
# No cached acceptance, -run filter, package subset, or manual build tag.
exec go test -count=1 -timeout=30m -tags oracle ./...
