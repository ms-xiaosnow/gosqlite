# ADR 0001: Native core and evidence boundaries

Accepted 2026-10-10. Module: github.com/ms-xiaosnow/gosqlite, matching the user-owned private repository.

Implement modular native Go algorithms from public behavior specifications. Official SQLite is an isolated test oracle; no production imports, subprocess delegation, CGO, C ABI, or remote database execution. The original goal remains byte-identical.

M0–M3 are the foundation. Later independent implementation may proceed when a platform-restricted verification gate blocks, per the user's later instruction; this does not satisfy that gate or authorize a workaround. Any milestone with unmet core acceptance remains IN_PROGRESS/BLOCKED. Reports separate PASSED, FAILED, BLOCKED and NOT_RUN.

Durability: preserve exact Git history by reading ordinary Git objects through the authorized GitHub app. Commit trees are uploaded with a guarded fast-forward ref update, read back, and mirrored exactly. Full-history bundles are verified, restored and saved to Library at each coherent checkpoint. Never accumulate large unsaved changes if either path fails.
