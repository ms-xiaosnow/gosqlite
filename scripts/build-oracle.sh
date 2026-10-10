#!/usr/bin/env bash
# Build the pinned, official SQLite CLI for tests only. No upstream tests run.
set -euo pipefail
umask 077
if (( $# < 1 || $# > 2 )); then
  echo "Usage: $0 /absolute/new/directory [/absolute/sqlite-src-3540000.zip]" >&2
  exit 2
fi
base=$1
archive=${2:-}
case "$base" in
  /*) ;;
  *) echo 'Oracle build directory must be absolute and must not exist.' >&2; exit 2 ;;
esac
for tool in python3 unzip make cc; do
  command -v "$tool" >/dev/null || { echo "Required build tool missing: $tool" >&2; exit 1; }
done
if [[ -n "$archive" ]]; then
  case "$archive" in
    /*) ;;
    *) echo 'Local source archive must be an absolute path.' >&2; exit 2 ;;
  esac
  test -f "$archive"
else
  command -v curl >/dev/null
fi
# Refuse any existing destination rather than overwrite another toolchain.
mkdir -- "$base"
mkdir -- "$base/evidence"
if [[ -z "$archive" ]]; then
  mkdir -- "$base/downloads"
  archive="$base/downloads/sqlite-src-3540000.zip"
  curl -fL --proto '=https' --tlsv1.2 --retry 3 \
    https://www.sqlite.org/2026/sqlite-src-3540000.zip -o "$archive"
fi
# An explicit local archive is an offline input, never a verification bypass.
python3 - "$archive" <<'PY'
import hashlib
import pathlib
import sys
p = pathlib.Path(sys.argv[1])
if p.stat().st_size != 14881448:
    sys.exit(f'SQLite source archive size mismatch: {p.stat().st_size} bytes')
with p.open('rb') as source:
    actual = hashlib.file_digest(source, 'sha3_256').hexdigest()
expected = 'a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e'
if actual != expected:
    sys.exit(f'SQLite source archive SHA3-256 mismatch: {actual}')
print(f'SQLite source archive SHA3-256: {actual} OK')
PY
unzip -q "$archive" -d "$base"
mkdir -- "$base/sqlite-build-3540000"
cd "$base/sqlite-build-3540000"
if ! ../sqlite-src-3540000/configure --all --disable-readline --disable-tcl --disable-shared \
  > ../evidence/sqlite-configure.log 2>&1; then
  cat ../evidence/sqlite-configure.log >&2
  exit 1
fi
if ! make -j2 sqlite3 > ../evidence/sqlite-build.log 2>&1; then
  cat ../evidence/sqlite-build.log >&2
  exit 1
fi
python3 - <<'PY'
import hashlib
import pathlib
import sys
with pathlib.Path('sqlite3.c').open('rb') as source:
    actual = hashlib.file_digest(source, 'sha3_256').hexdigest()
expected = '8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e'
if actual != expected:
    sys.exit(f'Generated sqlite3.c SHA3-256 mismatch: {actual}')
print(f'Generated sqlite3.c SHA3-256: {actual} OK')
PY
expected='3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3'
actual=$(./sqlite3 -batch -bail -noheader -list :memory: 'SELECT sqlite_version(),sqlite_source_id();')
if [[ "$actual" != "$expected" ]]; then
  printf 'SQLite oracle identity mismatch: %s\n' "$actual" >&2
  exit 1
fi
printf 'SQLite oracle identity: %s OK\n' "$actual"
./sqlite3 --version
printf 'Test-only SQLITE_ORACLE=%s/sqlite-build-3540000/sqlite3\n' "$base"
