# Generate review-only PostgreSQL transition artifacts

Run from the repository root with pinned Go 1.27.2 and the established offline
module/cache environment:

```sh
CGO_ENABLED=0 ../toolchains/go1.27.2/bin/go run ./examples/migration-transition \
  -approach shadow -mode batches
CGO_ENABLED=0 ../toolchains/go1.27.2/bin/go run ./examples/migration-transition \
  -approach retain -mode single -out-dir /tmp/new-review-directory
```

The first command prints a JSON envelope of named text artifacts. The second
creates an exclusively new directory with the artifact files. Existing
directories/files are not overwritten. A write failure can leave a partial
review directory; check the returned error before using any output.

This is a synthetic, executable Go example using explicit OIDs and choices from
`testdata/pgembedding-0.3.6/catalog.json`; no PostgreSQL SQL is executed and no
network/database connection is made. For real catalog inputs, call the library
with explicit reviewed choices as documented in
[`docs/migration-transition.md`](../../docs/migration-transition.md).

`golden/shadow-batches/` and `golden/retain-single/` contain deterministic
review-only bundles. Tests compare generated SQL phase bytes with these files.
Every phase except read-only preflight is fully commented. The typed copy means
one nullable shadow column; historical full-table copy is unsupported. The
manifest hashes other files, not itself. Neither synthetic coverage claims nor
generated approvals prove actual server readiness or authorize execution.
