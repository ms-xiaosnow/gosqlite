# Offline pg_embedding SQL converter (M9 B)

Checkpoint B adds a real bounded PostgreSQL scanner, source-span AST, typed
catalog binder and two explicit SQL targets. This is a SQL conversion tool,
not the checkpoint C CSV/NDJSON importer or checkpoint D PostgreSQL migration
bundle. No PostgreSQL connection, remote SQL execution, production SQL delegation,
global string replacement, extension transition or automatic application deployment
exists in this path. M3/M8 restricted qualification and release gates remain open.

## Run it

From the repository root, using the project's verified Go toolchain:

```sh
CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding convert \
  --catalog examples/migrate-pgembedding/converter-catalog.json \
  --input examples/migrate-pgembedding/source.sql \
  --target local --table 30000=documents

CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding convert \
  --catalog examples/migrate-pgembedding/converter-catalog.json \
  --input examples/migrate-pgembedding/source.sql \
  --target local --table 30000=documents --format sql

CGO_ENABLED=0 go run ./cmd/gosqlite migrate pgembedding convert \
  --catalog examples/migrate-pgembedding/converter-catalog.json \
  --input examples/migrate-pgembedding/source.sql \
  --target postgres --table 30000=documents --target-schema public \
  --vector-schema public --format sql

CGO_ENABLED=0 go run ./examples/migrate-pgembedding
```

The CLI reads a catalog and SQL file; it never opens a database. `--report NEW`
optionally creates a new mode-0600 report/SQL artifact with exclusive creation,
refusing existing paths. Unsupported flags, duplicate flags, credentials, DSNs,
`--apply`, `--resume` and `--output` are rejected. The default JSON report includes
original evidence, per-statement findings, edits, bind transformations and query
settings. A statement that cannot be converted has `status=unsupported`, exact
original evidence and source spans, **no converted SQL and no edits**. JSON mode
reports all statements then returns a nonzero error if any were unsupported.
`--format sql` refuses the entire script instead of mixing unsupported source
into executable output. No partial SQL is written.

The example is different from the CLI: it executes only the embedded synthetic
fixture against a new temporary local database. It builds a real HNSW index,
closes/reopens normally, and verifies Manhattan distances 0, 5, 6 followed by the
NULL row. Tests separately compare L2, cosine and L1 against independent scalar
math with `abs(got-want) <= 1e-12*max(1,abs(want))` on this bounded fixture.
This tolerance is not a universal bound or evidence of legacy float32 bit identity.

## Native interface

`migration/postgres.Convert(ctx, source, catalog, ConvertOptions)` returns a
`Conversion`. `TargetLocal` or `TargetPostgres` is mandatory. Each referenced
relation requires an explicit `TableMapping{RelationOID, TargetName, TargetSchema}`;
local schema must be empty, PostgreSQL schema must be supplied. `VectorSchema`
is required for PostgreSQL text and identifies the intended pgvector namespace;
it does not establish installation, privileges or extension version.

`StandardConformingStrings: true` is an explicit source contract. The converter
supports ordinary strings with PostgreSQL's standard-conforming semantics, not
legacy backslash interpretation. Optional source/target profiles must match the
existing pinned profiles. Selected columns/dimensions come from verified legacy
catalog inspection, optionally augmented by `Selected []Selection`; a blocked
or review-required selected column is not made usable by the converter.

`Conversion.SQL()` succeeds only if every statement converted. Source and target
SHA-256 values hash exact source bytes and concatenated converted statement bytes.
`catalog_sha256` identifies the inspected canonical catalog. CLI SQL-mode warning
headers are presentation text outside the target hash. Hashes establish identity,
not authenticity, installation evidence or migration approval. Reports contain
source SQL intentionally, but errors do not echo literal values or identifiers.

A parameter used in a proven selected real-array context yields
`BindTransform{Action: "real_array_to_vector", Dimensions: n}`. Call its
`ConvertValue` for every runtime value: it accepts NULL, PostgreSQL array text or
`[]float32`, validates finite binary32 dimensions, then returns local GSV v1 bytes
or PostgreSQL vector text. True signed zeros and representable subnormals are retained. Unlike the public
checkpoint-A ParseRealArray dense-value policy, B rejects nonzero source decimal
text that underflows to zero, preserving PostgreSQL float4 input errors; already
valid []float32 zero values remain valid. It never
accepts `[]float64` by guessing precision. Scalar parameters have typed `identity`
transformations with reported NOT NULL, integer-width and nonnegative-LIMIT
constraints where applicable. Apply `ConvertValue` to every reported parameter,
including identity binds. Untyped parameters are refused. Scalar text requires
valid UTF-8 without NUL, booleans require bool, and real parameters require finite
source-width values (float4 uses float32; float8 accepts float32/float64).
Bypassing the adapter can lose source constraints, including
PostgreSQL primary-key NULL rejection versus local rowid allocation. Local `$n` becomes `?n`; PostgreSQL vector parameters get an
explicit schema-qualified vector cast. Conflicting repeated scalar/vector/dimension
uses block the statement. Local execution still needs the application's bound
values; SQL containing parameters is not a standalone populated script.

## Supported and refused forms

| Form | Implemented boundary |
|---|---|
| CREATE TABLE | Catalog-matching complete column list, ordinary integer/real/text and selected real[]; inline one-column PK/NOT NULL and catalog-matching supported defaults. Selected real[] becomes VECTOR(n). |
| INSERT | Explicit column list and constant VALUES tuples; required source PK and no-default NOT NULL columns must be present. Vector literals/casts/binds are typed and validated. |
| UPDATE | Single table, assignments and optional scalar WHERE; local aliases are refused. |
| SELECT | Explicit scalar/column projections, one table/optional alias, optional WHERE, ORDER BY and LIMIT; supported typed distances and scalar operators. |
| Arrays | Constant one-dimensional ARRAY decimals or real-array strings in positively bound vector contexts, including ::real[] and CAST(... AS real[]). No inference from merely mentioning embedding elsewhere. |
| Functions | Verified extension-owned two-real[] signatures of l2_distance, cosine_distance, manhattan_distance. Shadows/ambiguity block; Manhattan maps to l1_distance. |
| HNSW CREATE INDEX | One selected plain column; verified legacy method/handler/opclass/operator/function identity; dims/m/efconstruction/efsearch integer options only. Unknown, duplicate or invalid options block. |
| Modern bit distance | Two bound fixed-width bit columns plus a verified pgvector-0.8.7 bit-operator/function signature preserve Hamming `<~>` and Jaccard `<%>`. They are never legacy Manhattan rewrites. Exact-width ordinary string assignments use the canonical bit constructor; bit runtime parameters are unsupported. Local BIT support is supplied by the separate typed-SQL checkpoint. |
| Other PostgreSQL | Joins, CTEs, subqueries, aggregates, procedural/dynamic SQL, star projection, expression/partial indexes, arbitrary functions/casts, array slices/nonconstant/multidimensional arrays, serial/identity/generated columns, unknown opclasses and schema mutations are unsupported. |

The scalar subset is deliberately narrower than PostgreSQL. General array
columns remain unchanged for PostgreSQL selection, and are unsupported locally.
Vector comparisons/orderings are not approximated by BLOB comparisons. Order
ordinals and unknown ordering types are refused. Local text comparison/ordering
is also refused because this catalog profile does not establish a source
collation contract. Case-insensitive local alias/column collisions and duplicate explicit/implicit
output labels are blocked. Unaliased supported expression/function projections
receive explicit source-result labels, so function renames do not silently change
column metadata. Give distinct explicit aliases when necessary.
LIMIT accepts only nonnegative integer literals or non-NULL runtime-constrained binds (PostgreSQL LIMIT NULL is outside this narrower contract);
negative values and arbitrary limit arithmetic are rejected. Narrow source integer types map
to local signed-64-bit INTEGER and source scalar real to local float64 REAL;
findings flag the wider local storage/write ranges. Context-bound scalar
floating assignments, including integer-spelled constants, are rounded to the source width before local emission. Mixed numeric comparisons, dynamic integer-to-real or float8-to-float4 assignments, and
untyped arbitrary-precision decimal scalar expressions are unsupported. Source sequence/identity behavior
is never approximated by local automatic rowid allocation. Local binary scalar arithmetic and nonliteral numeric unary operations are
refused because PostgreSQL overflow/division/error behavior is not emulated.
Primary-key writes accept only explicit integer literals or constrained direct
parameters, preventing expression-derived NULL from allocating a local rowid.
PostgreSQL scalar compatibility beyond these bounds is not claimed. Shift operations are limited to constant counts 0..31
and an int8 operand or a checked constant without PostgreSQL int4 overflow.

Unqualified names must resolve uniquely in the supplied catalog. This is not
live `search_path` discovery or a proof that the inventory contains every possible
shadow object. Review and freeze the source catalog/application namespace before
using output. The current converter keeps selected column names; creation/query
snippets are not a shadow-column backfill/cutover plan.

## Operator, NULL and setting semantics

Only a positively bound legacy-vector operator with verified extension ownership
and real[] signature is rewritten. Legacy Manhattan `<~>` becomes dense `<+>`;
L2 `<->` and cosine `<=>` retain their metrics. PostgreSQL output qualifies target
operators with `OPERATOR(schema.op)` and target vector types/functions/opclasses
with the explicit pgvector schema. Unknown overloaded identities block output. SQL keyword type aliases are
recognized only unquoted and unqualified; quoted/qualified type names resolve
actual catalog typnames, and unqualified named-type shadows are rejected.

PostgreSQL ASC defaults to NULLS LAST and DESC to NULLS FIRST, opposite to local
SQLite ordering. For nullable sort expressions, the converter adds the needed
`(expression) IS NULL ASC|DESC` key, preserves remaining tie keys and never drops
NULL rows. Output aliases use the underlying converted expression for that added
key. All explicit/default direction/NULL combinations are executed in tests.
The report warns that this extra key can disable narrow ANN/top-k recognition
and use the exact general sorter. No ANN performance or recall equivalence is
claimed by SQL syntax conversion.

Local metric arithmetic uses compensated float64; legacy functions return
float32 with architecture-sensitive accumulation. Local zero-norm cosine errors
rather than silently matching an upstream NaN. Corresponding findings remain
visible. HNSW target constraints are enforced independently: dense dimension
<=2000, m=2..100, ef_construction=4..1000 and >=2*m, query ef=1..1000. Neither
legacy default100/16 nor the historical example3/5 is automatically retuned.

`efsearch` is emitted only as a separate `QuerySetting` artifact:

- Local: set `sqlite.VectorSearchOptions.EFSearch` in native application calls.
  Local SQL has no equivalent per-query setting; no fake SET statement is emitted.
- PostgreSQL: review `SET LOCAL hnsw.ef_search = n;` inside the application's
  existing transaction. The converter never inserts BEGIN/COMMIT wrappers.

An application must decide which query/index each setting belongs to. Converting
an index does not secretly change later query options. Legacy
`SET enable_seqscan=off` is an unsupported testing hint, not a required production
migration setting.

## Bounds, source spans and trivia

The independent migration scanner uses UTF-8 PostgreSQL-15 lexical forms and
current compatible precedence for its accepted subset. It recognizes quoted
identifiers, doubled ordinary quotes, dollar strings, E/bit/Unicode opaque forms,
parameters, casts, arrays, maximal-munch operators and nested block comments.
Ordinary identifier folding is ASCII-only; >63-byte names are refused rather than
silently truncated. It never scans SQL inside opaque bodies. Psql commands, COPY
sections, malformed/unclosed streams, NUL and invalid UTF-8 fail before conversion.

Defaults: 1 MiB source/literal bytes, 32,768 tokens including trivia, nesting/AST
height 64 and 256 nonempty statements. Complete custom limits can only lower
these maxima. Target edits are capped at 4 MiB per statement. Context is checked
before work and between bounded statements. Repeated nested AST serialization is
bounded by the depth/token limits. These are work caps, not a streaming parser.

Edits are nonoverlapping absolute half-open byte spans. Outside edited nodes,
source bytes are unchanged. Within expression rewrites, child spans and original
gaps retain original literals/comments. Compound ARRAY/type/name/index rewrites
emit new significant tokens then retain every original trivia byte once, in its
original relative order. NULL-sort duplicate keys omit copied comments, so original
comments still occur once. Parentheses preserve the parsed source operator tree
where target precedence differs. There is no whole-file search/replace.

Opaque literals in executable expressions require manual review. Nested block
comments are safely tokenized, but are explicitly unsupported for local output.
Bare-CR line-comment terminators are also blocked because PostgreSQL and the
local lexer end those comments differently; CRLF remains supported. A local
parse success alone is not treated as evidence of equivalent comment semantics. Extra boundary
semicolons around a real statement may similarly fail local parser validation;
trivia/empty-only inputs are retained without executing anything. These limits
are not worked around by dropping comments or unknown syntax.

## Reproducible references and evidence

The original source pins, licenses and manifests remain in
`testdata/pgembedding-0.3.6` and `testdata/pgvector-v0.8.7`. Converter code is
independently written. New catalog/query fixtures are explicitly synthetic,
not captured PostgreSQL execution. Exact local SQL and deterministic JSON goldens
are in `migration/postgres/testdata/converter`.

Primary syntax/identity references checked during this implementation:

- [PostgreSQL lexical structure and precedence](https://www.postgresql.org/docs/18/sql-syntax-lexical.html)
- [PostgreSQL result-column naming source](https://github.com/postgres/postgres/blob/REL_18_STABLE/src/backend/parser/parse_target.c)
- [PostgreSQL float4 input source](https://github.com/postgres/postgres/blob/REL_18_STABLE/src/backend/utils/adt/float.c)
- [PostgreSQL NULL ordering](https://www.postgresql.org/docs/18/queries-order.html)
- [Pinned pg_embedding definitions](https://github.com/neondatabase/pg_embedding/blob/5d48508aeaeb86b9e9c630468ada7a124b905795/embedding--0.3.6.sql)
- [Pinned pgvector vector/bit definitions](https://github.com/pgvector/pgvector/blob/f37c13f68b57d2c3472b2214fbcff699d6d34876/sql/vector.sql)

PostgreSQL output is source-reviewed text only. Local parser acceptance, local
execution, independent exact-distance comparisons, normal reopen, pure-memory
fuzz and race tests are distinct evidence categories. No live PostgreSQL, physical
corruption/tamper, wrong-key, process-crash/forced-exit, disk-fault or recovery test
is part of this checkpoint. Integration evidence records exact reviewed commands.
