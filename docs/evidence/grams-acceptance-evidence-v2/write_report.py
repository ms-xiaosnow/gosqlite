from pathlib import Path
import hashlib,json
p=Path(__file__).parent
s=json.loads((p/'measurements-summary.json').read_text())
assert set(s)=={'smoke','medium'}
lines=['# Grams normalized acceptance and ordinary performance result','',
'## Result','',
'Implemented and verified the bounded ordinary slice. The native benchmark harness and explicit NFC-input acceptance gap are closed for the recorded source/profile. This is not full M4/M10/goal acceptance, a large-release benchmark, a latency win, or durability/recovery qualification.','',
'Final source: `grams-acceptance-freeze-v2`, tree `762a9e9dd8105cf620c68a8398922ddf254a9d5f`.',
'Baseline: `plain-final-verify`, tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`.',
'Only four additive files; all engine/API/main CLI/central docs/goal/existing tests unchanged. Complete baseline/final inventories and checked patch are retained. No commits, uploads or remote writes.','',
'## Correctness and execution evidence','',
'- New persisted corpus: 128 randomized NFC texts × 100 randomized patterns = 12,800 executed query/text memberships. NFC changes 100 generated inputs. All 100 query plans use real raw grams SEARCH, 45 results are nonempty, and actual gram lengths 1/2/3 are required. Complete pre-index scans and post-index real forced scans equal the indexed results.',
'- Independent review additionally found 128 distinct texts and 97 distinct patterns, giving 12,416 distinct query/text pairs. The reported 12,800 counts executed combinations, not SQL queries or necessarily unique pairs.',
'- NFC corpus SHA256 (length-prefixed UTF-8 texts then patterns): `ab4a4a4d2cbb9d85630b88d0d7b1bdeabf7d74820f69702ba4cf5431f3472944`.',
'- Ordinary rollback/shared-WAL DML, rowid changes, read-own-writes, rollback/savepoint, REINDEX, peer visibility, checkpoint and normal reopen reconcile index/source/frequencies and retain full query equality. Untouched raw/decomposed/Unicode/ESCAPE/NUL/NULL/Boolean/fallback/MATCH controls pass.',
'- Owner selected sqlite normal/race: 101.369s / 443.583s. These run on v1; the complete inventory delta proves v2 only changes benchmark main.go/README and the sqlite/imported source closure is identical.',
'- Owner final v2 harness normal/race: 24.420s / 114.818s; source guard, vet, native CGO0 build/dependency closure and invalid-option rejection pass.',
'- Independent v2 selected sqlite normal/race: 131.769s / 419.481s; harness normal/race: 49.085s / 105.015s; vet/build/no-CGO closure pass. Independent evidence remains separately attributed in `grams-independent-evidence-v2`.',
'- Final repeated trimpath CLI builds and the independently built executable all hash `7c80d5a732adc95b534a8b04b128c51bdfc5f0ff6a64160b9e5f356f170ef40f`.',
'- Repeated indented smoke corpus files hash `e3ed9f485418869a1af81d930bb2822148eaae72570d5025fd356422a368e3e9`; this differs from compact JSON workload identity solely by serialization format.',
'- Independent final review `../grams-independent-evidence-v2/FINAL.md` (SHA256 `63739e9dd93cfe3952441d4f9e3f8a667a9ac346fb05ba2cedde104293d90d40`) accepts the bounded source/test/measurement slice. Both owner reports were independently checked against expected ID hashes regenerated from each corpus, plus exact counters, quantiles, standard deviation, identities and space arithmetic.',
'- No test or benchmark failure occurred. Expected invalid-flag rejection happens before a DB opens. The unsuccessful lscpu discovery command and its supported /proc fallback are disclosed in scope.md.','',
'## Actual standalone measurements','',
'All are CGO0 Go 1.27.2 Linux/amd64; visible host model AMD EPYC 9V74 80-Core Processor, 9 allowed CPUs, GOMAXPROCS=2, page 4096, seed 2026101052. Overlay backing medium, OS-cache state and co-tenant workload are uncontrolled. Native build samples are for the whole joint 1/2/3-gram index, including both LIKE_RAW_V1 and MATCH_CONFIG_V1 namespaces.','']
for name,r in s.items():
 lines += [f'### {name.capitalize()}', '',
 f"- {r['rows']} rows; {r['queries_per_length']} queries per gram length; {r['repeats']} independently built databases. Both cold-connection and warmed-process profiles, plus two explained SCAN controls each.",
 f"- Whole harness elapsed: {r['elapsed_seconds']:.3f}s. Workload SHA256: `{r['workload_sha256']}`.",
 f"- Full ordered result equality: {r['full_query_pairs_equal']}/{r['full_query_pairs_equal']} pairs; {r['actual_search_query_pairs']} genuine SEARCH pairs and {r['truthful_fallback_query_pairs']} SCAN-control pairs; {r['membership_comparisons']:,} executed memberships.",
 '- Index builds (ms): '+', '.join(f'{v:.3f}' for v in r['build_ms'])+'.',
 f"- Before/with-index bytes per build: {r['before_bytes'][0]:,} / {r['after_bytes'][0]:,}; incremental allocation {r['incremental_bytes'][0]:,} bytes; database amplification {r['database_amplification'][0]:.3f}×; incremental bytes/source UTF-8 text bytes {r['index_over_utf8_text'][0]:.3f}×.",
 '- Plans, complete result hashes, work counters and file-size observations are identical across repeats after excluding timing/order metadata; deterministic projection hashes are retained in measurements-summary.json.', '',
 '| Cache | Grams | Automatic P50 ms by repeat | Automatic P95 ms by repeat | Scan P50 ms by repeat | Scan P95 ms by repeat | Mean source fetches automatic / scan |',
 '|---|---:|---|---|---|---|---|']
 for q in r['query_summary']:
  if q['gram_length']==0:continue
  cols=[q['cache'],str(q['gram_length'])]
  for k in ['automatic_p50_ms_by_repeat','automatic_p95_ms_by_repeat','scan_p50_ms_by_repeat','scan_p95_ms_by_repeat']:cols.append(', '.join(f'{v:.4f}' for v in q[k]))
  cols.append(f"{q['mean_automatic_source_fetches']:g} / {q['mean_scan_source_fetches']:g}")
  lines.append('| '+' | '.join(cols)+' |')
 lines+=['']
lines += ['## Interpretation and limits','',
'On the smoke profile, automatic/index P50 was slower than real-scan P50 in every one of the 12 selective cache/length/repeat buckets, despite fetching 4 rather than 128 source rows. This small-workload result is preserved, not described as a speedup. The independent reviewer reproduced complete results, plans and counters in a separate native run and independently recomputed this report’s smoke results and arithmetic.','',
'The medium profile also has automatic/index P50 slower than scan in 4 of 18 selective cache/length/repeat buckets; other observed buckets are faster. Its total database allocation is 147× the pre-index database. These are measured profile-specific costs, not statistical generalizations.', '',
'The measured index reads fewer source rows, but this does not imply faster latency. Compare the actual same-machine values above; no statistical speedup claim is made. The full JSON retains every timed query, raw native plan, frequency/posting/source-work counter, empirical quantile, mean and sample standard deviation. Confidence-method labels are descriptive and CI bounds are null. Small-sample P95 may equal the maximum. Controlled synthetic markers have about 1/32 selectivity and query draws can repeat.','',
'Cold connection means fresh connection/no workload warmup, not cold OS cache or cold process. Warmed-process queries follow a complete untimed workload pass on that connection. Timings include parsing/planning/complete query consumption/Rows.Close, excluding connection open/close, comparison and integrity checks. Cache profile and automatic/scan order alternate. Build and allocation metrics describe the whole joint index, not per-length attributable costs, isolated live payload, peak RSS or temporary disk use.','',
'No larger release workload was run. The proposed 10,000-row × 96-query/length × 5-build profile remains unapproved/unmeasured here and is an engineering proposal, not a numeric requirement in the goal. No recovery timing, physical failure/namespace/crash/tamper/wrong-key/interrupted-publication work was run. Stable-hardware release qualification, larger representative workloads, CI regression policy, long soak and restricted/external acceptance remain separate.','',
'## Files and identities','',
'- Commands, terminal gates and measurement execution: `checks-and-measurements.sh/.log/.exit`; original exact-closure sqlite evidence: `../grams-acceptance-evidence/checks.sh/.log/.exit`.',
'- Raw actual results: `smoke.json`, `medium.json`, their stderr/exit files and load observations. Derived arithmetic/deterministic projections: `measurements-summary.json`, `summarize.py`.',
'- Scope/methodology: `scope.md`, source `benchmarks/grams/README.md`, actual `environment.txt`, executable build settings, no-CGO closure.',
'- Full inventories, owned baseline/final hashes, source checksum lists, `changes.patch`, patch replay result and complete source revalidation after measurements.','']
for name in ['changes.patch','owned-files.json','baseline-inventory.json','final-inventory.json','smoke.json','medium.json']:
 lines.append(f"- SHA256 `{name}`: `{hashlib.sha256((p/name).read_bytes()).hexdigest()}`")
(p/'report.md').write_text('\n'.join(lines)+'\n')
