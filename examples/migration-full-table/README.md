# Synthetic whole-table review example

From the repository root, `go run ./examples/migration-full-table` prints the
fixed full-table copy/swap review bundle. Use the pinned Go 1.27.2 toolchain and
offline environment from [the full-table guide](../../docs/migration-full-table.md).

`catalog.json`, `evidence.json` and `options.json` are intentionally synthetic.
They demonstrate complete explicit typed facts, not a live PostgreSQL collector
or measurements. `golden/` holds every deterministic review artifact. Every
mutation SQL line is commented. No database/server is opened and no SQL runs.

The separate transition CLI accepts the same three input files through explicit
`--catalog`, `--options` and `--full-table-evidence` flags. Retained-array and
shadow-column examples remain in `../migration-transition` unchanged.
