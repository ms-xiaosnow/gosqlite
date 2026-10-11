#!/usr/bin/env bash
# Use a fresh, empty destination. This installs development/test tools only.
set -euo pipefail
base="${1:?Usage: install-baselines.sh /absolute/empty/toolchains-dir}"
case "$base" in
  /*) ;;
  *) echo 'Toolchain directory must be absolute.' >&2; exit 2 ;;
esac
mkdir -p "$base/downloads" "$base/evidence"
cd "$base"
test ! -e go1.27.2
test ! -e go
test ! -e sqlite-src-3540000
test ! -e sqlite-build-3540000
curl -fL --proto '=https' --tlsv1.2 https://go.dev/dl/go1.27.2.linux-amd64.tar.gz -o downloads/go1.27.2.linux-amd64.tar.gz
printf '%s  %s\n' ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5 downloads/go1.27.2.linux-amd64.tar.gz | sha256sum -c -
curl -fL --proto '=https' --tlsv1.2 https://www.sqlite.org/2026/sqlite-src-3540000.zip -o downloads/sqlite-src-3540000.zip
python3 - <<'PY'
import hashlib,pathlib
p=pathlib.Path('downloads/sqlite-src-3540000.zip')
h=hashlib.sha3_256(p.read_bytes()).hexdigest()
assert h=='a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e',h
print(p, 'SHA3-256:', h, 'OK')
PY
tar -xzf downloads/go1.27.2.linux-amd64.tar.gz
mv go go1.27.2
unzip -q downloads/sqlite-src-3540000.zip
./go1.27.2/bin/go version
mkdir sqlite-build-3540000
cd sqlite-build-3540000
../sqlite-src-3540000/configure --all --disable-readline --disable-tcl --disable-shared > ../evidence/sqlite-configure.log 2>&1
make -j2 sqlite3 > ../evidence/sqlite-build.log 2>&1
python3 - <<'PY'
import hashlib,pathlib
h=hashlib.sha3_256(pathlib.Path('sqlite3.c').read_bytes()).hexdigest()
assert h=='8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e',h
print('generated sqlite3.c SHA3-256:',h,'OK')
PY
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
oracle_exe="$oracle_work_dir/sqlite3"
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
/usr/bin/env -i LC_ALL=C LANG=C TZ=UTC "SQLITE_TMPDIR=$oracle_temp_root" "TMPDIR=$oracle_temp_root" "$oracle_exe" -noinit --version
/usr/bin/env -i LC_ALL=C LANG=C TZ=UTC "SQLITE_TMPDIR=$oracle_temp_root" "TMPDIR=$oracle_temp_root" "$oracle_exe" -noinit :memory: 'SELECT sqlite_version(), sqlite_source_id(); PRAGMA integrity_check;'
