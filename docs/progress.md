# Implementation and acceptance progress

Test-scope correction: historical full suites from M2 through 7addc493 included two namespace-mutation tests. Their PASS outputs remain factual, but are not clean ordinary-scope evidence. See [the audit and gating correction](test-scope-audit.md).

Fresh restart, 2026-10-10 UTC. Historical work and test results are not acceptance evidence. Original goal SHA-256: 9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81.

### M0: Baseline and scaffold
- Status: VERIFIED
- Deliverables: pinned Go module, native CLI version/help, unit tests, ordinary-check script/CI, architecture/compatibility/security/format ADRs, durable original checkpoint.
- Key ADR: 0001-native-core-and-evidence; official SQLite is test-only; exact remote history and independent persistent bundle backups.
- Code: cmd/gosqlite, internal/buildinfo, scripts/check.sh, go.mod, .github/workflows/check.yml.
- Actual checks (Linux/amd64, Go 1.27.2, 2026-10-10): CGO_ENABLED=0 go test ./... PASSED (CLI package and buildinfo compiled); CGO_ENABLED=0 go vet ./... PASSED; two CGO_ENABLED=0 go build -trimpath -o <distinct path> ./cmd/gosqlite builds byte-identical; binary version output matched all baselines; go test -race ./... PASSED (race tooling with CGO enabled). Default home cache attempt FAILED because home is read-only; rerun with writable GOCACHE/GOPATH succeeded. Initial go.mod needed go mod tidy to remove redundant equal toolchain directive; fixed, go line remains exact 1.27.2.
- Oracle: official SQLite 3.54.0 source zip published SHA3-256 and generated sqlite3.c SHA3-256 independently matched; CLI source ID and in-memory integrity_check matched. No database-engine differential test exists yet.
- Limits: scaffold only. All M1–M10 database features unsupported. Ordinary-only remote CI PASSED for checkpoint 7742337875980eaa1fb78a7a6e6f2bd4c00d9114: https://github.com/ms-xiaosnow/gosqlite/actions/runs/38040601986.
- Risk/safety/migration: no database writes implemented, no production readiness claim; source/license review is informational and not an independent legal audit.
- Next: M1 read-only SQLite header/record/B-tree/schema and limited SELECT against official fixtures.

### M1: Native Go read-only SQLite
- Status: VERIFIED (documented Linux read-only subset)
- Deliverables: validated header/record/varint codecs, streaming table B-trees and overflow/schema reads, SELECT executor and parser, database/sql driver, query/inspect CLI and official fixtures.
- Key ADR: 0002-readonly-locks-and-limits; shared-lock handle lifetime, bounded resource usage and fail-closed unsupported formats.
- Code: internal/record, internal/pager, internal/btree, internal/sql, sqlite, driver, examples/readonly, cmd/gosqlite; oracle-tagged internal/testkit and committed testdata.
- Actual checks: final CGO_ENABLED=0 unit/vet/build and separate CGO_ENABLED=1 race gates passed; complete oracle-tagged suite passed with exact3.54.0 version/source verification; race-enabled oracle conformance passed. Four-target CGO-disabled builds passed (Linux amd64/arm64, Darwin arm64, Windows amd64). Four memory-only fuzz targets each ran5 seconds and passed; parser fuzz completed 48,614 executions. See docs/evidence/m1-aggregate-final.txt, m1-aggregate.txt and m1-memory-fuzz.txt.
- Official comparison:5,160 expression/affinity cases in one suite; end-to-end suite 9 top-level tests/272 events with80 seeded predicates and300 seeded scalars. Real512/4096/65536-byte fixtures each contain1,028 records, plus separate schema/ALTER fixtures. Oracle integrity_check=ok; Go reads preserve source SHA-256. See docs/evidence/m1-conformance.md and fixture manifests.
- Limits: writes/transactions/recovery/WAL, index traversal/planning and unlisted SQL are unsupported; native non-Linux Open fails; full integrity checking is absent; ORDER BY is bounded in-memory. Cross-builds are not native-OS verification.
- Risk/safety/migration: no database modification path; existing nonempty journals rejected rather than recovered. Read-only tests do not qualify durability, release security or legal review. Initial SQL discrepancies were corrected with retained oracle regressions.
- Next: M2 native transaction overlay/allocator, mutable table and index B-trees, journal-backed publishing and SQL CRUD, while keeping prohibited fault workflows manual/unverified.

### M2: Native writes and SQL MVP
- Status: VERIFIED (documented ordinary Linux SQL/write MVP; crash/fault durability excluded)
- Deliverables: SQLite freelist allocator, bounded transaction/statement overlays, stable-root local table/index split/merge and overflow mutation, native Linux writer locks, standard DELETE journal normal-success publication, CREATE TABLE/INDEX and CRUD, explicit native/database/sql transactions, writable CLI and examples.
- Key ADR: 0003-write-journal-and-fail-closed. Existing journals refuse recovery; ambiguous physical-I/O errors poison the transaction. No crash safety claim.
- Code: internal/pager/{tx,freelist,commit,write_lock_*}.go, internal/journal, internal/btree/{mutable,index_read,ownership}.go, sqlite/{transaction,write_rows,write_schema}.go, driver, cmd/gosqlite, examples/write.
- Checks and official comparison: final CGO0 unit/vet/build, separate CGO1 race and full official-oracle suite PASSED; four-target CGO0 cross-builds PASSED, runtime remains Linux only. Six fixture manifest hashes and original goal hash unchanged. See docs/evidence/m2-aggregate.txt and m2-review.md. Ordinary-success oracle tests passed at512/4096/65536 bytes; official integrity_check and forced index reads matched. Valid-memory randomized table/index models, statement rollback, normal cancellation, constraints, BUSY retry, concurrency, normal cross-process lock contention and race checks passed in component runs.
- Review: canonical connection paths prevent cwd/symlink retargeting; uncertain PENDING cleanup poisons; journal refusal classification unified; whole-file page ownership and equal-depth checks prevent unsafe mutation of unsupported structure. The review is engineering source review, not independent security/legal certification. A real legacy-test failure mutated a generated fixture; explicit read-only options and exact verified fixture restoration fixed the cause. Final aggregate passed with unchanged fixture hashes.
- Limits: ordinary nonunique BINARY ASC indices maintained but SELECT still scans; unknown schema/constraints fail closed; only Linux runtime; no WAL/recovery, no savepoints, no full integrity_check. Existing zero-length files rejected. Public SectorSize is an explicit4096-default assumption, not device certification. Dirty-page and materialization budgets may reject large transactions safely.
- Risk/safety/migration: standard SQLite plaintext file format, no custom data encoding or hidden SQL delegation; source-only physical-error branches and all restricted crash/fault gates remain BLOCKED/NOT_RUN. No acceptance or production claim beyond ordinary M2 evidence.
- Next: persist verified M2 checkpoint, then M3 native recovery/WAL implementation and allowed concurrency/interop evidence; restricted gates remain manual and later milestone acceptance conditional.

### M3: Durability and concurrency
- Status: PARTIAL / BLOCKED; ordinary exclusive and shared-WAL paths implemented. M3 is not complete.
- Implemented: native WAL codec/committed-prefix scanner; optional connection-owned exclusive session; real shared-memory WAL-index dual headers, hash blocks, read marks, DMS and reader/writer/checkpoint/reconstruction locks; independent concurrent connection snapshots, deferred single writer and BUSY_SNAPSHOT; conservative append, partial/full checkpoint and safe reset. Native/driver/CLI support both explicit modes. Shared sessions never rename/unlink the WAL or SHM inode.
- ADRs: 0004-exclusive-wal-and-recovery-gates and 0006-shared-wal-protocol. Exclusive remains an intermediate/restricted alternative; the shared implementation now supplies actual single-writer/multiple-reader coordination rather than substituting an exclusive lock.
- Evidence: native valid-data page-size suites, all-four-readmark saturation, multiple hash blocks, pooled writes, ordinary rollback/reopen and live official SQLite 3.54.0 concurrent reader/writer/checkpoint/reset tests pass. Public/driver/CLI and live oracle race runs pass. Full aggregate and review evidence: docs/evidence/m3-shared-aggregate.txt and m3-shared-review.md. No failure-path execution is implied.
- Required implementation still open: broader rollback-journal recovery segments/modes and automatic safe Open recovery; incomplete/checksum-invalid current-generation WAL tail recovery; native non-Linux file locking. These remain code gaps, separately from unavailable execution gates.
- Required execution evidence blocked: physical hot rollback replay, failed/short/wrong-offset I/O, disk-full/permission faults, forced exit/power loss and interrupted checkpoint. Source-only recovery compilation is not runtime recovery verification. These gates remain BLOCKED/NOT_RUN and cannot be moved elsewhere to evade the restriction.
- Limits: Linux native runtime, writable DB/WAL/SHM required, one local lease per connection, bounded WAL growth/work, no native shrink, conservative imported-sector append/checkpoint gates unless caller explicitly asserts PSOW. Cold reconstruction accepts only committed data, valid discarded rollback suffixes or normal old-salt reset suffixes; other ambiguous tails refuse. No production acceptance claim.
- Next: preserve the reviewed shared implementation durably; implement remaining recovery/platform code under the same execution restrictions while advancing independent M4 features.

### M4: Persistent grams13 and indexed LIKE
- Status: ORDINARY PATH VERIFIED / CONDITIONAL; final aggregate and engineering review passed. Crash/recovery acceptance remains BLOCKED/NOT_RUN, so full M4 is not VERIFIED.
- Deliverables: immutable real method registry; legal versioned catalog/field/posting/frequency tables and covering B-trees; transactional full build, old/new incremental deltas, rowid changes, cancellable atomic rebuild/config changes, normal rollback/reopen/checkpoint; strict shadow ownership and external-writer guards/schema epochs; bounded full extension consistency checker.
- Query path: real direct prefix ranges → candidate rowids → source SeekTable → original WHERE recheck. Safe AND/OR, high-frequency/budget/unsupported fallback, actual one/two/three-rune LIKE search, snapshot-configured grams13_match, EXPLAIN and Rows.Plan/Stats, explicit NOT INDEXED reference. Unknown formats/handlers fail closed. Public native/database/sql/CLI/example paths are real executable code.
- Code: extension/registry, extension/grams13/registry.go, sqlite/extension_*.go, plan_grams.go, plan_match.go, query/rows/transaction/write integration, internal/sql extensions, driver/CLI/examples/grams.
- ADR0005 records the design; grams13.md fixes actual APIs/semantics and differs from unsupported virtual-table/FTS syntax. A separate LIKE_RAW_V1 namespace prevents NFC substring false negatives; MATCH_CONFIG_V1 uses canonical pinned Unicode configuration.
- Actual component evidence:100 deterministic randomized queries ×128 texts =12,800 query/text memberships; all100 used SEARCH and equaled forced scans. Additional12,000 escaped-literal memory proof trials, one/two/three-rune fetch/EXPLAIN assertions, Unicode/ESCAPE/NUL/NULL/BLOB/numeric and configured MATCH tests passed. Pure-Go storage/registry tests cover real callback dispatch, unchanged posting identities, quotas/cancellation, atomic rebuild and rollback. Lead oracle six-way512/4096/65536-byte × rollback/shared-WAL matrix passed82.38s before final added reconciliation/MATCH checks; the final aggregate supersedes that component run. Full CGO0 unit/vet/build, separate full race, complete pinned oracle suite, four cross-builds and dependency verification passed; source/fixture hashes unchanged. Targeted latest review regressions and oracle race passed. Evidence: m4-aggregate.txt, m4-final-review-regressions.txt, m4-oracle-race.txt, m4-review-oracle-race.txt and m4-review.md.
- Review found and fixed VACUUM invalidation: external VACUUM can renumber implicit rowids without firing DML guards, so persisted schema-cookie checks now require reindex. Only known-clean/newly rebuilt indices receive native epoch refresh. Ordinary VACUUM regression and multi-index dirty-peer/DDL/rollback tests pass. Missing guard repair is narrow; unexpected changed guard code refuses.
- Limits: bounded no-spill build,1024 definitions/256 fields per index, configured tokenizer/transaction/query quotas; large rebuild can reject. General native triggers/ALTER/DROP, positional search and arbitrary planner plugins remain unsupported. CheckExtensionIndex is not general SQLite integrity_check. Unknown canonical metadata errors; dirty but intact metadata scans. Physical recovery, interruption, independent native-OS/security/legal/soak gates remain unverified.
- Durable verification: exact ad7d1c9082d715e91a7ef728e441737e72340408 mirrored remotely/locally, Library libfile_5849fa1d054481918c22f0994f770633 byte-identical readback and clean restore/fsck; restored ordinary unit/vet/build/race and complete pinned oracle passed. CI passed for exact SHA: https://github.com/ms-xiaosnow/gosqlite/actions/runs/38046568934.
- Next: integrate M5 SQL/scoring/ranking over the real persistent foundation; retain restricted gates.

### M5: Text similarity
- Status: ORDINARY PATH IMPLEMENTED / CONDITIONAL. SQL and persistent exact text search passed the M5 aggregate; M6 now adds same-snapshot vector/text/business-filter SQL and Go RRF integration. The earlier hybrid code gap is closed; restricted data-safety and release gates still prevent full acceptance.
- Deliverables: SQL ngram_similarity, bigram_similarity and rune Levenshtein; explicit DB/Tx.SimilaritySearch with persistent MATCH configuration, complete posting-union candidates or explained exact scan, business WHERE, deterministic threshold/Top-K, plan/config/generation identity and actual work counters; bounded pure-Go weighted RRF with contribution details.
- Key ADR:0007-exact-similarity-and-vector-values. Positive threshold plus nonempty query set implies a shared gram; the whole union is mandatory. Threshold0/empty sets/dirty indices/incomplete posting budget scan rather than lose recall. Scalars have explicit default NFC semantics, not implicit index configuration. No pg_bigm compatibility claim.
- Code: extension/similarity; internal/sql/{similarity,eval,functions}; sqlite/similarity_search.go plus context propagation; driver tests; examples/similarity. Full API/edge semantics: similarity.md.
- Component checks: mathematical12,000 set/6,000 edit-distance independent references,288 native indexed-versus-forced-scan comparisons,504 additional independent map/full-sort recall cases across six gram ranges/both metrics,5000 exact-decimal control references, SQL/driver/context/quotas/NULL/Unicode/NUL/rowid/filter/configuration/rollback/reopen/checkpoint tests passed. Focused unit/race/vet pass. Scoped review found/fixed trailing-line-comment filters and post-Valuer NULL cancellation; pre-copy input/metric limits were checked. In-memory math fuzz216,581 executions/15s passed. Evidence: m5-review.md.
- Live official SQLite comparison: ordinary external UPDATE/INSERT/DELETE invalidation produces exact SCAN; REINDEX restores SEARCH; normal shared checkpoint plus integrity_check succeeds. Final targeted oracle race15.417s, driver race count3 1.276s, executable example and CLI scalar-ranking output pass; see m5-driver-oracle-example.txt. SQLite has no built-in matching custom-score oracle; independent mathematical references validate scores.
- Aggregate: full CGO0 unit/vet/build, separate full race, complete pinned-oracle suite, four CGO0 cross-builds, dependency verification and unchanged original/fixture hashes PASSED. sqlite unit45.065s, full race239.330s; testkit oracle101.089s. See m5-aggregate.txt. Two trimpath CGO0 CLI builds were byte-identical and statically linked (m5-build.txt); build identity honestly records the precommit working tree.
- Limits/risks: SQL score expressions scan; indexed search is an explicit native API. Native needs a named persisted grams13 index even ForceScan; operations have strict candidate/input/token/DP/comparison/result budgets and return zero result on error. SQL streaming users must check Rows.Err. Registry/Valuer callbacks are trusted application code. Full restricted physical recovery, native-OS, independent security/legal and soak gates remain unverified. No new similarity disk schema is introduced.
- Durable verification: exact e04308ae9a95dd48a6d44f46312929708692ded1 and Library libfile_2b0076ca6d3881919ac4dbf3c8e82273 verified; CI passed. The first restored-test completion was lost to executor transport and not counted; retained-log rerun passed all ordinary gates and pinned oracle130.715s (m5-library-restore-recheck.txt).
- Next: persist M6 vector/hybrid integration after its own aggregate, retaining all restricted gates.

### M6: Dense vectors and exact KNN
- Status: ORDINARY IMPLEMENTATION COMPLETE / CONDITIONAL; independent scoped review, focused gates and full ordinary aggregate passed. Crash/recovery and release acceptance are not claimed.
- Deliverables: legal VECTOR(n), strict finite/dimensional type validation with canonical versioned BLOB mutation, nullable native/driver values, nine SQL functions/four dense operators, DB/Tx.VectorSearch, actual bounded SQL heap ranking with LIMIT/OFFSET, observable source/ranking plans and work, same-snapshot text/vector/business-filter RRF example retaining original scores/distances/contributions.
- Key ADR:0008-typed-vectors-and-exact-ranking. Metric identity is closed; source access and ranking are distinct. Native NULL candidates are excluded; SQL ASC retains NULL-first ordering. Unsupported heap shapes use the general sorter. Ordinary equality retains SQLite storage-class semantics, not pgvector equality.
- Code: extension/vector; internal/sql/vector.go and parser/lexer/eval/functions; sqlite/{vector_value,vector_search,plan_vector}.go and transaction/query integration; driver/vector_conformance_test.go; examples/vector; oracle vector conformance. Full APIs/limits in vector.md.
- Actual component checks:768 implementation-side native searches against independent references;216 additional review-native searches and216 SQL heap/full-sort comparisons;61 published pgvector SQL golden cases; typed defaults, dimensions/nonfinite/binary rejection, large16000-component overflow values, signed-zero/subnormal persistence, NULL/zero norms, negative-IP sign, deterministic ties, filters, quota classification, late-error statement atomicity, cancellation/reader release, Tx snapshots, rollback/normal reopen/checkpoint all pass. Focused repeated/unit/race/vet checks pass. Pure-vector prerequisite coverage98.8% belongs only to that package; additional review memory Top-K fuzz24,072 executions passed. Review evidence: m6-review.md.
- Ordinary official comparison: six page-size512/4096/65536 × rollback/shared-WAL cases plus official-created VECTOR(3), valid external text/BLOB writes, native canonical rewrite, exact bit preservation and integrity_check pass. Ordinary official wrong-dimension TEXT has consistent typed errors in heap/general-sort/projection while excluded rows remain lazy; valid corrective UPDATE works. Final oracle race1.279s and driver race count3 1.142s passed. Executable hybrid and CLI results match their asserted plans. See m6-driver-oracle-example.txt.
- Review fixes: heap initially skipped referenced typed-candidate validation for a NULL query unlike the scalar fallback; now consistent and regression-pinned. Typed quota errors no longer acquire an erroneous mismatch code. Rows.Close releases cached vector-ranking state while preserving diagnostics. No unresolved concrete scoped review issue remains; this is not an independent release audit.
- Aggregate: complete CGO0 unit/vet/build, separate full race, pinned-oracle suite, four CGO0 cross-builds and module/dependency closure checks PASSED. sqlite unit53.994s, full race275.033s, oracle testkit101.202s; original goal and fixture hashes unchanged. Exact captured output and success marker: m6-aggregate.txt. Native non-Linux execution remains unverified.
- Provenance/precision: pgvector v0.8.7 commit f37c13f68b57d2c3472b2214fbcff699d6d34876 source/fixture pins; no PostgreSQL/pgvector executable. Float64 compensated arithmetic intentionally differs from upstream float32/FMA extremes; zero-norm cosine errors rather than NaN. GSV encoding is not PostgreSQL binary COPY/wire. Original license retained; independent legal audit not run.
- Limits/risks: bounded memory, no ANN yet; native physical typed column required; SQL vector operators are SQLite-extension grammar, not full PostgreSQL syntax. Ordinary vector read/write/reopen evidence is not interrupted recovery evidence. M3 broader recovery/native-platform code plus all restricted fault/security/legal/soak gates remain separately open. No encryption/confidentiality is supplied.
- Next: persist reviewed ordinary M6 checkpoint, then implement actual persistent HNSW before IVFFlat, with exact fallback and explicit approximate diagnostics.

### M7: Persistent approximate vector indexes
- Status: HNSW and IVFFlat ORDINARY IMPLEMENTATION / CONDITIONAL. Both real persistent methods exist; physical recovery and release acceptance remain open. The first-slice evidence below is historical, followed by the IVFFlat increment.
- Deliverables: independently written deterministic multi-layer HNSW, legal persistent SQLite state/node/live mapping with optional backwards-compatible ANN family, lazy B-tree node access, immutable identity/tombstones, same-transaction mutation/reindex, filtered widening, exact base recheck/rescore and cumulative bounded fallback. Native exact remains default; eligible SQL may choose approximate HNSW with NOT INDEXED as exact reference.
- Code: extension/vector/ann, sqlite/ann_*.go and planner/catalog/lifecycle integration, examples/ann, benchmarks/ann. ADR0009, ann.md and file-formats.md document actual API, metadata and limits.
- Ordinary evidence: algorithm/model/query/storage/race/vet pass; four independent review regressions cover same-Tx updates/deletes/rowid reuse/reindex/rollback, mixed grams/ANN, signed int64 extremes and SQL lazy/empty/large-limit cases. Reviewer found/fixed requested-LIMIT memory overreservation against tiny live cardinality. Memory adjacency fuzz407170 executions passed. GOARCH386 compiled but runtime returned exec format error; no32bit execution claim.
- Official SQLite comparison: six page-size512/4096/65536 × rollback/shared-WAL cases passed10.299s, including native source/graph updates, external legal source DML dirty guards, exact fallback, rebuild, normal close/reopen/checkpoint and official integrity_check. An actual M6-created grams-only fixture upgraded without breaking its index; original seven shadow schemas are preserved. Evidence m7-oracle-focused.txt.
- Persistent measurement:2048×12 finite float32 normal rows,48 held-out queries, fixed seed20261010, default m16/ef_construction64/ef_search40. Reopened persistent index achieved recall@1=1.0, @10=1.0, @100=0.999375 with zero exact fallbacks. @10 average source fetches40 vs2048 and graph distances575.08 vs2048. HNSW was slower: p50~40.62ms vs exact~2.12ms. Build49.38s; database1462272bytes vs286720 before index. Shared-cloud warm-cache observations, not universal recall, isolated hardware or speedup claim. JSON and runnable benchmark retained.
- Driver: prepared binary vector bindings, SQL HNSW EXPLAIN, same-Tx update/rollback and REINDEX pass in rollback/shared modes; final race count3 passed11.294s. An initial test-only assertion incorrectly expected method=hnsw rather than the actual USING ... hnsw v1 detail; it was corrected, no production change.
- Aggregate: full CGO0 unit/vet/build, separate full race, complete pinned SQLite3.54 oracle, focused HNSW oracle race, four-target CGO0 cross-builds, dependency verification and empty CgoFiles closure PASSED, exit0 at2026-10-10T12:01:06Z. sqlite unit53.470s/race278.229s; full oracle testkit118.424s/sqlite59.580s; HNSW oracle race40.848s. Original goal and all readonly/legacy fixtures unchanged. Evidence m7-aggregate.txt; final driver race supplements the earlier full-race compile.
- Limits: ANN dimensions<=2000; bounded work/tombstones and immutable construction config; cosine zero values reject; nullable automatic SQL ANN unsupported; project grammar differs from PostgreSQL. At this first HNSW checkpoint IVFFlat/probes remained open; the increment below supplies them. Broader recovery/native platforms, restricted physical durability and independent audits/soak remain open. No full acceptance or production claim.
- Durable first slice: exact5a794e5d9fd6e04a3e488c1ad4fd82de74641d6f, Library libfile_aed7ff62fdf081919ae2c03043d27d58. Byte-identical Library readback/clean clone/fsck verified; restored full ordinary gates and pinned oracle passed exit0 at12:12:46Z. CI38050546401 passed for that exact SHA.

#### M7 IVFFlat increment
- Delivered: independent deterministic bounded minhash training, L2/spherical cosine/IP clustering, persisted untrained state, fixed-centroid transactional DML, actual centroid/member/reverse B-trees, three matching metric/opclass pairs, selected-range probing/filter compensation, original-vector rescore and cumulative exact fallback. All-list probes truthfully SCAN; native default remains exact.
- Compatibility: original dense HNSW canonical bytes and original grams shadow family preserved. A real previous-checkpoint HNSW fixture is copied before upgrade; old fixture hash remains unchanged. First matching automatic index can mask a later trained index; native named Index avoids this. EMPTY queries validate global probe ranges but skip index-dependent validation.
- Component evidence: algorithm unit/race and memory fuzz, all-metric independent membership/training/quota references, signed rowid extremes, NULL/IP-zero/cosine policy, fixed-centroid DML, statement rollback and failed rebuild preserving earlier work, shared-WAL snapshot, native/SQL planning, cumulative budget fallback and six-way ordinary SQLite-oracle tests pass. Source review found no remaining concrete blocker for this scope. See m7-ivf-integration.txt and m7-ivf-oracle-focused.txt; final aggregate and driver checks are separately recorded.
- Persistent benchmark: same2048x12,48held-out,seed20261010 as HNSW, lists100. Probes1/10/50 recall@10=.2541667/.7708333/.9958333; mean centroid scores100 and source scores23.9167/239.3542/1149.625, versus exact2048. No exact fallbacks. Corresponding median ANN6.42/41.12/142.09ms versus exact3.06/3.24/2.42ms; no speedup claim. Probes1 may widen to2 for insufficient result count. Build1.66/1.53/1.23s,503808bytes versus286720before. Raw JSON retained; probes1 has a legacy descriptive graph note, with actual method/counters unaffected.
- Aggregate and final supplement: full CGO0 unit/vet/build, separate full race, complete pinned oracle, four CGO0 cross-builds, dependency verification and empty CgoFiles closure passed exit0 at12:37:41Z. First unit sqlite52.620s/full race305.030s; complete oracle testkit201.560s/sqlite79.728s. A final source-only sidecar-preflight change and late examples/driver tests are covered by final CGO0 unit/vet/build, focused storage/driver race, driver IVFFlat race count3, runnable examples and4096-byte IVF/encrypted oracle race53.702s, exit0 at12:33:59Z. First compiled full-race binaries preceded that final source change; the supplement explicitly qualifies it. See m7-ivf-m8-snapshot-aggregate.txt and final-supplement.txt. Original goal/fixture hashes unchanged.
- Limits: dense only, bounded training/query memory and work; no L1 IVF, no silent retraining, no automatic dynamic tuning or general SQL ANN hint. ANN approximation remains explicit in diagnostics. Restricted recovery and broader remaining code are not qualified by these ordinary results.

### M8: Authenticated encryption
- Status: experimental snapshot, dirty-page WAL and Linux shared encrypted sessions implemented; non-Linux encrypted coordination remains required code. Physical tamper/recovery and independent security acceptance remain open.
- Delivered: write-once AES256GCM records, domain-separated HKDFSHA256, bounded Argon2id, code KeyProvider lifecycle, strict bootstrap/record codecs and expected-location authentication. Final component unit0.921s/race4.113s/purego0.958s/vet passed; upstream RFC Argon2 test0.005s passed. AES/HKDF known answers, memory-only substitutions/truncation/nonce/provider/cancel/race tests pass; three10second actual fuzz campaigns total1400866 executions pass. Evidence m8-primitives.txt, ADR0010. At that primitive-only checkpoint no database encryption was integrated. The following increment supplies an explicit encrypted session; plaintext options still remain plaintext. No SEE compatibility or independent cryptographic audit is claimed.

- Snapshot increment: native SQL/database/sql correct-key encrypted create/read/write/rollback/normal reopen; raw/password provider, full-page root/base authentication, stable Linux lifetime leases, rekey, portable encrypted backup/restore and explicit plaintext export/import. Changed commits copy full logical snapshot with fresh identities; no plaintext journal/WAL/staging path. Mode rejects native WAL options. ADR0011 and encryption-storage.md give the exact format/API/quotas.
- Review and component checks: engineering source review plus memory canonical/geometry/accounting tests and correct-key ordinary storage/race/vet passed. Three-page-size512/4096/65536 oracle lifecycle with long text, overflow, grams/HNSW, rekey/copy/import/export and official plaintext integrity_check passed77.501s. First run failed because standalone restore validation omitted default quotas; shared normalization fixed it and the unchanged rerun passed. Both logs retained.
- Resource/namespace fixes: credential and actual geometry preflight precede new lock creation, new-family scan reserves lock slot, no-follow/nonblocking regular-file checks, bounded family inventory and backup bytes, single-link pinned plaintext import. A final source-only fix moved readonly plaintext-sidecar preflight before possible stable-lock creation while retaining under-lock recheck; its refusal branch was not disk-negative tested. No remaining concrete scoped reviewer blocker; not an independent cryptographic certification.
- Snapshot-checkpoint limitations (historical; the WAL increment below updates live namespace handling): old credentials decrypt retained history after rekey; no secure erase/historical revocation. Whole-family authenticated replacement/historic rollback lacks external expected identity/freshness protection. Canonical parent paths are not continuously pinned directory namespaces. Writer lifetime EXCLUSIVE prevents concurrent writable/read-only sessions; multiple read-only SHARED sessions work. Snapshot-only is intermediate, not encrypted WAL or full M8 acceptance.
- At the snapshot checkpoint, remaining code included authenticated WAL segments/checkpoint/reopen and broader encrypted concurrency, later per-database ATTACH key policy when ATTACH exists; no spill implementation exists and no plaintext spill allowed. Remaining evidence: wrong-key/tampered-on-disk, interrupted publication/rekey, physical fault/crash/recovery and independent native-platform/security/crypto/legal/soak gates BLOCKED/NOT_RUN. Do not route restricted execution elsewhere.

#### M8 authenticated dirty-page WAL increment
- Delivered: explicit EncryptionWAL/layout2, immutable final-dirty-page segments, complete authenticated ordered selection and content ancestry, bounded overlay reads, full-head checkpoint/rekey/copy, normal retained-head reopen, explicit snapshot upgrade and mode-preserving rekey. Existing snapshot/base/archive/primitive bytes remain intact. Live Linux family namespace operations use a pinned parent directory and relative opens/publication; copy destinations and initial path selection retain disclosed limitations. ADR0012 and encryption-storage.md give actual layouts, APIs and quotas.
- Engineering review fixed six concrete issues: selected object/candidate identity checks, complete source authentication before rekey, canonical ancestry, growth-aware replay accounting, summed publication I/O and old/new per-master minted-ID ledger memory. Focused CGO0 tests/vet and separate race pass; frozen-source memory-format fuzz144089 executions passed6.035s. No remaining concrete scoped blocker was identified; this is not external cryptographic acceptance.
- Actual sparse publication: independent ordinary SQL fixture301 logical pages ->111 final dirty frames/62644-byte segment; existing base bytes unchanged. Additional official-oracle full-head lifecycle at512/4096/65536 passed17.880s, including grams/HNSW/IVF/ordinary indices, normal rollback/reopen, explicit plaintext export integrity and exact results, encrypted archive/restore/import and mode-preserving rekey. Its respective head/dirty-frame/segment sizes were574/164/92112,80/48/199648 and21/19/1246948. All credentials/files were correct/intact.
- Evidence: m8-wal-components.txt, m8-wal-source-sha256.txt, m8-wal-oracle-focused.txt and final oracle race91.950s; final aggregate and documentation/example supplement are separate. No physical tamper, wrong-key disk, missing selected disk file, namespace-race injection, interrupted publication or recovery gate was run.
- Remaining code: simultaneous encrypted writer/readers, future ATTACH key policy and broader native platforms. Current lifetime EXCLUSIVE writer and SHARED readonly sessions are an intermediate model. Retained history/no-GC, old-key decryptability, whole-family replay/substitution and lack of external expected freshness remain risks. Full M8 acceptance stays conditional.
- Previous snapshot checkpoint bd6b19c9e88b7729fa0db953b6e280cd15ced239: exact Library libfile_832c64f26ac08191af1bf09abc9edafb readback/restore/fsck verified; restored ordinary checks and pinned oracle passed exit0 at12:55:04Z, CI38052742168 completed success. See m8-library-restore-recheck.txt and checkpoints.json.

### Ordinary SQL B-tree planner increment
- Delivered: actual rowid/IPK equality seek and nonunique BINARY ASC leading equality-prefix range -> bounded signed-rowid sort -> source seeks -> original predicate. ANN/grams precedence, NOT INDEXED scans and prior fallback diagnostics are retained. Error-capable residuals conservatively keep scanning; no inequality/covering/reverse/order-elision claim. ADR0013 and ordinary-index.md record semantics/counters/limits.
- Evidence: final focused tests and race passed68.816s/119.803s; full sqlite regression115.782s. Expanded six-way ordinary official-oracle/unit and race passed2.597s/4.523s; final-source oracle after fallback-reason fix passed1.731s. Cases include storage classes/affinities, int64/2^53, NULL, overflow/NUL text keys, normal transactions, shared snapshots, external legal DML and unsupported peer index definitions.
- Measured logical work:1800-row fixture20 matching keys/3 range pages/20 base rows versus1800 forced scan rows; separate1600-key late-prefix instrumentation3 pages versus84. Not a latency/universal speedup claim.
- Review initially caught two grams fallback-reason regressions; move equality extraction before safety classification fixed them without changing scan results. Final component logs retained. Physical durability remains excluded.

### M9: Migration and extended vector types
- Status: checkpoint A inspector, checkpoint B typed SQL converter, checkpoint E BIT, F HALFVEC and G SPARSEVEC integration plus extended memory primitives implemented. Resumable mapped import is implemented; transition tooling and all extended ANN remain incomplete code.
- Inspector: strict bounded exact-schema JSON, extension/type/method/operator/function identity chain, dependency/shape/default findings, direct finite binary32 real[] conversion, deterministic JSON/Markdown and CLI reports. Reports do not execute SQL or authorize changes. Invalid legacy defaults100/16 and example3/5 are review findings with no usable proposed target. Programmatic structs receive aggregate preflight; canonical inspected-facts hashes and optional unchanged-source hashes are distinguished.
- Component checks: CGO0 unit/vet/build, separate race, CLI and bounded memory fuzz pass (real-array62076, catalog194 inputs). Independent tests include201 finite binary32 roundtrips, NULL/shape/redaction, duplicate/unknown/UTF8 JSON, ownership spoofing, nonfinite profile numbers, mutated catalog provenance and deterministic cancellation. Initial invalid-parameter proposal assertion failed and was fixed; logs retained. See m9-inspector-components.txt and review evidence.
- Public references: old embedding0.3.6 commit5d48508aeaeb86b9e9c630468ada7a124b905795, exact Apache2 LICENSE/NOTICE and source manifest. Current Neon page was retrieved but lacks the requested chapter; the historical official2024 chapter is linked, not mislabeled current. Export SQL is source-reviewed but not PostgreSQL-executed. Synthetic catalogs are explicitly synthetic.
- Extended primitives: immutable HalfVector/BitVector/SparseVector, distinct GSH/GSB/GSSv1 codecs, explicit conversions and loss reports, cancellable typed metrics, nullable adapters, subvector and binary quantization. Dense GSVv1/APIs are unchanged. Half covers all65536 encodings and adjacent midpoint neighbors; sparse math is O(nonzeros) at dimension1e9, with explicit malformed-zero-index and underflow policies; zero/zero bit Jaccard is1.
- Extended component evidence: CGO0 tests x3/vet, separate race,207 published expected fixture cases and ten bounded memory fuzz campaigns passed. New production-file statement coverage100%, package99.6%; this scoped metric is not security acceptance. No PostgreSQL runtime oracle. Finite float64 arithmetic and documented upstream overflow/NaN differences remain intentional. See m9-extended-vector-components.txt and extended-values.md.
- Final-source qualification: the early inspector CLI transcript predates addition of explicit column Identity metadata; final-supplement.txt reruns current inspector/CLI tests and prints its updated canonical hash. Independent extended review adds729 operand pairs across six scales, cancellation checkpoints and zero-allocation/canonical-header regressions; final three-pass tests/vet/race pass with no production change.
- Required code still open: typed AST PostgreSQL SQL conversion, bounded local CSV/NDJSON import/resume and verification, review-only PostgreSQL transition bundles, HALFVEC/BIT/SPARSEVEC SQL/storage/search and ANN integration. A primitive Scanner/Valuer does not implement these database semantics.

### Aggregate for the dirty-WAL / ordinary-index / M9 prerequisite checkpoint
- CGO0 full unit/vet/build, separate full race, complete pinned SQLite3.54 oracle, Linux amd64/arm64 + Darwin arm64 + Windows amd64 CGO0 cross-builds, module verification and empty CgoFiles closure PASSED exit0 at2026-10-10T13:14:43Z. SQLite unit65.146s/race337.220s; oracle testkit226.617s/sqlite76.306s. Original goal and prior read-only/extension fixtures unchanged; newly retained source fixtures match exact manifests. See m8-wal-m9-prerequisites-aggregate.txt.
- The new encrypted-WAL example and final extended-review test arrived after initial package enumeration; final CGO0 focused tests, separate vector/example race, vet, runnable example and current migration CLI passed at13:09:39Z. Full oracle subsequently included the example. The entire production library was already frozen before aggregate compilation. The three-size encrypted-head oracle race passed91.950s. See m8-wal-m9-final-supplement.txt and m8-wal-oracle-race.txt.
- Two CGO0 -trimpath CLI builds were byte-identical, statically linked, SHA2562eb16d98eb3fa8bcac783b577a03c96485af6468bf8bb08c2208de97d749c097. Build metadata explicitly identifies the prior commit plus a dirty working tree, not the future published revision. No new dependencies or restricted physical tests.

### M10: Independent release acceptance
- Status: NOT_STARTED / required independent evidence unavailable.
- Native-OS runtime, security, cryptography, legal/license/API review, large benchmarks and real-duration soak remain separate gates. Restricted physical fault/crash/hot recovery remains BLOCKED/NOT_RUN and must not be relocated.
- Every meaningful checkpoint stays remotely committed and independently backed up. Ordinary results and source plans do not establish full goal acceptance.

### Native platform prerequisite: managed file ownership
- Status: IN_PROGRESS (integrated Linux ownership slice; Darwin/Windows backends remain missing)
- Deliverables: real `internal/vfs` private file/mapping ownership across plain pager probes, DB/WAL/journal/temp/directory opens, WAL-index DMS/slots, and source-only recovery helpers. Linux OFD semantics/SQLite offsets preserved. Portable SHM conversions require explicit release. Mapping retirement precedes DMS and main SHARED release; unresolved cleanup retains resources.
- Key ADR: 0014-managed-native-file-ownership. Exclusive imported-WAL replacement now requires managed role-checked leases; shared-WAL never receives replacement authority. Encrypted container implementation remains separate.
- Code: internal/vfs; internal/pager native_file/pager/tx/commit/session/shared_session/wal/recovery; internal/walindex native_file/index. A plaintext-copy source check now uses its existing managed reader handle.
- Actual focused checks: CGO0 ordinary vfs/pager/walindex tests; separate CGO1 race; scoped vet; pinned SQLite TestTxOracle transaction/lock checks; four-target production package builds; Darwin/Windows test-binary compilation; manual-recovery source compilation only. See docs/evidence/native-ownership-components.txt for exact passes. No physical failure/recovery test executed.
- Evidence boundary: native execution is Linux/amd64 only. Cross-builds still include unsupported non-Linux hooks. Memory-only view-retirement/alignment/ledger models are not native failure-injection or interprocess-memory/durability proof.
- Required next code: Darwin process-inode lock aggregation/deferred-close manager and POSIX/mmap/sync backend; Windows native identity/open policy, exact LockFileEx bookkeeping, DMS, granularity-aware mapping handles, replacement and declared sync policy. Native ordinary interoperability tests follow their actual implementation. M3/M10 restricted/independent acceptance remains open.

### S1 transaction controls and bounded undo
- Implemented: deferred snapshot acquisition at first actual database access,
  IMMEDIATE/EXCLUSIVE, nested/repeated savepoint names, release/rollback-to,
  savepoint-started outer commit, and authoritative native/driver autocommit state.
  Statement and named undo share a prepaid aggregate memory budget, including
  page images, freelist state and retained metadata. Native APIs participate in
  the same first-access path; constant SELECT stays snapshot-free.
- Focused evidence: guarded CGO0 SQL/pager/sqlite/driver tests, all SQL tests,
  affected-package vet/build, CGO1 races and pinned transaction oracle/race passed.
  Independent review exercised name substring ownership, aggregate retained undo,
  BUSY retry, failed lazy replay cleanup, cancellation and extension restoration.
  See s1-transactions component/race/oracle evidence and transactions.md.
- Limits: active transaction Rows still block mutations, named controls and
  Commit; cursor relaxation remains required code. Connection-level SQL counters
  are absent. Combined-tree aggregate/publication is recorded separately; none of
  these checks qualifies physical interruption or independent release acceptance.

### M9 checkpoint E: BIT SQL and exact ranking
- Implemented: BIT(1..64000), strict GSB persistence/default/mutation/driver paths,
  Hamming/<~>, Jaccard/<%>, bit accessors, dense subvector/binary_quantize, explicit
  native exact search and eligible bounded SQL heap ranking with packed-byte
  quotas, distinct representation/metric diagnostics and deterministic ties.
- Focused guarded repeated unit/race/vet/build, preserved dense regressions,
  three ordinary official-oracle cases and executable example passed. Review
  fixed external trailing-default NUMERIC affinity and strengthened statement
  rollback assertions. See m9-bit-sql component/oracle/review/example evidence.
- The same-snapshot rerank example measures recall@1=0 with one Hamming candidate
  and 1 with two; exact dense rescoring cannot restore an omitted candidate.
  Half/sparse SQL and all extended-type ANN remain missing code. BIT's legal
  SQLite NUMERIC affinity requires canonical BLOBs from external writers to
  preserve leading zeros. Full combined-tree gates remain separately recorded.

### Corrected ordinary-scope durable verification
- Exact c7764896c541c4ed6177288ec8f9625828093ade and Library
  libfile_e94b5944a3a481919e03d83293617671 verified. CI38056050522 passed.
  Byte-identical clean restore passed guarded unit/vet/build/race and complete
  pinned SQLite oracle, exit0 at13:46:19Z; restored tree stayed clean. The earlier
  namespace-test findings and their historical scope qualifications remain.

### M9 checkpoint B: typed offline SQL converter
- Delivered: bounded PostgreSQL scanner/AST/catalog binder, typed source-span
  rewrites, deterministic complete-script reports, local executable and separate
  review-only PostgreSQL output, runtime bind contracts, CLI and real local HNSW
  conversion example. Unsupported statements have no converted SQL/edits; an
  unsupported member blocks the whole apply-ready script. See migration-converter.md.
- Review fixed reproduced alias/result-label, primary-key NULL, scalar/source
  type-width, arithmetic/error, LIMIT, collation, type-shadow, bind-inference and
  decimal-underflow hazards through explicit validation or narrower refusal.
  Nil LIMIT was already guarded; an additional regression confirms that contract.
  A inspector/parser semantics and dense GSV bytes are unchanged.
- Actual final isolated snapshot tree8e4808e09fcd45666bb2efb60d96bbf0033eeb91
  passed guard, complete migration/CLI/example CGO0 tests0.185/0.055/0.021s,
  vet/build, separate race3.031/1.282/1.109s, executable example and four target
  cross-builds. All snapshot file hashes remained unchanged; exit0 at13:56:16Z.
  Final converter fuzz6359 and scanner/parser33419 executions passed; independent
  scanner fuzz15114 passed. See m9-converter components/review/isolated evidence.
- Example source29464448... produces raw local SQL23ed9155..., IDs3/1/2/4 with
  L1 distances0/5/6/NULL and normal reopen. Query EFSearch64 remains explicit
  application metadata, with no fake SQL setting or transaction wrapper.
- Limits: no live PostgreSQL execution, resumable data importer, remote application
  deployment or extension-transition bundle. General PostgreSQL compatibility is
  not claimed. Full shared core aggregate/CI/restored evidence and physical or
  independent release acceptance are separate; this is a conditional code slice.

S1/BIT/Linux-ownership frozen-source full aggregate completed exit0 at14:02:38Z,
including guarded CGO0 unit/vet/build, CGO1 race, pinned oracle, four cross-builds,
modules/no-CGO closure and complete unchanged-tree/fixture verification. SQLite
unit81.243s/race406.026s; oracle testkit301.806s/sqlite97.002s. The exact durable
718e3109fccc0f4d728d6f95b555cf4fd6e3e282 and Library
libfile_a2e7be4289308191896d1abf730de740 were already verified; its restored full
checks and CI38057793162 are tracked independently.

### M9 checkpoint F: HALFVEC SQL and exact ranking
- Delivered: HALFVEC(1..16000), distinct canonical GSH persistence/nullable
  bindings/defaults/mutations, closed-metric DB/Tx.HalfSearch and bounded exact
  SQL ranking, explicit dense/half conversions and three precision-loss reports,
  half subvector/quantization, and bound representation tracking through supported
  aliases/value-preserving expressions. Mixed dense/half signatures reject.
- Focused guarded component tests x3, exhaustive half primitives,76 retained
  published SQL fixture cases, dense/BIT regressions, race/vet/build and four
  official SQLite oracle cases x3 passed. Independent review added driver,
  resource/ranking/snapshot and six-way page-size/mode evidence. It fixed valid
  explicit replacement after an external numeric half default while retaining
  native CREATE strictness. No live PostgreSQL was run.
- Same-snapshot example: dense nearest row20; one rounded half candidate picks
  row10 and recall0, two reranked candidates recover row20 and recall1. Query
  conversion changes2 components, maximum absolute error0.00020003318786621094
  and relative error1. Exact rescoring cannot recover an omitted candidate.
- Evidence: m9-half-sql components/review/source hashes/example, half-vector.md.
  Frozen full aggregate tree98d857b4e07b755fa9bd67346905785135fba2ef passed
  guarded CGO0 unit/vet/build, CGO1 full race, complete pinned oracle, four
  cross-builds, module/no-CGO and unchanged source/fixture checks exit0 at
  2026-10-10T14:25:15Z. SQLite unit99.307s/race454.551s; oracle
  testkit422.888s/sqlite126.083s. Later features were excluded.
  Sparse SQL, all extended ANN and restricted/independent release gates remain open.

### Plain native Darwin/Windows and M8 shared encrypted increment
- Delivered: real Darwin managed inode-lock aggregation/deferred descriptor close,
  checked Windows/amd64 handles/LockFileEx intervals/aligned mappings and explicit
  publication/sync policies. Linux OFD implementation bytes remain unchanged.
  Independent ownership source review plus portable memory laws pass; four-target
  builds/test compilation pass. Native Darwin/Windows tests were NOT_RUN. Windows
  default cross-vet has one disclosed MapViewOfFile unsafe-pointer diagnostic.
- Delivered: explicit EncryptionWALShared/layout3 on Linux, immutable authenticated
  reader selections, independent single writer, stale snapshot promotion,
  provider/epoch reuse, lazy/savepoint/pool semantics, latest-head checkpoint,
  single-selection verify/copy/export, quiescent rekey and explicit legacy upgrade.
  Old mode0/1 and layout1/2 APIs and bytes remain unchanged. Operation admission accounts for
  simultaneous old/new graphs, descriptors and retained issuance history.
- Independent review fixed pre-allocation graph accounting, missing stable-lock/
  namespace checks at publication and stale checkpoint metadata. Ten named review
  tests and focused correct-key intact-file component/race tests pass, including
  concurrent publisher/checkpoint while another operation validates/copies its
  older head. No restricted disk/namespace/native-failure workflows ran.
- Evidence: native-platform-components/review, native-windows-components,
  m8-shared-components/race/review/source manifests; ADR0015/0016 and runnable
  examples/encrypted-shared. Lead full aggregate on immutable tree
  ba85a097c9c10cf96e0bc9049d796887cfd3ba80 is tracked separately; no terminal
  aggregate/remote-CI/restored-test success is asserted before its result. Frozen
  full CGO0 unit/vet/build already passed (sqlite86.528s); full race/oracle remain
  running at this checkpoint.
- Remaining code: non-Linux encrypted coordination; failed-open mapping-retirement
  retry lifecycle; broader recovery/SQL/extended ANN/migration. Retained history,
  old-key decryptability and no external freshness remain disclosed. Physical
  durability and independent native/security/crypto/legal/soak gates remain open.

S1 exact Library-restored full guarded checks passed exit0 at2026-10-10T14:24:14Z;
CI38057793162 also passed. M9 B CI38058308322 completed SUCCESS at14:16:31Z.
See checkpoints.json and s1-library-restore.txt for distinct evidence scopes.

### M9 checkpoint G: SPARSEVEC SQL and support-bounded exact ranking
- Delivered: SPARSEVEC dimensions1..1e9, at most16000 nonzeros, canonical distinct
  GSS persistence, defaults/DML/driver adapters, DB/Tx.SparseSearch and SQL exact
  heap ranking for L2/negative-IP/cosine/L1. Work and memory depend on supports,
  never declared dimensions; conversions are explicit and bounded.
- Pinned56 parse and40 metric cases pass; repeated component/driver/storage
  tests and separate race/vet/build pass. Official SQLite intact-file interop
  validates canonical overflow values at512/4096/65536-byte pages, external legal
  TEXT/GSS/default values, shared snapshots and normal rollback/reopen.
- Independent review added ten tests for strict identity/bytes, billion-dimension
  rankings, quotas/laziness/cancellation/rollback and unsupported boundaries.
  It found and fixed combined heap-retention plus support-scratch admission before
  candidate decoding; exact-boundary/zero-allocation-on-refusal tests pass.
- Evidence: m9-sparse-sql-components/review/source hashes/example and
  sparse-vector.md. Lead isolated tree57b02e0fa1f99f4c6a4f470c7741624cce1ba977
  focused validation passed exit0 at2026-10-10T14:35:25Z: full production
  vet/build, dense/half/bit/sparse component regressions and separate race,
  named extended-type pinned-oracle checks, four cross-builds/modules and
  unchanged complete source inventory. Later importer/SQL/platform edits excluded.
  No live PostgreSQL execution or sparse ANN is claimed; all extended ANN and
  restricted/independent release gates remain open.

### M9 checkpoint C: resumable native CSV/NDJSON import
- Delivered: strict bounded versioned manifest and record framing, exact field/
  NULL/type maps, direct int64 and binary32 parsing, new-destination reservation,
  native batched DML/provenance/rejection/cursor atomicity, deliberate committed
  stop and normal reopen/resume, exhaustive export/database verification and
  deterministic canonical hashes. CLI defaults dry-run; writes require --apply.
- Genuine HNSW/IVF builds and exact native config/source identity follow data
  completion; partial index progress and semantic failures remain explicit pending
  states. No conflicting object is overwritten. Quarantine distinguishes rejected
  PKs from the verified absent-key set; duplicate rejection does not invent loss.
- Seventeen independent review tests pass CGO0 and separate race, with fixes for
  memory/hash allocation, explicit index method and state/identity consistency.
  Final isolated source3f83ee9453319a71b0db3c78a098a7855a2986ff excludes D/S2/
  Windows-encrypted changes. Final source delta clones retained CSV text; a memory
  regression failed at3,412,304 retained bytes before the fix and passes after it.
  Earlier full focused unit/race/vet/build/example evidence and final delta checks
  are preserved separately, with unchanged archive hashes.
- Synthetic64-row CSV/NDJSON outputs share canonical hash
  recorded in m9-import-example.json; no general recall promise follows.
  Both real indexes achieve recall@4=1 on its seeded bounded query. Separate
  independent256-bit distance comparisons preserve per-PK tolerances and treat
  numerical cosine ties explicitly, without deleting cases or changing metrics.
- Evidence: import-pgembedding.md, m9-import components/review/final-delta/source
  hashes/example/retention-before-fix. No live PostgreSQL, source-schema behavior,
  crash/interrupted-commit or physical durability acceptance is claimed. Required
  transition templates, extended ANN/broader SQL/recovery and independent gates
  remain separate work.

Native/shared-encrypted immutable treeba85a097c9c10cf96e0bc9049d796887cfd3ba80
full guarded CGO0 unit/vet/build, CGO1 race, pinned SQLite oracle, four cross-builds,
module/no-CGO and unchanged full-source/fixture checks passed exit0 at
2026-10-10T14:44:18Z. See native-shared-aggregate.txt. This excludes newer sparse,
importer, expression and Windows-encrypted code; native runtime remains Linux.
