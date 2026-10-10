# Owner verification records

`initial/` preserves the first candidate's historical logs, including failed draft
checks and its later scoped passing suites; independent review subsequently found
real implementation defects. `corrected/` records the repaired33-path candidate.
No earlier PASS is promoted to qualification of corrected bytes or live PG16.
The independent review and final lead integration are recorded separately in
../m9-full-table-independent-review.md and ../m9-full-table-lead-checks.txt.

These folders preserve original text/JSON evidence, not duplicate tarballs. Their
original evidence inventories may name local overlay archives that are not stored
here. Corrected source bytes are the committed paths in owned-paths.txt and can
be checked against source-sha256.txt from the repository root. Historical local
workspace paths identify test provenance, not a substitute for the remote commit
and separately verified full-history Library bundle recorded in checkpoints.json.
