# pg_embedding 0.3.6 source-reference fixtures

The four upstream files in this directory are byte-for-byte copies from
[neondatabase/pg_embedding, commit 5d48508](https://github.com/neondatabase/pg_embedding/tree/5d48508aeaeb86b9e9c630468ada7a124b905795),
dated 2023-09-28. The extension is named `embedding`; its control file declares
version `0.3.6`. The source was retrieved during research on 2026-10-10 and
copied from that reference cache without further downloads. `manifest.json`
records locally computed SHA-256 values, file lengths, and pinned source URLs.
These hashes are integrity records, not upstream-published signatures.

## What is captured

- `embedding--0.3.6.sql`: upstream installation declarations for the `real[]`
  distance operators, `hnsw` access method, and legacy operator classes.
- `embedding.control`: upstream extension identity and version declaration.
- `LICENSE` and `NOTICE`: exact upstream Apache-2.0 license and notices.
  The notice also preserves MIT attribution for upstream's derived HNSW code.

The SQL is a source-reference fixture. It was not executed, and these fixtures
are not an executable migration or a live PostgreSQL catalog export. No upstream
C/C++ implementation is bundled or copied into production Go packages.

Any adjacent `catalog.json`, query examples, or data fixtures are synthetic,
locally authored test inputs unless they have their own explicit provenance.
They are not captured PostgreSQL results, assigned production OIDs, or upstream
files. This manifest's upstream-file hashes cover only the four files above;
the README and manifest are locally authored documentation.

## Migration limits

No live PostgreSQL or Neon installation, query, catalog extraction, or migration
was run to establish this source profile. Local tests cannot establish remote
server capabilities, live query plans, ANN recall, or complete compatibility.
The source distance functions return PostgreSQL `real`; local float64 distance
calculations need not reproduce their output bits.

The current [Neon pgvector page](https://neon.com/docs/extensions/pgvector#migrate-from-pg_embedding-to-pgvector)
was retrieved on 2026-10-10, but its requested migration chapter was absent.
The official historical chapter is available in the
[2024-01-23 website snapshot](https://github.com/neondatabase/website/blob/d4c1cf37f79acbf4c02880eddb97dd9a36cb2d20/content/docs/extensions/pg_embedding.md#migrate-from-pg_embedding-to-pgvector).
Its destructive examples are historical guidance, not authorization to run
`DROP ... CASCADE` or a table drop.

The pinned [legacy options source](https://github.com/neondatabase/pg_embedding/blob/5d48508aeaeb86b9e9c630468ada7a124b905795/embedding.c)
defaults to `m=100`, `efconstruction=16`, and `efsearch=64`. The
[pinned README example](https://github.com/neondatabase/pg_embedding/blob/5d48508aeaeb86b9e9c630468ada7a124b905795/README.md)
uses `m=3` and `efconstruction=5`. Both pairs violate the modern pgvector HNSW
constraint `ef_construction >= 2*m` after renaming the option. They must produce
a `target_option_constraint` finding rather than silent clamping or retuning.
Modern constraints here refer to the project's pinned pgvector v0.8.7 reference;
verify the installed target's version and capabilities separately.

Both legacy pg_embedding and modern pgvector declare the database-wide `hnsw`
access method. Moving schemas does not avoid that name collision. Review
extension dependencies and a rollback plan before any remote transition.
