#!/usr/bin/env bash
# Use a fresh, empty destination. This installs development/test tools only.
set -euo pipefail
base="${1:?Usage: install-baselines.sh /absolute/empty/toolchains-dir}"
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
./sqlite3 --version
./sqlite3 :memory: 'SELECT sqlite_version(), sqlite_source_id(); PRAGMA integrity_check;'
