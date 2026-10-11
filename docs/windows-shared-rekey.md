# Windows shared encrypted rekey: source, protocol and evidence

2026-10-11. **Implemented in this unpublished source composition; native Windows
runtime NOT_RUN.** This is not M3/M8/M10, cryptographic, durability or release
acceptance. Documentation changes do not create execution permission.

## Exact identities and publication boundary

The published base is commit `67318ded6416547f26d685719d3e01e208454a87`, tree
`1452ea832a41bdb8214bffd98fdb1b4d4fc37757`. Its exact Library restore completed
both focused checks and all 15 qualification gates at 13:27 UTC. Its existing
[CI run 38141275294](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38141275294)
is SUCCESS, with [ordinary job 114475481426](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38141275294/job/114475481426)
and [oracle job 114475481239](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38141275294/job/114475481239).
The nonverbose CI logs do not provide per-test path counts or prove zero skips.
Main's local restore freshly observed ordinary/race 5,570 PASS + seven expected
oracle skips, complete oracle 7,202 PASS/zero test skips, and the two-root LIMIT
oracle race 394 PASS. Those are base results only.

The reviewed Windows source candidate is recursive Git-format tree
`b0e21b81b6eee598124171ec2fb7f4c6df77751a`: 3,153 regular files, 148 directory
records including root, exactly nine changed/added source paths over that base.
This document and the bounded companion edits create a separate documentation
composition; b0e identifies its unchanged input code tree, not the final
documentation tree or an invented commit. No new candidate commit, remote CI,
Library-restore qualification or native Windows execution is asserted here.

The nine source results (all POSIX 0644 / Git 100644) are:

| Source path | SHA-256 |
|---|---|
| [exact_locks.go](../internal/encstore/exact_locks.go) | `deab6e4f6e41bcde99b97fb1db08ab7c3fe5b78574b6e84c007981f074fb6307` |
| [lifecycle_windows_protocol.go](../internal/encstore/lifecycle_windows_protocol.go) | `c9672bf5155e629a3f2cbc0e8d3604ecd28509a24ee9f080a24cd355ee39aefd` |
| [lifecycle_windows_protocol_test.go](../internal/encstore/lifecycle_windows_protocol_test.go) | `f028cce257ab86d2ee7df26730e81827042aa498636477d4dd6675e5f16dcb89` |
| [lifecycle_windows_source_test.go](../internal/encstore/lifecycle_windows_source_test.go) | `f4d231b6a9f2147f7841f3dfc4bf1953ad9e3948aa2eba3bd26701a0cf44668d` |
| [native_windows_test.go](../internal/encstore/native_windows_test.go) | `dab439b3d89a78339962718db7d3b51cbf9fc69a0351ac58b7d38f83cc3a9ddf` |
| [platform_windows.go](../internal/encstore/platform_windows.go) | `41944a8ee182f69acd296282e7197e3d182c2eaff79166a386e7b3d9585c19aa` |
| [shared.go](../internal/encstore/shared.go) | `3a1cf34b05c25fae763a97085b64dba2c4629f402bf26363102172fc22e9c379` |
| [encryption_shared_rekey_test.go](../sqlite/encryption_shared_rekey_test.go) | `fd6eb9b944bdcfa14418abe0649b26434f077afbbb53184e6feccea691fa4e74` |
| [encryption_shared_test.go](../sqlite/encryption_shared_test.go) | `9378a090edcbff7979cf4df2c1c90719e18e4c7dd894655d534b13c8af7ce9ef` |

The initial eight-path reviewed result is retained exactly; the ninth path changes
only the existing quiescent test's owner cleanup/comment, preserving all 26
setup/operation/assertion obligations. Newer startup isolation, WAL provenance,
LIMIT/EXPLAIN, modules, scripts, workflows, manual gates and guard contracts are
preserved. Git tree identity does not prove actual non-executable permissions:
all 32 unusual file modes and both 0700 golden directories remain separately
accounted for. No `checkpoints.json` update is implied by this uncommitted slice.

## Bounded native protocol

The target remains Windows/amd64, Windows 10 1709+, local fixed NTFS, CGO0.
Storage modes 0/1/2, layouts 1/2/3, lock bytes 0/1/2, protocol byte, encrypted
record formats and providers are unchanged. See the
[native platform policy](native-encrypted-windows.md).

- Every updated Windows initial lifecycle acquisition, including read-only/shared
  and legacy exclusive owners, takes exclusive capture byte 2, acquires lifecycle
  byte 0 and releases capture. This is broader than the Rekey entrypoint alone.
- Shared rekey first reserves writer byte 1 and refreshes/authenticates the latest
  selection. Conversion requires ledger `[1,2,0]`, where 1 is shared and 2 is
  exclusive. It takes capture, releases shared lifecycle, tries exclusive once,
  and releases capture on success. There is a real non-atomic interval.
- Capture contention preserves `[1,2,0]`. Exclusive contention returns ordinary
  BUSY only after shared restoration and successful gate release. Unexpected
  native results, failed restoration (even BUSY), or failed release poison and
  retain unresolved owners/protectors. No wait or automatic retry loop is added.
- Updated admission cannot enter the protected interval. Older binaries can ignore
  it: an old shared owner can yield restored BUSY; an old exclusive owner can
  prevent restoration; a transient old owner can change authority and retire.
  The memory models represent these cases, not actual mixed-binary execution.

`DB.Rekey -> EncryptedSession.RekeyPrepared -> operationLocked ->
Store.BeginMaintenance` orders maintenance before copy budget, source Verify,
logical validation and output. After successful conversion, BeginMaintenance
again takes capture and calls `checkAuthority`: recheck retained directory/lock,
open the current named root, bound/read its extent, authenticate with retained
source master/options, compare exact root bytes, and verify handle/name identity.
Gate release is joined; a refusal joins `EndMaintenance(false)`. There is no
source-provider reacquisition, alternate graph adoption or fallback after the gap.
Preparing target credentials earlier is a distinct existing operation.

The post-conversion check also applies to successful Linux/Darwin shared rekey;
their backend source is unchanged. Linux/Darwin convert atomically rather than
using the Windows gap. Existing publisher authority/candidate checks remain,
and rename attempt remains the uncertainty boundary. Successful target-master
adoption follows verified publication. Downgrade precedes writer release.

## Resource and error limits

The exact Windows ledger still rejects direct shared-to-exclusive `change`
without releasing the shared hold. Unexpected lock/release outcomes retain
poison. A joined error containing BUSY is not automatically safe to retry.
Cleanup may retry a known-live managed lock obligation; it must never retry a
numeric native handle after an uncertain consuming Close. Existing `closeUnknown`
tombstones and parent pins remain, potentially for process lifetime.

Both SQLite rekey fixtures now have distinct owner references and checked
once-only DB.Close attempts, with attempted flags set before calls. This does
not instrument native Close or guarantee eventual retirement. The inherited
query helper still defers Rows.Close without checking its result. CLI helpers
retain their existing explicit and cleanup public DB.Close calls; that is neither
a once-only-owner proof nor evidence of repeated raw unknown-handle close.

Windows directory Sync supplies **no promised namespace-persistence barrier**.
Regular-file flush and handle-relative rename cannot be promoted to power-loss
ordering. Old roots/bases/segments remain retained and may still be decrypted
with old credentials: rekey is not secure erasure or historical-key revocation.
Whole-family replay still lacks an external freshness/identity anchor.

## Evidence layers on the exact code tree

| Layer | Actual result / boundary |
|---|---|
| Nine-path independent source review | No source blocker found; not runtime or security certification |
| Ten portable lifecycle memory laws | One fresh Linux normal command PASS: ten roots + 20 children = 30 named paths; inert callbacks only |
| Source integration control | Separate normal PASS, one path; source file I/O and Go AST, not native execution/control-flow proof |
| Latest-head ordinary shared rekey | Separate Linux correct-key normal PASS, one path; current data/identity/layout/head and new-key reopen |
| Existing quiescent/retained-lease test | Separate Linux normal PASS, one path; ordinary reader BUSY, retained lease, successful rekey and new-key reopen |
| CLI correct-key snapshot/wal/shared | Separate one-command normal PASS: root and three children = four paths; only shared reaches the new branch |
| Full b0e code qualification | **PASS**, 14:42 UTC: existing 15 gates plus Windows encstore test compilation and host symbol inspection, 17 total; details below |
| Native Windows capability/version/NTFS/LockFileEx/Close and mixed binaries | **NOT_RUN**; compile-only or Linux tests cannot supply these results |
| New source+documentation publication/restore/remote CI | **NOT_RUN**; current main's terminal result belongs to 67318ded |

The four normal commands finished at 13:39:57 UTC and released their compiler
lease at 13:39:59. The separate CLI command finished at 13:52:56 UTC and released
at 13:52:59. All five naturally exited zero, with exact named selections and empty
stderr. They are separate observations, not a replacement for full qualification.
The former 5d1e457 candidate's normal/race results retain their old identity.

Whole-repository source scope closed the earlier two-package-only omission:
`cmd/gosqlite.TestCLIMaintenanceCorrectKeyRoundTrips/shared` is an existing caller,
along with the quiescent SQLite root and new latest-head root. The candidate adds
12 portable roots/32 paths to ordinary and oracle selection. Expected full sets
were 5,602 PASS/seven known oracle skips in ordinary/race and 7,234 PASS/zero skips
in complete oracle; the fresh terminal run now independently observes those exact
sets. The frozen pre-terminal documentation V1 retains its original PENDING state.
The 41 fuzz roots/313 fixed seeds are not a fuzz-engine campaign. Existing Ubuntu
CI remains unchanged and broader than focused checks, but omits the local
five-crossbuild/dependency/LIMIT-oracle-race and Windows test-compile gates.
No new native runner is added. All manual recovery/namespace tags and gates stay
excluded. The old broad automatic-CI hold is not evidence of a newly unknown
selected Linux closure; execution/publication still needs its own exact approval.

Source/test names can be inspected above. Out-of-tree reports, not copied into
this bounded documentation change, are identified by exact filename and hash:

- `windows-rekey-rebased-independent-review-v1/REPORT.md`:
  `e20b4527d83ff20fefb06c5d5cc9577a3dae51800d838f01d49400397d38e455`.
- `windows-rekey-rebased-normal-verification-v1/REPORT.md`:
  `e246f08e3912f9814f78724b212629a345558ba440fa9c4d3e08a7577ab211ca`.
- `windows-rekey-cli-normal-verification-v1/REPORT.md`:
  `2a8cd2dbf3c9b8d370a7e9e7a249b161c9027cde3f2719db22ddad9853912302`.
- `windows-rekey-repository-scope-v1/REPORT.md`:
  `bbf18563ead97fbac0e70f2921bf83dcc3c3cc9b5b92af18df4c1a52614544cb`.

Existing Windows source/scoped archive Library
`libfile_e941d1feb72081918a2ebcc191900a85` predates the later CLI/full-repository
audit and must not be described as containing them. Published-main bundle
`libfile_eba6fe4a3af88191b588c5274ce29617` and terminal archive
`libfile_6138028dd5648191a7631b648e864f3b` belong to 67318ded only. A later archive
or final documentation identity needs its own verified record.

## Terminal seventeen-gate code qualification

The exact b0e code tree was run from 14:14:08.186681 UTC through the last natural
command return at 14:42:01.157503 UTC. Postchecks completed at 14:42:11.111010,
and the sole compiler lease was released at 14:42:11.115207 UTC. All 17 outer
commands naturally exited zero; the unchanged oracle script expands the total
to **19 explicit Go invocations** (16 other Go commands plus its env/run/test).
No retry, widened selector, assertion change, new runner or source repair occurred.

| Actual lane/gate | Fresh result on b0e |
|---|---|
| Ordinary ./... | 5,602 named PASS + seven expected oracle leaf SKIP; 1,241 Test, 41 Fuzz roots, 313 fixed seeds, six Examples |
| Ordinary race ./... | The same exact 5,602 PASS/seven SKIP set; CGO1 race instrumentation is separate from CGO0 production |
| Complete pinned SQLite 3.54.0 oracle script | 7,234 named PASS, zero test SKIP; 1,346 Test, 41 Fuzz roots/313 fixed seeds, six Examples |
| Two exact LIMIT oracle-race roots | 394 PASS = 49 phase + 345 conversion paths; not a full oracle-race suite |
| Guard/vet/CGO0 build/module verification | All PASS; existing startup/reference guard and source contracts unchanged |
| Five CGO0 builds | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64; no foreign execution |
| Whole-module non-test closure | 206 packages / 1,317 actual inputs: 1,255 Go + 54 assembly + six headers + two embeds; tooling legitimately includes testoracle/testscope/os/exec |
| Shipping sqlite/driver/CLI union | 164 packages / 1,228 actual inputs: 1,169 Go + 53 assembly + six headers; no testoracle/testscope/os/exec or native C-family/Syso input |
| Windows encstore test compile | PE32+ x86-64, 8,741,888 bytes; SHA-256 `c51ae619665b51c8c328d7d6735fd35241ac4aeac75097a156b8012785da7a40`; never executed |
| Host nm inspection | 8,054 symbols; new TestWindowsEncryptedSharedRekeyCapability present; obsolete refusal and manual namespace test absent; no PE execution |

The seven skipped leaves are SupportedFixtures/{512,4096,65536},
FragmentedTrees/{512,4096,65536} and UnsupportedCatalogAndDirtyExtension under
TestNativeCheckOfficial. They pass in the later complete oracle lane; the ordinary
records remain SKIP. Eleven no-test-file packages in each full lane are not test
skips. Actual test events have zero FAIL, complete RUN/terminal pairing, no extra,
missing, duplicate, malformed or cached-result substitution. Separate early four
normals/CLI remain their own evidence rather than replacing this full run.

The existing fixed Go 1.27.2 distribution has no preinstalled nm. Original gate17
`go tool nm` resolves and builds/reuses the official host cmd/nm through the Go
builtin-tool mechanism before reading the PE. The source/audit record pins 26
inputs against the exact official archive. It does not promise a cache miss or a
zero-compilation/write operation. The outer gates remain serial; `go tool` does
not accept the general -p flag, and its internal bootstrap BuildP defaults to the
approved GOMAXPROCS=2. Do not describe all internal compilation as -p=1. No extra
probe, download, cache reroute, foreign binary execution or new project command
was added. The explicit source scope of the three future native manual commands
is unchanged by this host-tool bootstrap finding.

A recorder accounting detail is preserved: intermediate `*.natural-exit.json`
files were written with initialized `status:FAIL` before post-validation. All 17
actual `exit_code` values are zero and `natural_wait_returned` is true; final
per-command results and command-statuses are PASS, with no STOP. Reconciliation
records that distinction rather than altering the intermediate files. “Zero FAIL”
above describes actual test events, not every intermediate JSON status field.

The sealed terminal source report is `windows-rekey-full-verification-v1/REPORT.md`,
SHA-256 `081965fa462ddcced2503519000eadc43b550fac8498be8bf9eb8af1d705e627`.
Its 277-artifact manifest is SHA-256
`75586d88e525ccf709593108a45f6f1f15bb23b283398bdbc541f92f935f4789`;
`result-reconciliation.json` is
`f166387ab11d6d43be74dccc5359b6a0520ed984a794a2f48358dfd4acb3b498`.
It preserves exact argv/environment, resources, source/tool pins, raw outputs,
complete named sets, closures, binary/symbol data, bootstrap provenance and this
accounting reconciliation. This document reports that code-tree execution; the
later code-plus-documentation composition has not itself run project commands.
No new commit, Library restore, publication or candidate remote CI is claimed.
Native Windows/Darwin, physical failure/durability, mixed binaries and release
acceptance remain NOT_RUN/unqualified.

## What remains unimplemented or unqualified

Native mixed-version scheduling and physical failed lock/close/unmap/Sync/I/O,
wrong-key/tamper/missing-selected-file, namespace interference, short writes,
ENOSPC/torn writes, crash/kill/recovery/interrupted publication/rekey and power loss
remain HOLD/NOT_RUN. Some broader physical acceptance facilities are still
IMPLEMENTATION_MISSING; no nonexistent harness command is provided. Current
native capability and intact ordinary API tests are available for a separately
approved user procedure, not complete acceptance; see
[Chinese section 22](manual-verification-zh.md#22-windows-shared-rekey-原生普通路径独立规程).

Native retained-directory plaintext WAL-tail publication still lacks code.
Unpublished backup R1 work is a separate candidate and does not enable plaintext
backup in this composition. Darwin Unicode inventory, broader SQL/other goal
items and external reviews remain separate. ANN V4, the interrupted 41-target
campaign and unknown interrupted chains retain their actual holds; this bounded
source/docs work and separately authorized ordinary gates do not restart them.
The [restriction record](restriction-provenance-20261011.md) preserves the
separation between missing code, limited authorization and prohibited execution.
