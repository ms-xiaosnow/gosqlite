# Grouped aggregate v1 review: changes required

Exact candidate: `dffdc169ee2f0f032fe10852baf8420f6530aec8`, sibling `grouped-aggregate-freeze-v1`. Baseline: `8d7a0fcc5a2e3a83850ea780b64b403047677568`. Verified all 1,251 candidate and 1,225 baseline hashes, rebuilt exact Git trees in reviewer-owned isolated copies, verified 37 owned paths and zero removals, and applied the full base-to-candidate patch to a fresh baseline copy with every resulting hash equal. Patch SHA256: `71c1bcc1d16e306120cc023c038ee350fe2adf89a49060257a9ab2ba27dba5e9`.

## Findings

1. P2, general ORDER BY/LIMIT projection timing. The existing general materializer evaluates scalar projections for every group, including later groups rejected by SQLite's top-K sorter before projection. On `re(k,v)` containing `(1,1),(2,minint),(3,3)`, `SELECT k,CASE WHEN k=2 THEN abs(-9223372036854775808) ELSE count(*) END FROM re GROUP BY k ORDER BY k+0 LIMIT 1` returns `(1,1)` officially but native Query returns integer overflow. The independent 508-case differential matrix has exactly nine failures, all this family; remaining 499 cases pass. Exact failures also cover DESC, explicit NULL placement, compound ordering, and OFFSET. Preserved source and log: `grouped_reviewer_oracle_test.go.txt`, `probes-v1.txt/.exit`.

2. P2, physical-rowid grouping stream with LIMIT. The grouped source unconditionally consumes every input row. On `rp(id INTEGER PRIMARY KEY,v)` containing `(1,1),(2,2),(3,minint),(4,4)`, `SELECT id,count(*) FROM rp WHERE abs(v)>0 GROUP BY id LIMIT 1` returns `(1,1)` officially but native Next reports overflow from the later third row. The same mismatch is reproduced with NOT INDEXED, a CASE predicate, and explicit `ORDER BY id`. DESC is a genuine overflow negative in both engines. This is physical rowid grouping, not a secondary-index or collation feature. Source `grouped_reviewer_rowid_test.go.txt`, original five-case output `rowid-stream-v1.txt/.exit`. Later expansions of the source are separate from that original run.

Both findings were immediately reported to owner and lead. They are supported-shape semantic mismatches, rather than oracle formatting failures. The exact v1 freeze remains unchanged; no production edit was made by the reviewer.

## Other v1 evidence

PASS: candidate-owned grouped parser/native/driver/new-oracle selection on the independent execution copy. Guard and behavioral audit preceded each test batch. Exact script/log: `run-owned-v1.sh`, `owned-v1.txt/.exit`. The 508-case independent matrix compares complete labels, values and Go storage classes on successful queries; failures compare ordinary overflow errors and reject any native partial grouped output. NUL data is inspected through hex/typeof to avoid the CLI quote display's known truncation.

Source inspection checked the pinned select.c/expr.c group/HAVING/aggregate discovery logic and func.c arithmetic ancestry; key comparison is storage-class-aware without affinity coercion and uses exact mixed int64/REAL ordering. Distinct NULL, INTEGER/REAL, TEXT/BLOB, empty values, NUL bytes and large numeric boundaries are covered. Representative selection and computed-root versus nested grouping expression behavior passed the requested matrix. Input and output share one budget and private child scopes; source/state/growth/copy/output reservations and cancellation paths were inspected. Full grouped acceptance remains pending corrected source and final checks.

## Expanded v1 characterization

The separate 316-case follow-on selection (`run-expanded-v1.sh`, `expanded-v1.txt/.exit`) has 224 PASS and 92 FAIL: 84 additional top-K projection cases and eight rowid early-stop cases, all belonging to the same two findings. Rowid grouping identity also appears in duplicate/composite keys and a later key position, e.g. `GROUP BY id+0,id`; a row-2 WHERE overflow remains a required lookahead error. Original first-run sources are preserved separately as `grouped_reviewer_original_v1_test.go.txt` and `grouped_reviewer_rowid_original_v1_test.go.txt`.

Additional official-only ordinary in-memory probes preserved in `rowid-order-extra-official.txt` and `rowid-null-order-official.txt` establish negative controls: extra ORDER terms or mismatched GROUP/ORDER arity do not automatically permit stream elision, and unnatural NULL placement still requires the later error. These diagnostic probes do not claim any native pass. `rowid-order-prefix-official.txt` preserves an earlier expected-compatibility hypothesis that the official engine rejected; it is not a product finding.
