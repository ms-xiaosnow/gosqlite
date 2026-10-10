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

## M1 behavior references used

The native record/B-tree implementation follows the official file format and the pinned public source's payload and varint rules. Scalar conversion/formatting was independently implemented from observable3.54.0 behavior, with util.c and printf.c used as public-domain references; no C lookup tables are copied. Exact official-oracle results fix numeric rounding boundaries in Go tests.

- https://www.sqlite.org/fileformat2.html
- https://www.sqlite.org/lockingv3.html
- https://www.sqlite.org/lang_expr.html
- https://www.sqlite.org/datatype3.html



## M4 Unicode dependency: golang.org/x/text v0.43.0

Downloaded and verified 2026-10-10 using Go1.27.2 with GOPROXY=https://proxy.golang.org and GOSUMDB=sum.golang.org. Exact downloaded module checksum and origin matched independently retrieved official metadata:

- Release timestamp:2026-10-09T15:18:32Z
- Origin: https://go.googlesource.com/text, tag refs/tags/v0.43.0, commit ac40b0f06c159afb5bfca4d1860f8b4b60dbc160
- Module h1:1QivrlhwAsnMOcqgOdl9my7s9OyZl8O5/UGUTJVXoWc=
- go.mod h1:hbSIYA/amXcRXTFSZqh8tWHEoqRvvbKPJ2uz/KU+a+s=
- Independently recomputed downloaded module ZIP SHA256:b75131f628f6e2e4892fd638d33127e952a5027775e0162b94ba069cb4e13863 (not an upstream-published ZIP hash claim)
- Required Go1.26.0; project uses1.27.2
- License:BSD-3-Clause, unmodified notice retained under third_party/

Primary metadata: [official proxy version](https://proxy.golang.org/golang.org/x/text/@v/v0.43.0.info), [Go checksum database](https://sum.golang.org/lookup/golang.org/x/text@v0.43.0), [tagged go.mod](https://raw.githubusercontent.com/golang/text/v0.43.0/go.mod), [tagged license](https://raw.githubusercontent.com/golang/text/v0.43.0/LICENSE), [normalization package](https://pkg.go.dev/golang.org/x/text/unicode/norm), [casefold package](https://pkg.go.dev/golang.org/x/text/cases).

The normalization implementation is stream-safe and may insert U+034F after long nonstarter runs; grams13 must explicitly reject such insertions or persist that policy rather than mislabeling unrestricted canonical equivalence. Casefold can change normalization form, so normalize again afterward. Actual versioned behavior belongs to tokenizer tests and config metadata.

## M6 public pgvector semantic reference

Retrieved2026-10-10 from the official pgvector GitHub repository, tag v0.8.7, verified commit f37c13f68b57d2c3472b2214fbcff699d6d34876. Public source and published expected outputs were read only; PostgreSQL/pgvector was not built or executed. Independently calculated SHA256 values (not upstream-published checksum claims):

- README.md:19cebadeeb8d3181d01ef00399674ec41bd2f0363850cdccc7685d4260c5a1c6
- src/vector.c:d97ce73c3e3977be2b086e01d39352376f3b064f3eef30efe12b28042c7fd1f1
- src/vector.h:a67272ea44553d174968f575a91da71dfe82b8278ec2c70d51e37c154e2f0049
- LICENSE:6bba9ebeb73e27477463b05e5ef1bf303bccbddb3db9bbc95905d351604d6a87
- test/sql/vector_type.sql:592f055f8d1940c0ac603883ad2f65700ef5629909af049d6a02bca041927f45
- test/expected/vector_type.out:0434592d5d32797d42ff6cfcfdf9e27353ae7f651d9ec2078fcc44c490ba51d6

Source: https://github.com/pgvector/pgvector/tree/f37c13f68b57d2c3472b2214fbcff699d6d34876. Retained fixture metadata and deliberate differences are in testdata/pgvector-v0.8.7/manifest.json and README.md. Exact upstream license is retained, including its copyright year range. The first guessed fixture path test/sql/vector.sql returned404; official directory listing located vector_type.sql. No older version was silently substituted. No private SEE source is accessed.

The specified Neon migration page and its .md variant returned fetch errors during prerequisite research. The official sunset announcement and old pg_embedding repository were readable. M9 must reverify the requested current migration chapter and snapshot date; these partial references do not establish full guide conformance.

## M7 public ANN algorithm references

Public pgvector v0.8.7, exact commit
`f37c13f68b57d2c3472b2214fbcff699d6d34876`, was read for documented HNSW/IVF
parameters and algorithm context. Official files fetched from that immutable
commit on2026-10-10: hnsw.h, hnswbuild.c, hnswutils.c, hnswscan.c, ivfflat.h,
ivfflat.c, ivfkmeans.c, ivfbuild.c, ivfscan.c and sql/vector.sql. No C implementation
is compiled/copied/invoked by production. Exact independently computed SHA256:

```text
c57af43d484ac0406bee00a7092f655f53c4b14f05f76c3a289baa34bdaf9647  src/hnsw.h
7f2564af813a06775a22e89d106a1e83ff1e951ea834fb16f93a35ab399f57fa  src/hnswbuild.c
52913cfdb915cd564712431e9ab67c28898a0a041414500fa67ba7818518f641  src/hnswutils.c
fd1dd993f51b05eed167c7ce12eb11ad3ad9a951af70ad7d99b321e18bc6180d  src/hnswscan.c
be00dd3383cc4921fa3e252c16b7bf1f6e418f32b8c8b08f2a0070cb76af399b  src/ivfflat.h
652309192fdd909048371ed24f5026ae0ebd1eecf253ca6512820ce114ea71e0  src/ivfflat.c
cf17de636966ee4af897d439b7fcc3893b56c5ff7a5e60146b78657f849061a6  src/ivfkmeans.c
f355fcee013b330ce98f54f28b73643bc14a76aff91afb332bc2eb6ae7a292a0  src/ivfbuild.c
4699881a03878a569d819b60fcddeaf108f34d5c027eddbdfe82a4991245f0e2  src/ivfscan.c
a4ceca5931cb614b40bec09ce6994a57058513d9af612c15b1458a338134609d  sql/vector.sql
```

Primary source: https://github.com/pgvector/pgvector/tree/f37c13f68b57d2c3472b2214fbcff699d6d34876
The PostgreSQL-style license already retained for public fixtures applies to
these references. Native topology, tombstones, finite float64 arithmetic and
storage are local contracts, not bit-for-bit upstream compatibility. No running
PostgreSQL/pgvector performance or recall comparison was performed.

## M8 memory-primitive dependencies

Verified2026-10-10 using Go1.27.2 official proxy+checksum database, then independent
ZIP hashes. No automatic toolchain switching, private crypto source or credentials.

- x/crypto v0.58.0, timestamp2026-10-09T16:24:17Z, official origin
  https://go.googlesource.com/crypto at8fdf07b3114278fb3b78c1cf1814e08bd851eb4c.
  Module h1:`COkYLr4k7nDI4r6QtMML2AfzeIswWHAJthLS6K/M/54=`;
  go.mod h1:`Xh+kl5A+M0gMiWDytH2Hhr5Z7WbNCpv/eB/EFgfSk+w=`.
  ZIP SHA256:`b0a45594cda20ddb4264d84aeff9b588b8570bf6a2833c51a0b821734a298d95`.
- x/sys v0.49.0, timestamp2026-10-06T18:35:47Z, official origin
  https://go.googlesource.com/sys atb6b8557e8e2974925aec7a136ba0e323eedc5f43.
  Module h1:`XbzkgYJHdqh/8m2Uu0W/dQv8nktxx4BFHp1M0gROTXA=`;
  go.mod h1:`hNLxWAXmnKAxqDtdwIYC4bM9oQPEecfsnNMuSxOs3og=`.
  ZIP SHA256:`e73a06591ca0d7016ce3ccb45c36a8ed7128468aecc1a357f94d815c760c6b31`.
- Both retained BSD3 licenses SHA256:
  `911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad`.

Actual added import closure: x/crypto/argon2, x/crypto/blake2b, x/sys/cpu and
x/sys/unix (the latter added by Linux encrypted storage); all CgoFiles empty.
The same pinned x/sys module is now a direct dependency. Experimental encrypted
snapshot integration is documented separately. Known-answer/reference tests do
not constitute an independent audit.

## M9 public migration and extended-value references

The old embedding extension profile is version0.3.6, exact public commit
5d48508aeaeb86b9e9c630468ada7a124b905795. Exact SQL/control source and Apache2
LICENSE/NOTICE are retained under testdata/pgembedding-0.3.6 with locally computed
SHA256/size manifest. The upstream NOTICE includes its HNSW attribution; that
C/C++ implementation is not bundled or invoked. Adjacent catalog/report fixtures
are locally authored synthetic data, not server captures.

The current requested Neon page was readable on2026-10-10 but lacked the requested
migration chapter. The fixture manifest identifies the historical official2024
chapter and snapshot. No claim is made to have followed a nonexistent current
chapter or executed the read-only exporter against PostgreSQL. Later migration
steps must separately verify actual target versions, ownership and privileges.

Half/bit/sparse public SQL/expected fixtures use the same pgvector v0.8.7 commit
f37c13f68b57d2c3472b2214fbcff699d6d34876. Exact retained bytes/hashes and read-only
source references are in testdata/pgvector-v0.8.7/extended/manifest.json; the
previously retained pgvector license applies. Production is independently written
Go;207 extracted published cases and separate mathematics are tested. PostgreSQL/
pgvector was not built or run. Sparse malformed-zero-index validation, finite
float64 versus upstream overflow/NaN, and explicit lossy conversions are recorded
in extension/vector/extended-values.md. No new production dependency was added.

## Native global aggregate behavior reference

The bounded global-aggregate SQL increment uses the same pinned 3.54.0 public
source identity above. `src/func.c` supplies the count/min/max and compensated
sum/total/avg numeric behavior; `src/vdbe.c` supplies numeric-type conversion;
`src/util.c` supplies infinity/NaN overflow classification. Native Go state and
pull execution are in internal/sql/aggregate.go and sqlite/aggregate.go. Tagged
complete-value/type/error tests invoke only the existing official oracle; no
production C dependency or private SEE material is introduced. SQLite's
public-domain notice applies to the referenced implementation algorithms.
