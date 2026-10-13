# Source-only extraction and finite validation scope

## Source identities

The target module is gosqlite at base commit
`40e287dc50f8c282f2003ca16475af5eb73a9400`. This overlay adds new directories;
it does not replace the existing native `benchmarks/ann/main.go`.

Reference extraction: archived V3 `benchmarks/ann` config/report/wire values,
pure seed/GSV fixture function and selected fixture-only tests. No archived
main, ownership, boundary test or mixed workload runner file is copied.

The final three-defect input is pinned before the package-name transformation:

- validate.go: `6716cf502e8502eb8dfc70a9ad802d431de5acb2372463f19cb717db08625497`
- regression_test.go: `1aebc0556726543096c489364899d913715ae295e64cdfdb7da44088f0fc62cd`

These fixes add HNSW traversal/read/rescore conservation, IVF all-lists source
coverage, and exact/ANN same-row final-distance equality. Earlier FAIL/source
review records remain historical evidence for their exact earlier trees; this
source extraction does not rewrite or supersede them with an executed PASS.

## Shipping files and extraction changes

- config.go: Config/Admission, bounded arithmetic, default option values and
  pure normalized algorithm configuration. `nativeOptions` constructs an
  Options value only; it never opens a database.
- report.go: report/observation/sample/hardware value types, digests and summary
  arithmetic. Removed `observedHardware`, `buildSettings`, `retainedOwners` and
  `retainedDiagnostics`. Ownership is only three reported integer counts.
- wire.go: strict JSON/schema admission and fixed policy/limit values.
  `SchemaVersion` and `MaxReportBytes` are exported constants; maxErrorNodes=128
  is retained as a wire-policy value, with no diagnostic/owner traversal code.
- validate.go: ValidatePass and value-only helper predicates plus the three
  fixes, with the same schema acceptance policy. The two RFC3339Nano parses
  use ParseInLocation(..., time.UTC), preserving the existing Z-only acceptance
  rule while preventing numeric-offset rejected inputs from initializing
  time.Local and reading TZ/local zoneinfo files. The dedicated offset controls
  do not mutate time.Local or environment variables.
- fixture.go: only generateFixture's local PRNG / float32 / GSV / SHA-256 loop.
  The validation caller uses retain=false; selected compatibility tests may
  use retain=true. No native index constructor/search runs in either branch.
- config/report/wire/regression tests: in-memory values and embedded fixtures.
  Old argument-parser tests are excluded; time.Now use in a malformed-date
  test is replaced by a fixed malformed date. Ownership/boundary tests, fuzz
  targets, benchmarks and TestMain are excluded.
- cmd/ann-report-check/main.go: stdin reader with 256 MiB+1 bound, strict decode,
  JSON stdout writer and natural exit status. No file path argument, file open,
  subprocess, network or output-file path is supported.
- cmd/ann-report-check/main_test.go: byte readers/buffers, small bounded-read
  control, synthetic I/O error values and the embedded complete synthetic JSON.

## Actual application call closure

`DecodeReport` -> token/UTF-8/byte/node/depth checks -> requiredFields/wireFields
reflection -> strict typed JSON decode -> `Report.ValidatePass`.

`ValidatePass` -> Config.admission; canonicalAlgorithm/canonicalBytes;
queryPolicy/nativeLimits/timingPolicy; validateProvenance; deterministic
generateFixture(false); workload/hardware/build digests; timestamp and path
syntax parsing; validateObservation; summarizeSample and checked arithmetic.

Cross-package application calls are limited to:

- ann.DefaultConfig, Config.Normalize, Config.Validate, Config.Canonical,
  ann.ParseConfig and their method-envelope/Opclass value-only helpers;
- vector.New -> checkDimensions/finite32, Vector.MarshalBinary -> Valid/Dims;
- vector.DefaultSearchLimits and sqlite.DefaultANNWorkLimits (value literals);
- pager.UndoLimit (integer arithmetic over an Options value).

sqlite.VectorResult/VectorStats/VectorPlan/QueryPlan/QueryStats and vector.Hit
are reused as values, including reflection/JSON of their exported fields. There
is no call to Open, Begin, Check, VectorSearch, Commit, Rollback, Close, SQL,
native HNSW/IVF build/search, owner retirement or arbitrary input callbacks.
`retainedPathsValid` only normalizes strings using the report's GOOS; it never
reads any report-named path.

The test-only modelMethodReport helper reads compile-time embedded JSON into
memory and assigns synthetic metadata; it calls only the same pure helpers.
The complete CLI fixture uses a source-only mechanical JSON/SHA transformation,
with its own provenance note. Neither one is a source of new measurements.

## Import and initialization boundary

This is not a standard-library-only package. sqlite transitively imports native
storage-containing project packages including pager/vfs, encryption and grams13;
external dependencies include x/crypto, x/sys and x/text. Retaining public API
value types deliberately avoids maintaining a divergent duplicate API.

Independent source inspection for the pinned Linux/amd64 closure found no
project func init in those imports. Project variable initialization constructs
errors, empty registries/maps/channel, numeric values and SQL builtin closures.
External initialization includes x/crypto hash constructors and CPU flags,
x/sys/cpu CPUID/XGETBV and GODEBUG parsing, and x/text static tables. This is not
a statement that the whole process/runtime/test harness has no descriptor use,
no CPU feature observation or no environment access. It is not cross-platform
initialization qualification. The direct report-processing functions do not
sample process or hardware state.

Toolchain build/test activity reads source, module/cache and go:embed files and
may write build caches/binaries. Those toolchain operations are distinct from
the application call closure and require the separately bounded validation
plan. The command's normal stdin/stdout/stderr descriptors are supplied by its
caller; it opens no report-named files.

## Proposed finite validation, not executed evidence

`TEST-SELECTORS.txt` lists the exact proposed root test names by package. There
is no wildcard ANN native harness command, no ./... test run, no performance CI
job, no native producer or benchmark invocation, and no fuzz target in this
slice. An anchored union of these names is the intended test selector.

At this source checkpoint Go, gofmt, compilation, tests, fixture constructors,
validator execution, race, process CLI execution and native work are NOT_RUN.
Tests or documentation describing an expected outcome are not results. A later
validation receipt must bind the final exact files, commands and natural exits.

## 后续有限验证

2026-10-13 06:11:06 UTC六门通过；上文NOT_RUN是初次源码审查时点。
[精确结果](../../../docs/evidence/20261013-ann-report-check/README.zh.md)。
