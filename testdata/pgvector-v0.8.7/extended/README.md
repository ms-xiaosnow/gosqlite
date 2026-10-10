# Extended pgvector v0.8.7 fixtures

Pinned commit: `f37c13f68b57d2c3472b2214fbcff699d6d34876`.
Source: https://github.com/pgvector/pgvector/tree/f37c13f68b57d2c3472b2214fbcff699d6d34876

The six halfvec/bit/sparsevec `.sql` and `.out` files are retained verbatim from
upstream `test/sql` and `test/expected`. The manifest records byte counts,
SHA-256 hashes, source locations, and the separately inspected public C-source
hashes. See `../../../third_party/pgvector-LICENSE` for retained fixture licensing.
The C reference files are not copied into production or executed.

`golden.json` extracts 88 parse, 93 metric, 6 quantization and 20 subvector
cases. Dense transformation cases point to the already retained parent
`vector_type.sql` / `vector_type.out`; every extracted statement is checked for
presence in its matching SQL and expected-output file. Text output is compared
after representation-preserving parsing to allow formatter threshold differences.
Metric tolerance on ordinary published examples is 1e-6 + 1e-5*abs(expected).
This is a fixture-specific tolerance, not a universal arithmetic error bound.

Upstream sparse float32 inner-product/L1 overflow and nonzero overflow-cosine
NaN are **explicit semantic differences** with separate exact local assertions.
Zero-norm cosine separately requires local `ErrZeroNorm`. They are not loose
tolerance matches. Other differences and the independent binary formats are
documented in `extension/vector/extended-values.md` at the repository root.

`sparse-edge-policies.json` records source-inspected zero/duplicate/index and
decimal-underflow rules. It labels source-inspection evidence distinctly from
published expected outputs. The local parser rejects invalid zero indices,
where the upstream parser discards the zero entry before checking its index.

Evidence is public source plus published outputs and independent Go math only.
No PostgreSQL or pgvector runtime was executed. SQL in these reference files is
not a claim that the local SQL engine supports the referenced syntax or types.
