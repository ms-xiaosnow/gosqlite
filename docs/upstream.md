# Verified development and oracle baselines

Verified 2026-10-10. All artifacts are outside the project repository. No repository code was edited or committed. No SEE private materials were accessed. No database fault-injection, crash, or process-exit tests were run.

## Executables and source

Base: `/workspace/scratch/5e06a74e5f65/durable-restart/toolchains`

- Go: `go1.27.2/bin/go` and `go1.27.2/bin/gofmt`
- Test-only SQLite CLI oracle: `sqlite-build-3540000/sqlite3`
- Complete public SQLite source: `sqlite-src-3540000/`
- Go smoke program and statically linked executable: `smoke-go/`
- Reusable installer: `install-baselines.sh` (run against a fresh destination)

Do not put the SQLite C source/CLI on any production execution or linking path. It exists solely as a development/test oracle. Invoke Go by its absolute path or prepend this Go bin directory; `/usr/bin/go` is not this compiler.

## Provenance

### Go 1.27.2

- Official downloads and published SHA-256: https://go.dev/dl/
- Official machine-readable metadata saved as `evidence/go-downloads.json`: https://go.dev/dl/?mode=json
- Download: https://go.dev/dl/go1.27.2.linux-amd64.tar.gz
- Bytes: 70,590,635
- Published and recalculated SHA-256: `ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5`
- License: BSD-style 3-clause, https://go.dev/LICENSE; original retained at `go1.27.2/LICENSE`.

### SQLite 3.54.0

- Official download/hash page: https://www.sqlite.org/download.html
- Release and source identity: https://www.sqlite.org/releaselog/3_54_0.html
- Download: https://www.sqlite.org/2026/sqlite-src-3540000.zip
- Bytes: 14,881,448
- Published and recalculated archive SHA3-256: `a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e`
- Independently calculated archive SHA-256: `8847659821e0c5116bd14a94644c82ba932b2d9a05ac81af2e34af814aad7c58` (not a published hash claim).
- `SQLITE_SOURCE_ID`: `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`
- Canonical check-in in extracted `manifest.uuid`: `be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`
- Extracted `manifest.tags` includes `version-3.54.0`, `release`, `major-release`, and `trunk`.
- Published and recalculated generated `sqlite3.c` SHA3-256: `8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e`
- Local compiled oracle SHA-256: `470775f6ada29ccfb2d3046ac2b4c975df95a9c20b782332d4e8ba6b6233e6c6` (machine/build-path-specific binary, not an upstream release hash).
- SQLite deliverable code/CLI is public domain: https://www.sqlite.org/copyright.html and extracted `sqlite-src-3540000/LICENSE.md`.
- Build tools are not blanket public domain. Extracted `LICENSE.md` identifies autosetup/configure and some legacy autoconf files as BSD-style exceptions. Preserve their original notices if redistributed. `sqlite-src-3540000/autosetup/LICENSE` retains the WorkWare two-condition BSD-style notice. No upstream files were modified.

Official page snapshots are in `evidence/sqlite-download.html` and `evidence/sqlite-release-3_54_0.html`.

## Actual build and verification

Host: Linux x86_64; compiler `gcc (Debian 14.2.0-19) 14.2.0`; `tclsh` reports 8.6. Curl, tar, unzip, make, gcc, Python 3 were already available.

Build commands from the toolchains base:

```sh
mkdir -p sqlite-build-3540000
cd sqlite-build-3540000
../sqlite-src-3540000/configure --all --disable-readline --disable-tcl --disable-shared > ../evidence/sqlite-configure.log 2>&1
make -j2 sqlite3 > ../evidence/sqlite-build.log 2>&1
./sqlite3 --version
./sqlite3 :memory: 'SELECT sqlite_version(), sqlite_source_id(); PRAGMA integrity_check;'
```

Configure and make returned exit status 0. CLI output:

```text
3.54.0 2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3 (64-bit)
3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3
ok
```

CLI embeds SQLite C and has system dependencies libm, libz, libc. No dynamic libsqlite3 dependency. Compilation enables FTS5, FTS4, RTree, JSON, session and other `--all` features. The shell uses `SQLITE_DQS=0`; exact enabled options are in `evidence/sqlite-compile-options.txt` and the configure/build logs. No full upstream SQLite suite was run.

Go commands from the toolchains base:

```sh
./go1.27.2/bin/go version
GOCACHE="$PWD/go-cache" GOTOOLCHAIN=local CGO_ENABLED=0 ./go1.27.2/bin/go build -o smoke-go/toolchain-smoke ./smoke-go/main.go
./smoke-go/toolchain-smoke
./go1.27.2/bin/go version smoke-go/toolchain-smoke
file smoke-go/toolchain-smoke
```

Results: `go version go1.27.2 linux/amd64`; build exit 0; program prints `native-go-toolchain-ok`; executable records `go1.27.2`; `file` reports a statically linked x86-64 ELF. This verifies compiler installation only, not project conformance.

The first attempt to collect post-build CLI evidence occurred just before linking completed and returned "No such file or directory". After the original build reached exit status 0, the same evidence collection succeeded. No second compile or workaround was needed.

Captured results: `evidence/toolchain-verification.txt`, `evidence/SHA256SUMS`, `evidence/sqlite-compile-options.txt`, `evidence/sqlite-configure.log`, `evidence/sqlite-build.log`.

For project execution use `GOTOOLCHAIN=local` and `CGO_ENABLED=0` to avoid automatic toolchain switching or accidental CGO production dependencies. Race verification is a separate platform-dependent gate; it was not run here.
