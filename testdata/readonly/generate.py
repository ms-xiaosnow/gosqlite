#!/usr/bin/env python3
"""Regenerate read-only fixtures with the pinned, test-only official SQLite CLI.
No sqlite3 Python module, third-party packages, network, or production helper.
Usage: python3 testdata/readonly/generate.py /path/to/sqlite3
"""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
CLI = Path(sys.argv[1]).resolve()
version = subprocess.check_output([str(CLI), "-version"], text=True).strip()
if not version.startswith("3.54.0 "):
    raise SystemExit("expected the pinned SQLite 3.54.0 test oracle, got " + version)
body = (HERE / "fixture.sql").read_text(encoding="utf-8")
projection = "SELECT id,label,hex(payload) AS payload_hex,payload IS NULL AS payload_null,score,optional FROM samples ORDER BY id;"
manifest = {"oracle": version, "files": {}}
expected = None
for size in (512, 4096, 65536):
    dest = HERE / f"page{size}.db"
    # Replaces only generator-owned fixtures, never an input database.
    dest.unlink(missing_ok=True)
    subprocess.run([str(CLI), "-batch", "-bail", str(dest)],
                   input=f"PRAGMA page_size={size};\n" + body, text=True, check=True)
    check = subprocess.check_output([str(CLI), "-readonly", str(dest), "PRAGMA integrity_check;"], text=True)
    if check.strip() != "ok":
        raise SystemExit(check)
    result = subprocess.check_output([str(CLI), "-readonly", "-json", str(dest), projection])
    if expected is None:
        expected = result
        (HERE / "samples.json").write_bytes(result)
    elif json.loads(result) != json.loads(expected):
        raise SystemExit("page-size differential mismatch")
    manifest["files"][dest.name] = {"sha256": hashlib.sha256(dest.read_bytes()).hexdigest(), "bytes": dest.stat().st_size, "integrity_check": "ok"}
# A dedicated schema fixture forces both an interior root-1 B-tree and
# overflow in a CREATE TABLE statement without bloating the ordinary fixtures.
dest = HERE / "schema512.db"
dest.unlink(missing_ok=True)
schema_sql = "PRAGMA page_size=512;\n" + "\n".join(
    f"CREATE TABLE table_{i:03d}(id INTEGER PRIMARY KEY, value TEXT);" for i in range(80)
) + "\nCREATE TABLE wide(" + ",".join(f"c{i:03d} TEXT" for i in range(600)) + ");\n"
subprocess.run([str(CLI), "-batch", "-bail", str(dest)], input=schema_sql, text=True, check=True)
check = subprocess.check_output([str(CLI), "-readonly", str(dest), "PRAGMA integrity_check;"], text=True)
if check.strip() != "ok":
    raise SystemExit(check)
schema = subprocess.check_output([str(CLI), "-readonly", "-json", str(dest), "SELECT type,name,tbl_name,rootpage,sql FROM sqlite_schema ORDER BY rowid;"])
(HERE / "schema.json").write_bytes(schema)
manifest["files"][dest.name] = {"sha256": hashlib.sha256(dest.read_bytes()).hexdigest(), "bytes": dest.stat().st_size, "integrity_check": "ok"}
manifest["files"]["schema.json"] = {"sha256": hashlib.sha256(schema).hexdigest(), "rows": len(json.loads(schema))}
manifest["files"]["samples.json"] = {"sha256": hashlib.sha256(expected).hexdigest(), "rows": len(json.loads(expected))}
(HERE / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
print(json.dumps(manifest, indent=2))
