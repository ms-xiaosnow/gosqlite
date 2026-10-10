# pgvector v0.8.7 published fixtures

Pinned official tag commit: `f37c13f68b57d2c3472b2214fbcff699d6d34876`.
Retrieved 2026-10-10 from https://github.com/pgvector/pgvector.

- `vector_type.sql` and `vector_type.out` are retained verbatim from
  `test/sql/vector_type.sql` and `test/expected/vector_type.out` at that commit.
- `golden.json` extracts supported parse/metric examples and their published
  expected results. This does not imply execution of PostgreSQL or pgvector.
- `manifest.json` records official source paths, hashes and intentional
  compatibility boundaries. Retained fixture licensing is in
  `third_party/pgvector-LICENSE` at the repository root.
- Production `extension/vector` code is independently written pure Go; no
  upstream C implementation is compiled, linked, or invoked.

The tests cover the selected public dense primitives, not every operation in
upstream's full fixture. Vector arithmetic operators, normalization, aggregates,
subvectors, quantization, and PostgreSQL type/SQL parsing are not implemented by
this package. ANNs, database transactions and persistence are separate layers.

## Deliberate numeric and representation differences

The four dense distances and norm widen input components to float64 and use
Neumaier-compensated accumulation. Unlike upstream float32 metric accumulation,
valid float32 inputs remain finite even at the extrema or for subnormals.
Architecture-dependent upstream FMA/rounding is not reproduced. Ordinary
published metric examples are checked with explicit tolerances, while upstream
Infinity/NaN extreme cases are checked against independent float64 mathematics.
Zero-norm cosine returns `ErrZeroNorm`; an exact scan fails if such a candidate
passes the filter. It never silently drops an undefined cosine result.

Type dimensions are 1..16000, separate from the recorded upstream dense ANN
limit of 2000. The parser accepts signed decimals, decimal points/exponents and
ASCII whitespace, rounds directly to float32, rejects non-finite/overflow values,
and accepts underflow to signed zero. It rejects hexadecimal forms, underscores,
quoted elements, Unicode whitespace, input over 1 MiB and tokens over 128 bytes.
Those explicit resource/syntax restrictions are narrower than upstream strtof.

The BLOB is an independent versioned big-endian format with exact float32-bit
round trips, including signed zero and subnormals. It is not PostgreSQL binary
COPY/wire format. Text formatting uses Go shortest float32 decimal strings;
formatting thresholds can differ from PostgreSQL without changing bits.

The expected-output fixture retains upstream aligned trailing spaces byte-for-byte; a path-specific .gitattributes whitespace exemption preserves its published-content hash.
