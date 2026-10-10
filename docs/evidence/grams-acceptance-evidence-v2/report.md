# Grams normalized acceptance and ordinary performance result

## Result

Implemented and verified the bounded ordinary slice. The native benchmark harness and explicit NFC-input acceptance gap are closed for the recorded source/profile. This is not full M4/M10/goal acceptance, a large-release benchmark, a latency win, or durability/recovery qualification.

Final source: `grams-acceptance-freeze-v2`, tree `762a9e9dd8105cf620c68a8398922ddf254a9d5f`.
Baseline: `plain-final-verify`, tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`.
Only four additive files; all engine/API/main CLI/central docs/goal/existing tests unchanged. Complete baseline/final inventories and checked patch are retained. No commits, uploads or remote writes.

## Correctness and execution evidence

- New persisted corpus: 128 randomized NFC texts × 100 randomized patterns = 12,800 executed query/text memberships. NFC changes 100 generated inputs. All 100 query plans use real raw grams SEARCH, 45 results are nonempty, and actual gram lengths 1/2/3 are required. Complete pre-index scans and post-index real forced scans equal the indexed results.
- Independent review additionally found 128 distinct texts and 97 distinct patterns, giving 12,416 distinct query/text pairs. The reported 12,800 counts executed combinations, not SQL queries or necessarily unique pairs.
- NFC corpus SHA256 (length-prefixed UTF-8 texts then patterns): `ab4a4a4d2cbb9d85630b88d0d7b1bdeabf7d74820f69702ba4cf5431f3472944`.
- Ordinary rollback/shared-WAL DML, rowid changes, read-own-writes, rollback/savepoint, REINDEX, peer visibility, checkpoint and normal reopen reconcile index/source/frequencies and retain full query equality. Untouched raw/decomposed/Unicode/ESCAPE/NUL/NULL/Boolean/fallback/MATCH controls pass.
- Owner selected sqlite normal/race: 101.369s / 443.583s. These run on v1; the complete inventory delta proves v2 only changes benchmark main.go/README and the sqlite/imported source closure is identical.
- Owner final v2 harness normal/race: 24.420s / 114.818s; source guard, vet, native CGO0 build/dependency closure and invalid-option rejection pass.
- Independent v2 selected sqlite normal/race: 131.769s / 419.481s; harness normal/race: 49.085s / 105.015s; vet/build/no-CGO closure pass. Independent evidence remains separately attributed in `grams-independent-evidence-v2`.
- Final repeated trimpath CLI builds and the independently built executable all hash `7c80d5a732adc95b534a8b04b128c51bdfc5f0ff6a64160b9e5f356f170ef40f`.
- Repeated indented smoke corpus files hash `e3ed9f485418869a1af81d930bb2822148eaae72570d5025fd356422a368e3e9`; this differs from compact JSON workload identity solely by serialization format.
- Independent final review `../grams-independent-evidence-v2/FINAL.md` (SHA256 `63739e9dd93cfe3952441d4f9e3f8a667a9ac346fb05ba2cedde104293d90d40`) accepts the bounded source/test/measurement slice. Both owner reports were independently checked against expected ID hashes regenerated from each corpus, plus exact counters, quantiles, standard deviation, identities and space arithmetic.
- No test or benchmark failure occurred. Expected invalid-flag rejection happens before a DB opens. The unsuccessful lscpu discovery command and its supported /proc fallback are disclosed in scope.md.

## Actual standalone measurements

All are CGO0 Go 1.27.2 Linux/amd64; visible host model AMD EPYC 9V74 80-Core Processor, 9 allowed CPUs, GOMAXPROCS=2, page 4096, seed 2026101052. Overlay backing medium, OS-cache state and co-tenant workload are uncontrolled. Native build samples are for the whole joint 1/2/3-gram index, including both LIKE_RAW_V1 and MATCH_CONFIG_V1 namespaces.

### Smoke

- 128 rows; 6 queries per gram length; 2 independently built databases. Both cold-connection and warmed-process profiles, plus two explained SCAN controls each.
- Whole harness elapsed: 52.052s. Workload SHA256: `a36ee786cd4c307b79d0c8a4abf35cb685bb9a336252010aa25cac015280be33`.
- Full ordered result equality: 80/80 pairs; 72 genuine SEARCH pairs and 8 SCAN-control pairs; 10,240 executed memberships.
- Index builds (ms): 11396.996, 8919.614.
- Before/with-index bytes per build: 24,576 / 3,141,632; incremental allocation 3,117,056 bytes; database amplification 127.833×; incremental bytes/source UTF-8 text bytes 370.946×.
- Plans, complete result hashes, work counters and file-size observations are identical across repeats after excluding timing/order metadata; deterministic projection hashes are retained in measurements-summary.json.

| Cache | Grams | Automatic P50 ms by repeat | Automatic P95 ms by repeat | Scan P50 ms by repeat | Scan P95 ms by repeat | Mean source fetches automatic / scan |
|---|---:|---|---|---|---|---|
| cold_connection | 1 | 0.8461, 0.8091 | 3.2556, 5.3630 | 0.4346, 0.3808 | 0.6645, 1.5862 | 4 / 128 |
| cold_connection | 2 | 0.7267, 1.0035 | 8.1913, 13.3116 | 0.4107, 0.5158 | 0.5249, 4.1115 | 4 / 128 |
| cold_connection | 3 | 0.7243, 0.7926 | 1.2698, 6.4448 | 0.4567, 0.5472 | 0.8082, 12.3673 | 4 / 128 |
| warm_process | 1 | 0.4487, 0.7628 | 0.4930, 4.4973 | 0.2737, 0.4434 | 0.8756, 0.6897 | 4 / 128 |
| warm_process | 2 | 1.0130, 0.6792 | 1.7306, 0.9122 | 0.2726, 0.4380 | 0.4558, 16.1046 | 4 / 128 |
| warm_process | 3 | 0.5793, 0.6745 | 1.1595, 7.4917 | 0.4563, 0.4790 | 1.3139, 5.5580 | 4 / 128 |

### Medium

- 512 rows; 12 queries per gram length; 3 independently built databases. Both cold-connection and warmed-process profiles, plus two explained SCAN controls each.
- Whole harness elapsed: 290.019s. Workload SHA256: `4075cb1cba12aec7e0bc69ed5949f388e3887ab7b3cddf2dad0757df04b1bed5`.
- Full ordered result equality: 228/228 pairs; 216 genuine SEARCH pairs and 12 SCAN-control pairs; 116,736 executed memberships.
- Index builds (ms): 65291.547, 37267.472, 41224.174.
- Before/with-index bytes per build: 77,824 / 11,440,128; incremental allocation 11,362,304 bytes; database amplification 147.000×; incremental bytes/source UTF-8 text bytes 348.558×.
- Plans, complete result hashes, work counters and file-size observations are identical across repeats after excluding timing/order metadata; deterministic projection hashes are retained in measurements-summary.json.

| Cache | Grams | Automatic P50 ms by repeat | Automatic P95 ms by repeat | Scan P50 ms by repeat | Scan P95 ms by repeat | Mean source fetches automatic / scan |
|---|---:|---|---|---|---|---|
| cold_connection | 1 | 1.3687, 1.2383, 0.9695 | 4.4111, 4.1180, 2.5137 | 1.5142, 1.2345, 1.3217 | 4.4205, 2.0488, 2.1152 | 16 / 512 |
| cold_connection | 2 | 1.2551, 1.0091, 1.1426 | 4.7031, 1.5548, 1.8697 | 1.3048, 1.0888, 1.0755 | 11.8272, 2.4370, 1.9726 | 16 / 512 |
| cold_connection | 3 | 1.2589, 1.2282, 1.2252 | 5.3998, 1.9514, 3.0250 | 1.7744, 1.5046, 1.4535 | 7.7301, 5.6362, 4.6499 | 16 / 512 |
| warm_process | 1 | 1.3001, 1.1589, 0.9759 | 9.0469, 2.1731, 3.6621 | 1.8312, 1.0471, 1.1566 | 12.6924, 2.1175, 2.1849 | 16 / 512 |
| warm_process | 2 | 1.2942, 1.2658, 1.0096 | 7.8607, 2.5351, 1.4602 | 1.8061, 1.2570, 1.1390 | 20.1388, 1.7327, 2.0133 | 16 / 512 |
| warm_process | 3 | 1.0453, 1.0141, 1.2336 | 3.6394, 1.6652, 2.3990 | 1.3547, 1.2820, 1.4605 | 5.0806, 3.4201, 1.6820 | 16 / 512 |

## Interpretation and limits

On the smoke profile, automatic/index P50 was slower than real-scan P50 in every one of the 12 selective cache/length/repeat buckets, despite fetching 4 rather than 128 source rows. This small-workload result is preserved, not described as a speedup. The independent reviewer reproduced complete results, plans and counters in a separate native run and independently recomputed this report’s smoke results and arithmetic.

The medium profile also has automatic/index P50 slower than scan in 4 of 18 selective cache/length/repeat buckets; other observed buckets are faster. Its total database allocation is 147× the pre-index database. These are measured profile-specific costs, not statistical generalizations.

The measured index reads fewer source rows, but this does not imply faster latency. Compare the actual same-machine values above; no statistical speedup claim is made. The full JSON retains every timed query, raw native plan, frequency/posting/source-work counter, empirical quantile, mean and sample standard deviation. Confidence-method labels are descriptive and CI bounds are null. Small-sample P95 may equal the maximum. Controlled synthetic markers have about 1/32 selectivity and query draws can repeat.

Cold connection means fresh connection/no workload warmup, not cold OS cache or cold process. Warmed-process queries follow a complete untimed workload pass on that connection. Timings include parsing/planning/complete query consumption/Rows.Close, excluding connection open/close, comparison and integrity checks. Cache profile and automatic/scan order alternate. Build and allocation metrics describe the whole joint index, not per-length attributable costs, isolated live payload, peak RSS or temporary disk use.

No larger release workload was run. The proposed 10,000-row × 96-query/length × 5-build profile remains unapproved/unmeasured here and is an engineering proposal, not a numeric requirement in the goal. No recovery timing, physical failure/namespace/crash/tamper/wrong-key/interrupted-publication work was run. Stable-hardware release qualification, larger representative workloads, CI regression policy, long soak and restricted/external acceptance remain separate.

## Files and identities

- Commands, terminal gates and measurement execution: `checks-and-measurements.sh/.log/.exit`; original exact-closure sqlite evidence: `../grams-acceptance-evidence/checks.sh/.log/.exit`.
- Raw actual results: `smoke.json`, `medium.json`, their stderr/exit files and load observations. Derived arithmetic/deterministic projections: `measurements-summary.json`, `summarize.py`.
- Scope/methodology: `scope.md`, source `benchmarks/grams/README.md`, actual `environment.txt`, executable build settings, no-CGO closure.
- Full inventories, owned baseline/final hashes, source checksum lists, `changes.patch`, patch replay result and complete source revalidation after measurements.

- SHA256 `changes.patch`: `d9feb249e2678f76d58da6d0bd25a56d47c1dc8a9dfd51b8060210fc75d9cbfe`
- SHA256 `owned-files.json`: `6a02f1610370617b1caca4866dd531476e3554d78d318bd609aa203f0fc8534d`
- SHA256 `baseline-inventory.json`: `54e73a824baf208687fddbace5754737447dde7b660262fd3c07abd901b2ff96`
- SHA256 `final-inventory.json`: `50bc2590a3cde425801bb162dfabbe9bec7ae2c8fbbbcb79cd1054580757e2c1`
- SHA256 `smoke.json`: `ba4b8ff37827db9b28f57c6b5a8b64c3c6798b087b113d7bdaa4b7153dd87ccd`
- SHA256 `medium.json`: `a0f61e1da2b14989bc772c8fcc887b37d5d9b657d55f6155b1e94b63c9715603`
