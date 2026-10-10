# Extended mathematical vector values

This checkpoint adds pure-memory half, packed-bit and sparse values to
`extension/vector`. Existing `Vector` APIs, dense metrics and GSV v1 bytes are
unchanged. It does **not** add HALFVEC/BIT/SPARSEVEC SQL declarations, SQL
operators/functions, type-aware exact-search entry points, typed ANN indexes,
database lifecycle guarantees, or PostgreSQL wire compatibility. Those require
separate integration checkpoints. `driver.Valuer`/nullable Scanner adapters here
encode and decode independent BLOB values; they do not implement SQL type
enforcement.

The semantic reference is pgvector v0.8.7, commit
`f37c13f68b57d2c3472b2214fbcff699d6d34876`. Production code is independently
written Go using only the standard library. No upstream C code is compiled,
copied into production, linked, or executed. Published test outputs are retained
under `testdata/pgvector-v0.8.7/extended`, with exact source hashes and existing
pgvector license attribution. Tests do not run PostgreSQL or pgvector.

## Public values and explicit conversion

| Value | Construction / text / binary | Bounds |
| --- | --- | --- |
| `HalfVector` | `NewHalf([]float32)`, `NewHalfBits([]uint16)`, `ParseHalf`, `ParseHalfDims`, `DecodeHalf` | 1..16000 components |
| `BitVector` | `NewBit(dims, packed)`, `ParseBit`, `ParseBitDims`, `DecodeBit` | 0..64000 bits in native math |
| `SparseVector` | `NewSparse(dims, []SparseEntry)`, `ParseSparse`, `ParseSparseDims`, `DecodeSparse` | 1..1e9 dimensions, <=16000 input entries/nonzeros |

All have `Valid`, `Dims`, `RequireDims`, `String`, `MarshalText`,
`MarshalBinary`, and `Value`. `HalfVector.Bits`/`Values`, `BitVector.Packed`,
and `SparseVector.Entries` return defensive copies. `SparseVector.NNZ` returns
the stored nonzero count. Private backing slices cannot be mutated through the
public API; completed values are safe to share for concurrent reads.

The zero Go object is invalid for every type, not NULL. A constructed empty
`BitVector` is valid and distinct from the invalid zero object, using a private
initialized flag. All-zero half/sparse vectors are also valid and distinct from
NULL. A future SQL BIT(n) declaration must require 1..64000 independently;
native zero-width values preserve published empty-bit distance fixtures.

`NullHalfVector`, `NullBitVector`, and `NullSparseVector` are mutable nullable
Scanner/Valuer adapters. They accept nil, a textual string, or the correct
versioned binary `[]byte`. Byte slices are never guessed to be text or retained.
Failed scans preserve both fields. A NULL scan resets both fields. A false
`Valid` binds SQL NULL; a true flag with an invalid contained value errors.
Scanner destinations require caller synchronization, like other sql.Scanners.

Conversions are explicit:

- `HalfFromVector(v)` returns the half value, `HalfConversion`, and error. The
  report counts changed binary32 bit patterns after re-widening and records
  maximum absolute and relative errors. Relative error is measured against
  nonzero source components; exact signed zeros have zero error. Underflow to
  half zero can give relative error 1. Overflow returns no partial value/report.
- `HalfVector.Dense()` widens every finite binary16 component exactly.
- `SparseFromVector(v)` retains nonzero binary32 bits and eliminates both signs
  of zero. The sparse representation intentionally does not preserve zero sign.
- `SparseVector.Dense()` checks the existing dense limit (16000) **before**
  allocating output. A larger dimension returns `ErrLimit`, regardless of nnz.

No heterogeneous metric API, arbitrary callback, kind inference from untagged
bytes, or implicit half/dense/sparse lossy conversion is provided. Go operand
types and distinct format identifiers fix representation identity. Invalid
metric strings error instead of enabling unsupported operations.

## Binary16 numerical contract

`NewHalf` rounds finite binary32 inputs to IEEE-754 binary16 using nearest,
ties-to-even. The independent implementation exactly widens binary32 to
binary64, scales by a power of two, and uses `math.RoundToEven`. Binary16
subnormals are multiples of 2^-24. Signed zero survives underflow and every
access/codec/conversion that preserves half representation.

Overflow is decided after rounding. 65504 is the largest finite half, but
65505 and 65519 round to 65504; 65520 rounds to infinity and is rejected. Every
NaN/infinity input or half bit pattern is rejected. Half text uses the pinned
decimal -> binary32 -> binary16 route, reusing dense Parse's bounded decimal
parser. For example `1.0004882812500000001` first rounds to a binary32 midpoint,
then to even half 1; direct decimal-to-half rounding would differ.

All 65536 half patterns are enumerated in tests: 63488 finite representations
must survive exact widening, re-quantization, text and binary round trips; the
remaining 2048 infinity/NaN encodings must fail. Every adjacent positive finite
half midpoint, its binary32 predecessor/successor, and both signs are tested.
A separate fuzzer compares conversion with an independent nearest-neighbor
search over IEEE-754 half values.

## Packed-bit contract

Bits are packed most-significant first. Unused low bits of the last byte must
be zero. Both `NewBit` and decoding reject noncanonical padding rather than
clearing it. Text is exactly ASCII 0/1 without trimming, prefixes, quotes,
PostgreSQL B'...' syntax, silent padding or truncation. Native empty text is a
valid empty value. Different bit widths never compare successfully.

`Hamming` and `Jaccard` are closed `BitMetricKind` identities, with typed
`Distance` and `DistanceContext` methods. `HammingDistance` counts XOR bits.
`JaccardDistance` is 1 - intersection/union, except an empty intersection returns
1. Consequently both all-zero/all-zero and empty/empty return Jaccard distance
1, matching pinned pgvector (unlike some set-similarity conventions).

Metrics visit packed bytes without allocation. Context cancellation is checked
before work, at most every 256 packed bytes, and before publishing the result.
All operand pairs at widths 0..8 are checked against a per-bit reference;
9/513/64000-bit boundaries, padding, empty values and cancellation are covered.

## Sparse contract and deliberate text boundary

Native `SparseEntry.Index` and binary indices are zero-based uint32. Text uses
one-based `{1:1,3:2}/5` notation. `{}/5` is a valid all-zero vector. `NewSparse`
checks input count before copying, validates all indices and finite values,
removes explicit zeros, sorts a defensive copy, and rejects duplicate retained
nonzero indices. Two nonzero entries at one index are rejected even if they sum
to zero. Zero plus nonzero at the same valid index is accepted; zero plus zero
is removed. Construction is O(nnz log nnz); metric work is O(nnz).

Sparse decimals round directly to binary32. Matching the pinned `strtof` range
policy, a mathematically nonzero decimal that rounds to zero is rejected with
`ErrSparseUnderflow` (e.g. +/-1e-46). Nonzero subnormals, including 1e-45, survive.
Explicit decimal +/-0 is allowed and discarded. Half/dense underflow policy is
different and remains unchanged.

One deliberate safety difference is fixed by tests and source inspection:
upstream `sparsevec_in` drops zero values before `CheckIndex`, so `{0:0}/1`,
`{2:0}/1` and `{-1:0}/1` can discard the malformed index. Locally **every input
index is checked even for zero values**, so these fail with `ErrSparseIndex`.
The retained `sparse-edge-policies.json` distinguishes source-inspected edge
behavior from published expected-output evidence. No runtime oracle claim is
made. Both implementations remove zeros before retained duplicate checks.

Text is bounded by `MaxTextBytes` (1 MiB), individual integer/decimal tokens by
`MaxElementTextBytes` (128 bytes), and input entries by 16000, including explicit
zeros. Decimal-only syntax excludes hex, underscores and quoted numbers; only
ASCII whitespace is recognized. These bounded syntax restrictions are narrower
than upstream libc parsing. String formatting follows Go's shortest binary32
round trip, so decimal/exponent formatting thresholds can differ from PostgreSQL.

Sparse decoding never repairs malformed values: entries must already be sorted,
strictly unique, in range, finite, and nonzero. It rejects both signs of zero.
Counts and exact byte lengths are validated before allocation. Dimensions do
not control sparse allocation. Billion-dimensional two-entry tests and
zero-allocation distance assertions fix this boundary.

## Numeric metrics and transforms

`MetricKind.DistanceHalf` / `DistanceHalfContext` and `DistanceSparse` /
`DistanceSparseContext` accept the same four identities as dense: `L2`, `L1`,
`NegativeIP`, `Cosine`. Named helpers are `HalfL2Distance`, `HalfL1Distance`,
`HalfInnerProduct`, `HalfNegativeInnerProduct`, `HalfCosineDistance`, `HalfNorm`,
and their `Sparse...` counterparts. Norm also has a Context variant. Metric
loops check cancellation at most every 256 half components or sparse support
positions; nil contexts error. No partial distances escape on cancellation.

L2/L1 merge sparse supports, IP multiplies matching values (implicit zeros
contribute zero), and cosine computes compensated dot/norm sums in one merge.
All numeric math widens before arithmetic and uses the existing Neumaier
compensation policy. Finite inputs stay finite in binary64. Cosine is clamped
to [0,2], with `ErrZeroNorm` for any zero norm. These are intentional differences
from upstream float32 accumulation/FMA, overflow, underflow, and undefined NaN
ordering. Retained fixtures explicitly distinguish zero-norm cosine NaN,
nonzero overflow-cosine NaN, and sparse L1/IP overflow; none is accepted by
widening a tolerance or relabeling it a matching result.

`Subvector(Vector,start,count)` and `HalfSubvector(HalfVector,start,count)`
preserve representation. Signed int32 start/count match the reference domain.
Count must be positive. The exclusive end is calculated from the original
start before clamping start to 1; int64 arithmetic avoids overflow. Ends past
dimension+1 truncate; empty results error. In particular (-1,3) of [1,2,3,4,5]
is [1], (-1,2) errors, and (-2147483644,2147483647) is [1,2].

`BinaryQuantize` / `HalfBinaryQuantize` emit one bit iff a component is >0.
Negative values and both signed zeros become 0; width equals input dimensions.
A positive dense value rounded to half zero can quantize differently after
half conversion. Sparse quantization is not implicitly supported.

The runnable `ExampleBinaryQuantize` uses a single immutable memory snapshot,
filters before candidate scoring, selects by exact Hamming distance, and
reranks original dense vectors by L2 with deterministic rowid ties. Candidate
and final metrics are separately printed. Candidate truncation can lose true
dense neighbors: exact reranking does not establish exact recall. No generated
column maintenance, expression index or unsupported SQL is implied.

## Canonical project BLOB formats

All fields are big-endian. Common bytes 0..3 are magic; byte 4 is version 1;
byte 5 is representation; byte 6 is byte-order 1; byte 7 is reserved and zero;
bytes 8..11 are uint32 dimensions. Unknown versions/representations/order,
nonzero flags, impossible dimensions/counts, truncated payloads and trailing
bytes are rejected. Each decoder accepts only its own complete header.

| Format | Representation byte | Remaining payload | Maximum bytes |
| --- | --- | --- | --- |
| GSV\0, unchanged | 1 | dimension binary32 words after 12-byte header | 64012 |
| GSH\0 | 2 | dimension binary16 words after 12-byte header | 32012 |
| GSB\0 | 3 | ceil(dimension/8) packed bytes after 12-byte header | 8012 |
| GSS\0 | 4 | uint32 nnz at 12..15, then nnz interleaved (uint32 index, binary32 value) pairs | 128016 |

No value encoding contains a checksum or authentication. These are project
storage formats, not PostgreSQL binary COPY/wire/internal representations.

## Verification scope

All new tests and fuzzers are bounded in-memory math/text/codec checks. They
include source/returned-slice mutation, borrowed scan-buffer reuse, failure
state preservation, cross-format rejection, context cancellation, randomized
sparse-vs-dense comparisons, extreme magnitudes, subnormals and signed zeros.
Published fixtures cover 88 parse cases, 93 metric cases, 6 quantizations, and
20 subvectors. Source hashes are checked by tests.

Focused commands (using the pinned Go toolchain in this checkout):

```sh
GOTOOLCHAIN=local GOCACHE=/tmp/gosqlite-m9-vector-cache CGO_ENABLED=0 ../toolchains/go1.27.2/bin/go test ./extension/vector -count=3
GOTOOLCHAIN=local GOCACHE=/tmp/gosqlite-m9-vector-cache CGO_ENABLED=0 ../toolchains/go1.27.2/bin/go vet ./extension/vector
GOTOOLCHAIN=local GOCACHE=/tmp/gosqlite-m9-vector-cache CGO_ENABLED=0 ../toolchains/go1.27.2/bin/go test ./extension/vector -run '^$' -fuzz '^FuzzHalfRounding$' -fuzztime=5s -parallel=2
```

This checkpoint supplies no physical database crash, tamper, fault-injection,
forced-exit or recovery evidence. Aggregate project evidence and any subsequent
SQL/driver/ANN integration are owned by their separate checkpoints.
