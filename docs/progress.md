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
- Status: experimental snapshot, dirty-page WAL and Linux shared encrypted sessions implemented; Windows/amd64 coordination is implemented but native runtime unverified; Darwin encrypted coordination and Windows shared rekey remain required code. Physical tamper/recovery and independent security acceptance remain open.
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
- Status: checkpoint A inspector, checkpoint B typed SQL converter, checkpoint E BIT, F HALFVEC and G SPARSEVEC integration plus extended memory primitives implemented. Resumable mapped import and offline retained-array/shadow-column transition bundles are implemented; full-table/online transition and all extended ANN remain incomplete code.
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

### S2 scalar prerequisites: CASE/CAST/BETWEEN/IN and NULL ordering
- Delivered: explicit bounded AST/parser/evaluator nodes, lazy CASE/single-value
  BETWEEN, SQLite CAST affinity/storage-class conversion, IN-list empty/NULL/
  comparison rules, SELECT ALL and NULLS FIRST/LAST. Shared walkers and immutable
  boolean binding traverse new trees; source labels and parameter numbering stay
  stable. This is scalar preparation, not the full relational S2 architecture.
- Independent review found/fixed three groups: eager error evaluation of constant
  IN RHS sets longer than2; typeless physical-column identity shadowed by typed
  aliases; and CAST typename contextual/forbidden keyword classification. All
  failing regressions are preserved in separate evidence. Typed dense/half/bit/
  sparse identity through CASE and supported TEXT/BLOB casts is tested; no private
  codec is reinterpreted as another representation or silently decoded by CAST.
- Final source compares18,413 scalar values and151 keyword acceptance cases with
  pinned SQLite, plus28 ordinary rollback/shared-WAL queries and integrity checks.
  Focused CGO0 unit/vet/build, separate CGO1 race, native/driver/DDL/default/undo/
  binding/resource/cancellation tests pass. Independent review added5,528 value
  comparisons and151 keyword cases within those scoped totals.
- Evidence: sql-expressions.md and s2-expressions-*/s2-expression-independent-review.
  Lead full aggregate uses immutable treedb470d2ec208a690202253db840620b219a6abdc,
  including published sparse/importer but excluding later D templates, Windows
  encrypted coordination, plain metadata repair and typed ANN. Terminal aggregate
  results are tracked separately, never inferred from focused evidence.
- Required code remains: full scoped relational binder, composable pull operators,
  shared nested query budgets, COLLATE, JOIN/subqueries/aggregates/CTEs/windows,
  broader constraints/DDL and planner/storage work. Restricted physical and
  independent/native release gates stay open.

S2 isolated full CGO0 unit/vet/build has passed by15:01:56Z; full race/oracle
phases remain running. See s2-expressions-lead-unit.txt. No terminal aggregate
or restored/remote result is inferred before observation.

### M9 checkpoint D: review-only transition bundles and CLI
- Delivered: strict bounded explicit options, catalog/identity-bound offline
  generator and13 deterministic artifacts. Retained real[]/cast and typed shadow
  column, single transaction and explicit resumable batch plans, global hnsw AM
  collision checks, capability/dependency gates, explicit index retuning and
  exhaustive binary32/count/NULL verification references are represented.
- All mutation and post-install SQL remains commented. Approval gates remain
  unresolved/blocked; the CLI only emits JSON review artifacts. It has no server,
  SQL execution, apply or resume path. Optional reports use exclusive creation
  and can remain partial on write errors; atomic publication is not claimed.
- Independent review corrected float32 verification alias capture and refuses
  known inherited parents. Fourteen core review tests and separate CLI/decoder
  review pass. Missing inventory remains a gate; no observed absence is invented.
- Full-table/dependency recreation, online/dual-write operation, privilege changes,
  live PostgreSQL/Neon capability verification and interrupted-operation acceptance
  remain open. Shadow-column copy does not satisfy full-table copy requirements.
- Evidence: migration-transition.md, migration-transition-cli.md, standalone
  example with two golden bundles, core and CLI independent-review reports. Lead
  first source13f6b73b2f0a87fe6fe6410736465d5b3b7c8242 passed focused migration/CLI
  unit/race, full vet/build and four cross-builds at15:03:29Z. Final isolated source
  415de5f50dbb6060befb52955208cbf5aeec7588 excludes unpublished platform/ANN code;
  focused migration/CLI/example unit/race, full vet/build, four cross-builds,
  synthetic CLI output, modules and unchanged source inventory passed exit0 at
  2026-10-10T15:10:56Z. No live SQL execution occurred.

S2 final immutable tree db470d2ec208a690202253db840620b219a6abdc completed full
guarded CGO0 unit/vet/build, separate CGO1 race, pinned SQLite ordinary oracle,
four cross-builds, module/no-CGO and unchanged source/fixture inventories at
2026-10-10T15:15:24Z, exit0. See s2-expressions-aggregate.txt. Its exact Library
restore focused checks are separate; one incorrect oracle environment variable
failed before queries, then the corrected invocation passed without source edits.
D exact Library restore focused unit/race/vet/build and identical synthetic text
generation passed15:14:56Z. Remote CI terminal states are recorded separately.

### Native Windows encrypted backend and public metadata correction
- Delivered: managed retained-parent/child handles, native one-component relative
  open/publication, full identity checks, exact lifecycle/writer/capture lock
  ledger and strict case-family/name accounting. Shared mode2 supports readers,
  writer, refresh/checkpoint and legacy upgrade. Protocol/storage bytes unchanged.
- Windows shared rekey remains unsupported before provider acquisition; no
  unlock/relock weakening. Native Windows/Darwin execution NOT_RUN. Directory
  Sync supplies no namespace barrier; restricted physical gates remain open.
- Corrected public plain VFS/pager/WAL-index metadata and Windows Open prelude:
  bounded owner reservations, immutable complete IDs, terminal uncertain-close
  tombstones, retained parents, preserved read/scan/map/close error causes.
  Encrypted public paths forward to the same managed canonical backend.
- Independent reviews repaired backend path/lifetime/accounting defects and
  metadata poison erasure. Source/model/ordinary Linux checks and cross-builds
  are scoped evidence; default Windows VFS cross-vet remains non-clean with its
  disclosed native mapped-address warning. Earlier logs are not retroactively
  promoted to cover the missing public metadata chain.
- Evidence: native-encrypted-windows-*, encrypted-windows-independent-review,
  native-plain-metadata-*, native-windows-metadata.md. Combined isolated source
  927ebc2a40d1ff4926914aca6777b7a236fc8992 excludes future Darwin/ANN changes; lead
  focused VFS/WAL-index/encstore/pager unit/race, public/encrypted/shared
  lifecycle unit/race, full Linux vet/build, transaction oracle, five cross-builds,
  six Windows package test-binary compilations, Windows encstore vet and unchanged
  source inventory passed exit0 at2026-10-10T15:19:47Z. Failed-open shared-session retirement
  retry remains a separate code gap. No full M3/M8/M10 acceptance follows.

### Windows mapping-retirement source correction

The unpublished candidate75552c9ecbf7788b581f154e2aaa4b134c37d771 was withdrawn
before any branch update. A subsequent source review found the historical plain
Windows mapping-object HANDLE could be passed to CloseHandle again after a prior
close error; the failed MapView acquisition-cleanup path had the same issue.
A close error does not prove the numeric handle is safely retryable. No native
fault test was executed, and no data-loss incident is asserted.

A portable private retirement state machine now backs both Windows paths. Its
first uncertain close becomes a terminal retained tombstone; repeated cleanup
returns the bounded retained cause without closing that handle or the exposed
view again. Successful object close followed by failed view unmap remains a
represented view retry. DMS/file protection and live Bytes are preserved while
ownership is unresolved. Exact locks, identity and mapping granularity do not
change. Independent three-path source review, memory/race and Windows compile
checks are recorded separately. Earlier metadata/native review evidence did not
prove this mapping path safe and is retained as historical scoped evidence.

Replacement Windows source tree8f9025bccbfcdc65458c276d683a1485b5a9290e completed
focused VFS/WAL-index/encstore/pager CGO0 unit and separate CGO1 race, ordinary
public/encrypted/shared lifecycle checks, full Linux vet/build, pinned transaction
oracle, five cross-builds, six Windows package test-binary compilations, scoped
Windows encstore vet and unchanged source inventory at2026-10-10T15:40:52Z, exit0.
See native-windows-corrected-lead-checks.txt. Default Windows VFS vet remains
non-clean with its mapped-address diagnostic. Remote publication, exact Library
restore and exact-SHA CI are separate pending steps, not inferred passes.

### Native Darwin encrypted coordination and failed-open retirement

Implemented shared plain/encrypted inode ownership, process-lock aggregation and
deferred close, native relative publication, direct bounded directory parsing,
and conservative ASCII target/inventory policy. Darwin Unicode inventory remains
required code; native runtime is NOT_RUN. A later poison-retention correction
latches the first stored cause instead of recursively retaining retry history.

Shared-WAL unsuccessful opening reserves one of64 groups before acquisitions,
collects nested managed/terminal owners, and retires child/index/DMS before WAL
and main. RetryOpenCleanup discovers all groups before bounded serialized retry.
Successful opens release the group; no background cleanup, eviction or unsafe raw
handle retry is introduced. Independent source/memory/ordinary/race checks pass.
Successful-session Close ordering, public DB.Close retry and driver disposal still
have required ownership fixes; this increment does not claim those paths covered.

Prior Darwin22-path source manifest remains historical; registry.go is explicitly
superseded by the bounded poison delta. Combined immutable source49f2749ffb838f936cc6066fb3496a4914cb1626,
including corrected Windows and reviewed typed ANN, completed focused native/ANN
unit/race, full vet/build, typed/transaction oracle, six cross-builds, module and
unchanged-source checks at2026-10-10T15:49:53Z. That is source-composition evidence,
not an exact restored-commit or native-platform acceptance claim.

### M9 typed persistent HNSW/IVF

Delivered genuine graph/list algorithms for half/bit/sparse supported opclasses,
version2 typed identities, original payloads, native/driver/SQL selection, bounded
work, cancellation, DML/savepoint/delete/reindex/reopen, and explicit fallback.
Old dense production/test paths and public layouts are preserved; independent
bidirectional legacy fixtures verify dense v1 bytes. Review found/fixed sparse
TEXT explicit-zero predecode work/scratch bypass; raw entry charges now precede
decode while preserving WHERE0 laziness. Sparse IVF/IVF L1/Jaccard unsupported.

Immutable typed source27de0d52d2ab16d17d4e78484046e940e9a53b7a completed full
CGO0 unit/vet/build, separate CGO1 race, full pinned ordinary SQLite oracle, four
cross-builds, modules/no-CGO and complete source/fixture hashes at15:51:11Z exit0.
Source49f2749ffb838f936cc6066fb3496a4914cb1626 separately passed focused combined
native/Darwin/failed-open/typed integration at15:49:53Z. Neither replaces exact
commit CI/Library restore, native-OS or restricted/independent acceptance. Fixture
HNSW recall@10=1 is dataset-specific; IVF one-probe recall spans0.30–0.90.

### Resumed durable phase saves and exact restore checks

On2026-10-10, explicit approval allowed conditional development checkpoints to
continue. Corrected Windows ea6591b, Darwin/failed-open b8c5ffa and typed ANN
efe40d7 were read back from main and exactly mirrored locally. Each full-history
bundle was persisted to Library and byte-compared, clean-cloned/fsck checked
with unchanged goal hash. Exact restored focused unit/race/vet/build and scoped
oracles passed; see checkpoints.json for scope, IDs and timestamps. The initial
Darwin encryption regex selected zero tests; a corrected explicit oracle ran and
passed. The initial no-argument typed example printed usage only; a corrected
disposable-path run passed and matched frozen output bytes. No missing execution
is represented as evidence. Windows exact-SHA CI38073597982 completed SUCCESS
18:04:41Z; the other two runs are separately monitored.

Actual further code work is in progress: native shared successful-session close
retirement, scoped relational JOIN execution and full-table migration plans.
Those unpublished changes are excluded from the preceding restored commits.

### Native shared-WAL successful-close retirement

Implemented separate1024 live/in-flight/pending admission before public native
metadata/acquisition;64 failed-open admission remains independent. Groups retain
minimal managed owners even if database/sql ignores Close errors. New explicit
completion adapters distinguish retired diagnostics from unresolved ownership.
Nested owners retire before index/DMS, WAL and main; no protecting parent is
released below a pending child. Logical DB.Close invalidation and checkpoint run
once, while repeated Close/RetryCloseCleanup can continue represented cleanup.
PendingCloseCleanups returns immutable bounded tokens without cleanup/sweeps.
Connector no longer manufactures a late-context cancellation disposal path.

Independent16-path review and isolated focused CGO0 unit, separate CGO1 race,
scoped vet/build and Linux-arm64/Darwin-arm64/Windows-amd64 compilation passed.
Lead close-only sourcebc3ddb29edab8a1d8e9ab599600dac8e6d114121 (efe40d7 plus
those16 paths, excluding future relational/migration work) completed full CGO0
unit/vet/build18:13:25Z. Full race/oracle is still running at this record, not
inferred complete. Runtime outside Linux and all restricted native failures stay
NOT_RUN. Rollback/exclusive-WAL/encrypted constructors and lifecycle loss remain
actual required code work; no full M3 or release acceptance follows.

Additional bounded pure-memory fuzz ran against exact restored69008bf at
2026-10-10T18:20:06Z–18:23:13Z: typed-config canonicality1,456,682 executions,
typed-value codec1,724,094 executions and necessary LIKE grams150,698 executions,
60 seconds each with two workers, all PASS and unchanged restored Git tree. The
recorded zero-throughput intervals are preserved; no throughput or coverage
completeness inference is made. See resume-memory-fuzz.txt. These byte/string
models invoke no database/native-fault/crash workflow and do not qualify recovery
or independent security/release gates.

Native shared-close frozen sourcebc3ddb29edab8a1d8e9ab599600dac8e6d114121
completed full guarded CGO0 unit/vet/build, separate full CGO1 race, full pinned
ordinary SQLite oracle, four cross-builds, modules and unchanged source inventory
at2026-10-10T18:25:56Z, exit0. Exact remote commit69008bf CI38075132617 completed
SUCCESS18:24:50Z. Exact Library-restored focused checks are recorded separately.
No restricted physical or native non-Linux qualification follows from these passes.

### Scoped relational JOIN execution

Implemented named-rowid-table INNER/CROSS/comma/LEFT pull joins, scoped aliases,
qualified names/stars, affinity-aware ON, correct null extension and one root
snapshot/Tx lease. No index/ANN join pushdown or unbounded inner materialization
is claimed. New per-context work/memory limits charge binding, pre-copy BLOB
retention, dense page-cell arrays, record reads, scalar coercion, sorting and
retained rows, with cancellation before allocation admission. Existing public
Options/QueryPlan/QueryStats layouts and dense/vector disk identities remain.

The 27-path final manifest4dd07287c3687ee466b51b7d6c343261aeb997ff81c01f75bdfe0188b85a9397
is immutable, reconstructed on69008bf without concurrent encrypted/migration
changes. Independent review passes seven suites, including604 pinned SQLite
comparisons, unit/race/vet/build and selected B-tree/legacy regressions. Concrete
Boolean binding/labels, quota/pre-copy/work and retained-result defects found
during review were fixed and initial failing evidence retained. The first lead full suite on
source2f1ab3df86cdf5aaf97143d6ad53d186da836b3f failed because a historical
negative fixture still expected a now-implemented JOIN to fail. The corrected
fixture retains all rejection assertions, uses unsupported USING for that
negative case, and adds an exact positive JOIN result; independent delta review
and focused tests pass. No production byte changed. The28-path formatted final
manifest is c64078230751713f951fcc3dd1a71f4470657d2eaf1e18e82035c89b826abb0b,
source treead73b8d6425cd86d1c15caccb0f858048df63ef9. Final aggregate evidence is
recorded separately in relational-lead-aggregate.txt. Full CGO0 unit/vet/build
passed18:39:10Z (SQLite78.121s); full race/oracle continuation remains running
at this checkpoint. A prior preflight had
correctly stopped on a changed manifest; no test ran against misattributed bytes.
All initial failure logs are preserved.

This is PARTIAL SQL work: USING/NATURAL/RIGHT/FULL, derived/parenthesized sources,
forward ON references, subqueries/aggregates/CTE/compound/windows and index join
probes remain actual code gaps. Inherited eager binary AND/OR still reports
errors for predicates SQLite folds/short-circuits; documented counterexamples
are not passing equivalence claims. No physical recovery, native non-Linux or
independent release acceptance follows. See relational-sql.md and the complete
relational-independent review evidence.

### M9 bounded full-table migration review generator

Implemented a distinct versioned PG16 whole-table copy/swap evidence contract and
CLI route: typed all-column creation/copy/comparison, retained backup/name swap,
recreation of supported immediate PK/UNIQUE/scalar CHECK/default/B-tree/ANN/owner/
ACL/comment contracts, and explicit same-sequence ownership transfer. Exact
row-type/TOAST/namespace/legacy support-row dependency identities and database-wide
event-trigger coverage are required. Unknown/unsupported facts block every
mutation proposal. Every mutation stays commented and no external SQL was run.
Existing catalog v1/retained-array/shadow API and golden bytes remain unchanged.

Independent review found eight semantic/gating issues despite the initial test
PASS, including integer-minimum cast precedence, archive constraint collision,
missing mandatory identity edges, actual PG16 PK/UNIQUE connoinherit semantics,
pinned float_ops and event-trigger inventory. All were fixed and independently
retested against the corrected33-path immutable overlay SHA256
ecd01b2650c9eb54e013fb7e2c40ef15aa72c4056071d62a3c8ddbb9af1349dc.
The review independently verified850 files against its baseline/owned paths and
passed unit/race/vet/build, exported-API probes and14-artifact comparisons.
Initial findings/logs remain retained; source documentation is not a PG executable
oracle. See migration-full-table.md and m9-full-table-independent-review.md.

Lead integration source0d65b3618e47ade6e958627c7ab26869b8fcfa0b combines only
those33 migration paths with durable relational8f75ccf. Exact commands/results are
in m9-full-table-lead-checks.txt. Full migration/CLI/example package unit/race,
whole-tree vet/build, standalone example/CLI byte parity, four crossbuilds,
modules and unchanged inventory passed18:44:44Z, exit0.
Native recovery/platform/Boolean work remains
excluded. FK/views/functions/triggers/RLS/partition/custom expression/collation/
ACL chains/other unsupported dependencies and online/batched cutover are actual
code gaps. Live PostgreSQL/Neon, source completeness, capacity/privilege checks,
application cached-OID handling and independent release acceptance remain open.

### Full ordinary oracle coverage correction discovered after JOIN save

The relational8f75ccf CI38076830650 completed SUCCESS18:52:34Z for its actual
default unit/vet/build/race scope; exact restored focused relational checks also
passed. The separate full sourcead73b8d race passed (SQLite412.237s), but its
full tagged oracle failed at the historical TestOracleErrorsAndResources JOIN
negative, which still expected a now-supported form to return ErrUnsupported.
The original failure is preserved in relational-lead-final-precorrection.txt
and exit1. That whole oracle run is FAIL, not a partial or inferred PASS.

The existing workflow did not build the pinned CLI or invoke test-oracle.sh;
that difference explains why this tagged assertion was absent from CI. A
separate complete ordinary oracle CI job and an official positive JOIN comparison
plus truly unsupported USING negative are being added. No restricted manual
namespace/recovery/native-fault source is enabled. Corrected exact source must
be rerun before claiming that broader gate.

### Encrypted acquisition/retirement ownership correction

Implemented pre-native admission for1024 opening/live/pending scopes and16384
common managed leases; native platform bounds also apply. Scope owners retain
minimal native resources/dependencies and bounded diagnostics, independently of
Store/DB/Rows lifetime or database/sql discarding Close errors. New public
ErrEncryptedCleanupPending, RetryEncryptedCleanup and PendingEncryptedCleanups
provide explicit discovery/retry. Whole retirement preserves child, lifecycle
and directory barriers; live retry touches only previously requested children.
Consumed diagnostics differ from unresolved or terminal ownership. No numeric
terminal handle is retried, no automatic native cleanup is performed, and
admission consumes only already-published atomic completion proofs.

Logical closure clears owned key-bearing state once, invalidates SQL and
attempts optional checkpoint once; BUSY before closure preserves options.
Constructor/probe/inventory/copy errors are adopted before child/protector
retirement. Darwin delayed retirement and Windows deferred directory pins have
explicit completion laws. Cipher/format/KDF/provider and lock-byte contracts
are unchanged. Opaque application error objects can retain arbitrary references,
so no absolute transitive secret-erasure or total-RSS claim is made.

Independent v3 source manifest25d5541df56656535db32ff1e344e3888869e1e1c84efd49f828b823434a9fa8
passed scoped unit/race/vet/build and four target builds/test compilation after
real BUSY-options, nested-error and admission-status defects were fixed. The
original independent v1 failures remain. Source508732922205872b7a8738b471b236fcf8ecb096
combines only that33-path correction with durable883999c; its full CGO0
unit/vet/build passed18:54:55Z and full race passed (SQLite363.426s). Its full
tagged oracle finished FAIL at the already discovered obsolete JOIN rejection
(testkit297.585s); the original log and exit1 are preserved. It does not qualify
the corrected complete gate.

Exact corrected sourcecd80bbc80228e1970f8d7e465799cc674eede3fb additionally
contains only the reviewed ordinary-oracle CI/test correction; unrelated
Boolean/affinity/range/plain-Reader work is excluded. Its complete CGO0
unit/vet/build passed19:09:35Z and complete pinned tagged ordinary oracle passed
19:13:56Z. The complete frozen aggregate finished PASS/exit0 at19:21:45Z: full race
(SQLite392.800s), five target cross-builds, modules and unchanged inventory also
passed. Hosted two-job CI and exact restored commit results will be recorded
separately when terminal, without relabeling prior snapshots. A supplemental reviewed-source Library archive protects the
unpublished encrypted patch while that gate runs; it is not a remote milestone
or final delivery. Plain rollback Reader/Tx/import-source and exclusive-WAL
ownership remain required code. Native failure/runtime and M3/M8/M10 acceptance
remain NOT_RUN. See encrypted-cleanup.md and its independent review.


## Predicate, affinity and ranking correction checkpoint

Source `816e84d58da57b537376bcff461776474c1759a4` combines the independently
reviewed Boolean-context compiler, comparison-affinity repair and corrected
ranking predicate entry points on durable ed4a38e. Scalar expressions remain
eager where SQLite requires it; WHERE/ON/searched CASE use context-sensitive
three-valued branching. Constant startup and fixed-column substitution preserve
binding, affinity and bounded compilation/runtime work. Declared BLOB affinity
is distinguished from no affinity when applying numeric/text conversions.

The first combined source1336a5d failed three unchanged ordinary tests: false
WHERE still decoded sparse values/defaults, and WHERE0 admitted an ANN result.
That genuine failure is preserved. The repaired common predicate startup now
covers cached terminal EOF/errors, fallback memory rows and typed/vector/ANN/
similarity entry points before unnecessary value decoding or graph work. The
original assertions remain unchanged; owner and independent negative controls
fail on the old source and pass on the repaired source.

Exact frozen complete CGO0 unit/vet/build passed2026-10-10T19:30:17Z; complete
pinned ordinary SQLite3.54 oracle passed19:39:38Z; complete race (SQLite396.812s),
five target cross-builds, module verification and every frozen hash passed with
exit0 at19:47:07Z. Independent affinity comparisons include10,128 owner and2,016
additional queries with full value/type checks. Historical broader Boolean
matrix failures remain recorded; unquoted Boolean alias/LEFT-ON provenance,
rowid-range planning and aggregate extensions are separate unpublished work.
This scoped correction does not establish broad SQLite equivalence or M10.

The preceding exact encrypted checkpoint ed4a38e4738a6906027a87cf42dc9f9ac84bc32b
has both complete ordinary and pinned-oracle CI jobs SUCCESS in run38079679118
at19:37:11Z; its exact Library-restored full unit/vet/build/oracle, focused race,
five cross-builds and clean-tree checks passed19:38:52Z. This records terminal
evidence without changing the earlier pre-correction failures.


### Bounded rowid planning, identifier provenance and global aggregates

This checkpoint integrates independently frozen and reviewed components, not a
claim that broad SQLite SQL is complete. Native signed forward/reverse rowid
interval cursors now feed real conservative inequality plans and proved rowid
order elimination. Independent whole-query page evidence is6 versus829 calls,
including planning; LIMIT0 and EXPLAIN still perform metadata planning. Secondary
inequalities/covering/general order/JOIN pushdown/statistics remain separate code.

TRUE/FALSE identifier parsing and SQLite's scoped direct LEFT-ON result-alias
provenance are preserved through immutable binding. The global aggregate profile
implements COUNT/SUM/TOTAL/AVG/MIN/MAX with bounded state, first-root snapshot and
native SQLite-compatible arithmetic/storage classes. GROUP BY/HAVING, DISTINCT,
windows, bare-column aggregate projections and broader ordering remain code gaps.
Independent interaction probes cover these components together; each component's
initial failures and fixes are retained in its own immutable evidence.

Initial combined8d7a0f failed two unchanged grams scan-reason assertions. A real
rowid planner diagnostic defect was fixed without changing those assertions:
rejected order-only optimization leaves the existing plan intact, and genuine
range rejection retains the preceding fallback explanation. Corrected1e413b4
then passed full ordinary unit/vet/build but its full tagged oracle failed one
obsolete sorting-quota assertion. Original SQL SELECT * FROM edge ORDER BY id
now has a complete official value/type and rowid-plan positive at MaxSortBytes1;
ORDER BY id+0 preserves an actual sort-quota negative and normal-quota official
comparison. This is not removal of the original statement or its resource test.
Both full-run failures remain exit1, and the one-path assertion correction was
independently reviewed on exact final source48370bbe2dc585c1b45821fbc360c525d156a9f5.

That exact final source passed complete CGO0 unit/vet/build, complete pinned
ordinary SQLite3.54 oracle, full race, five target cross-builds, modules and full
inventory with exit0 at2026-10-10T20:23:10Z. Exact commands/output are sql-final-commands.txt
and sql-final-lead-checks.txt. All published non-document bytes equal this tested
source. Plain ownership, secondary-range, grouping and new journal parsing code
are excluded. CI and exact restored commit results will be recorded separately.

Previous c89c88e74ec38adff2c752016cb07a96e90edbb3 completed CI38081496897 SUCCESS
20:03:43Z (ordinary job114299164359 and oracle job114299164537); its exact Library-
restored focused SQL/driver normal/race/oracle, full vet/build, five builds,
modules and clean tree passed19:54:46Z. No conditional checkpoint qualifies native
Windows/Darwin runtime, physical failures/recovery, independent security/legal or
M10 final acceptance. The current compatibility matrix is consolidated; its old
chronological statements are preserved in docs/history rather than left to
contradict the current feature summary.


### Plaintext cleanup ownership chain

Exact combined source 7d3184f87165e11e12560884ec6e4f92c9bfd039 adds four
independently reviewed corrections: Reader V5, rollback Tx V2, exclusive WAL V2
and public metadata-admission V1. The checked 45-path composition preserves the
previous SQL source, including predicate startup and the original rowid sorting
query's positive oracle control. Existing storage bytes and exported layouts are
unchanged. Private ownership and lifecycle behavior are materially corrected.

Bounded minimal roots are reserved before native acquisition; unresolved metadata,
journal and WAL obligations stay reachable through explicit pending/retry APIs.
Children retire before their main database owner and lock. Initial ownerless
absence has explicit provenance; post-acquisition errors cannot authorize file
creation by broad errors.Is matching. Public rollback/exclusive canonicalization
transfers the reserved root once. Shared pre-session errors enter the existing
successful-close registry, whereas failed pager construction retains its separate
failed-open registry. Logical close, checkpoint and key/data release remain
one-shot; cleanup retries only retire retained obligations. No registry retains
DB/Tx/Rows, plaintext pages, options or arbitrary callbacks.

The original Reader V3/V4 and Tx V1/Exclusive V1 review failures are retained with
the fixed-version reports. Descriptor-free negative controls and bounded owner
laws are distinct from intact native lifecycle tests. Source inspection of the
existing encrypted Windows canonicalization found its metadata owners already
retained; that protocol is unchanged. Native recovery ownership and shared/recovery
legacy absence coupling remain separate code work, not covered by this claim.

The exact combined frozen source passed full guarded CGO0 unit/vet/build, the
complete pinned ordinary SQLite oracle, full race, five target cross-builds,
module verification and complete inventory at 2026-10-10T20:24:10Z with exit 0. Exact
commands and output are plain-final-commands.txt and plain-final-lead-checks.txt.
All published non-document changed bytes equal that tested source. Remote CI
and Library-restored exact commit checks are recorded separately. This checkpoint
does not qualify native failure paths, physical recovery, Windows/Darwin runtime,
independent security/legal review or M10 release acceptance.


### Leading secondary-index inequalities and acceptance traceability

Source 60d8a2d68c36c1a48333baf9af2c22eb5fb83188 adds eight checked paths onto
fully verified plaintext/SQL source7d3184. It uses real leading BINARY ASC index
bounds, bounded candidate rowids and source seeks; complete WHERE rechecks and
existing sorting remain. Composite equality-plus-range, covering/reverse/order,
JOIN probes and statistics remain code work. Independent exact rational, forced-
scan and official matrices passed; measured whole-query page calls are16/19
versus277, not only the3 index-page calls. LIMIT0/EXPLAIN retain planning reads.
Initial stale k>10 SCAN negative was upgraded with the exact original typed
query, actual SEARCH counters and forced-scan comparison; earlier failed fixture
logs remain in the owner evidence. Nothing pretends a table scan is an index hit.

Exact combined full ordinary unit/vet/build, complete pinned ordinary oracle,
full race, five builds, modules and complete inventory passed exit0 at 2026-10-10T20:58:20Z.
All published non-document changed bytes equal this source. Remote and restored
commit gates remain separately recorded. New journal/recovery/group/counter,
CLI/import verification and benchmark sources are excluded from this checkpoint.

The full original goal audit is preserved under evidence with exact source/time
identities. acceptance-status.md separates required code, runnable ordinary
verification, restricted manual gates and real external environments. Required
CLI backup/rekey, caller-dataset distance checks, normalized corpus and benchmark/
soak/performance-CI work take priority over optional SQL breadth. Current-tense
architecture/security contradictions are corrected; original documents remain in
history. The user's manual verification commands remain intact. M10 is not complete.


### Bounded multi-segment rollback parsing and native recovery ownership

Exact combined source 293a25e4ed905f970b437c28ce6419c1e1b89176 extends verified secondary source 60d8a2d68c36c1a48333baf9af2c22eb5fb83188 with ten
checked paths. The parser accepts aligned published finite-count segments with
matching geometry, per-segment checksums and bounded unique original pages. Its
576 legacy single-segment digests remain unchanged. Unpublished/no-sync/master,
ambiguous and other unsupported forms still fail closed. Native RecoverRollback
consumes this broader parser, so unchanged former consumer bytes would not have
proved unchanged native behavior.

The consumer now reserves bounded cleanup before resolution/acquisition, retains
metadata and temporary/main obligations, distinguishes initial absence, and
retires children before their protecting main owner. Retrying cleanup cannot
replay, truncate, Sync, unlink or republish. Independent negative memory/AST laws
and all prior findings remain in evidence. No RecoverRollback call or physical
recovery workflow was executed. Automatic public Open recovery is still absent.

Exact combined full guarded ordinary unit/vet/build, complete pinned oracle,
full race, five builds, modules and inventory passed exit0 at 2026-10-10T21:12:23Z.
All published non-document bytes equal that fixed source. Journal predecessor
full-source results are retained separately, not substituted for this combined
identity. Native failure/crash/namespace recovery remains NOT_RUN. Shared legacy
absence provenance and the separately reproduced ordinary rollback reader-open
stat/SHARED timing race remain genuine engineering gaps. M10 is unfinished.


### Bounded grouped aggregates, connection state and cached ordinary preparation

Exact combined source13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33 adds79 checked paths
onto verified replay/ownership source293a25e4ed905f970b437c28ce6419c1e1b89176.
It composes independently reviewed grouped-v2, counters-v1, cached-preparation-v3
and independent regressions by checked Git hunks, preserving existing range,
cleanup and recovery code. GROUP BY/HAVING retains bounded group/representative/
state allocations, conservative supported keys and proved rowid grouping. Stable
LIMIT/OFFSET admission and lookahead fixes retain earlier failing transcripts.

Counter state is connection-local and distinct from statement Result; successful
source inserts, ABORT, transaction rollback/savepoints, DDL and maintenance have
explicit typed SQLite controls. Warm cached ordinary preparation now distinguishes
invalid SQL from execution-BUSY. Cache admission includes fixed empty-catalog
cost, invalidation, binding and supported DDL/rollback paths. Metadata cache never
authorizes data or native acquisition. Sessions/extensions/constant-query warming
and complete stale-schema reprepare remain unsupported cache profiles.

Two imported legacy assertions were identified as outdated BEFORE execution, not
reported as failed runs. The retained warm valid INSERT under EXCLUSIVE now has
strict official changes=0, not the old limitation's2; missing-table control is
strengthened to reject BUSY. Original GROUP BY counter SQL is retained as a full
typed value positive, with official counterpart and additional grouped histories;
the actual unsupported window negative remains unchanged. All old source/assertion
artifacts are preserved. No control is replaced by a log-only assertion.

Exact-source focused normal/race/typed oracle/oracle-race preflight passed20:48:25Z. Full guarded CGO0 unit/vet/build20:52:30Z, complete pinned
ordinary oracle21:05:18Z, full race(SQLite475.910s), five target builds, module
verification and all hashes PASS exit0 at2026-10-10T21:14:01Z. All published
non-document changed bytes equal that fixed source. Exact remote/restored/CI
gates are separately recorded; no recovery/native-fault or foreign runtime ran.

Known wider planner error/phase differences remain held and corrective. Reader/
shared acquisition fixes, CLI/import verification, grams/transaction harnesses and
native integrity/performance-CI additions are excluded from this checkpoint.
M10 remains unfinished; no complete-goal delivery claim follows.


### Native acquisition fixes, maintenance CLI and dataset/grams tooling

Exact source ddb957e2a1d804b236a295f7fe6ce383954b852c adds32 checked paths to
13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33. It composes independently reviewed
Reader post-SHARED stat, shared initial-absence provenance, encrypted backup/rekey
CLI, importer-distance v2 and NFC grams measurement/correctness sources. Central
documents are reconciled separately; every changed production/test byte equals
the fixed checked composition. Complete ordinary unit/vet/build, pinned complete
oracle, full race, five target builds, modules and full source hashes passed;
exact clocks and all commands remain in accepted-tools-combined-checks.txt.

The Reader fix closes the stat-before-SHARED race observed in the original failed
20-second rollback sample. Two retained baseline controls fail; four corrected
20-second samples pass. These observations do not constitute long-soak acceptance.
Shared absence handling accepts only initial no-handle absence and retains later
lookup/retirement diagnostics. Exceptional native paths were checked only with
inert ownership/source laws, never injected into disk-backed operations.

CLI backup/rekey has independently verified correct-key snapshot/WAL/shared
archive/native restore and explicit provider boundaries. Plaintext backup remains
unsupported. Existing history remains decryptable with prior credentials; no
secure-erasure or native-failure/rekey durability claim is made. Dataset verifier
uses source norms, full exact distances and separate Top-K checks; ANN is not_run.
Its original16000-dimensional cancellation bound failed by14.0724x and remains
failed history. V2 derives outward arithmetic bounds challenged with rational
references, rather than widening an arbitrary tolerance or changing kernels.

NFC corpus covers12800 memberships/12416 distinct pairs, including100 inputs
changed by normalization, without changing LIKE_RAW_V1. Grams smoke and medium
measure real SEARCH/fallback paths and preserve slowdowns: medium512rows reports
147x DB size and4/18 selective medians slower than scan; smoke all12 are slower.
No universal speedup, large benchmark or confidence-qualified result is claimed.

TransactionV4 and nativeCheckV2 have separate independent scoped PASS, but are
excluded from this checkpoint pending their own full-source gate. Wider planner
V3 remains held history for34 newly exposed inherited multi-key/partial-sort
failures; V4b is awaiting independent review after6 V4 bound-LIMIT regressions. Windows
shared-rekey source has independent model/compile review, not native runtime.
Public recovery/Darwin/performance tooling remain separate corrective slices.
Restricted recovery/fault, target runtime and independent M10 gates remain open.


### Native full-profile integrity checks and ordinary transaction/soak tooling

Exact source8b604e3c667152284a29d7ad7dfba5a10f63b053 adds22 checked paths over
ddb957e2a1d804b236a295f7fe6ce383954b852c: independently reviewed transaction
harness V4 and native CheckV2. Full ordinary unit/vet/build passed21:49:37Z,
complete pinned ordinary oracle21:59:34Z, full race(SQLite583.504s), five builds,
modules and complete source hashes passed22:10:33Z. Changed non-document bytes
in the published checkpoint equal that fixed source; central docs are reconciled.

DB.Check/Tx.Check/CLI full-profile checks share one pinned snapshot and bounded
traversal. They validate catalog/free/all-page ownership, B-tree cell/freeblock/
fragment accounting, table/ordinary-index correspondence and built-in grams plus
dense/typed HNSW/IVF semantics. Detached-memory V1 false-success cases remain
failed historical evidence; V2 exact accounting and builtin-only validation pass
those retained controls. Custom registry callbacks never redefine full-check
truth. Acquisition work has existing pager caps outside traversal counters;
logical-NULL IPK checks do not claim physical serial-type0-only validation.
Existing configured WAL sessions support Check; a genuine readonly plaintext-WAL
full CLI is still an unimplemented profile. Existing named WAL checks retain
historical writable coordination and are not relabeled readonly.

TransactionV4 records actual concurrent epochs, deterministic transfer histories,
reader snapshots, full rollback/savepoint outcomes, ordinary checkpoint/reopen,
source/workload identity and truthful error/retirement status. Original V1 Reader
ErrChanged run, V2/V3 mixed-error/pending-retirement model findings and V3 WAL
storage-quota failure remain preserved. V4 requires all errorgraph branches and
keeps pending owners through reporting; a later nilClose cannot erase uncertainty.
Three64-attempt fixed-candidate runs passed, each with48 commits, exact independent
model reconstruction and normal closure. These are bounded smoke observations.
The predeclared two-hour-per-profile run, larger throughput measurements and
confidence-qualified performance acceptance have NOT_RUN status here. There is
no crash, fault, native recovery, namespace substitution or interrupted-rekey
execution. Correctness/ordinary source review is not independent security/legal
release approval. Wider planner, Windows/Darwin/public recovery and repaired
ANN/performance-CI sources remain separate increments.


### 2026-10-11：中文受限验收说明与独立范围恢复

- 状态：IN_PROGRESS；这是文档和只读对账阶段，不是 M10 完成。
- 用户要求继续未受限工作；平台无法验证项写入项目中文 Markdown，
  包括详细自测步骤。被中止的 ANN V4、新 41 目标 fuzz 与未知触发链路
  保持平台阻塞，不重发、不改变输入或换执行器/CI 重试。
- 01:36 只读确认远端和本地 main 仍为
  `776b23eb59ce9613bf4498d352a716a193cdc9c3`，tree
  `8f6026330f968314456548a3b8684476fea73afb`；文档修改前工作树干净。
  原 goal 为 43,100 字节，SHA-256
  `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`，
  本轮未更改。
- 01:37 重新从 Library 取回代码备份
  `libfile_3e43b3629e148191bb995c979b80e9a2`，与本地 4,946,692 字节
  bundle 逐字节一致；SHA-256
  `e293948441bdb6953d759636b1e818a0ba1ba2703aa628818ba685d3f9c4015c`。
  原中断报告 Library `libfile_0958d62d04dc8191bdb2ea277785b416`
  再次可读。本轮没有再次 clone 执行或重跑数据库测试。
- 将中断范围、已有自然终态、未持久源码与环境限制整合到
  `docs/interruption-20261010.md`；新增中文手册
  `docs/manual-verification-zh.md`，保留历史英文记录并加入当前入口。
  `docs/acceptance-status.md` 分开列出必须实现、可自动验证、受限待人工、
  真实外部环境；`docs/compatibility.md` 修正“没有 native full checker”
  的过时摘要。Native Check V2、Transaction V4、NFC 组合覆盖和精确距离
  v2 都已在 main，不能继续列作未实现。
- 补记中断前源码提交的现有证据：CI `38091940645` 在 10 日 22:52:30
  自然 SUCCESS，ordinary `114329977006` 和 oracle `114329976910`
  均成功；Library 恢复门 22:37:31 exit 0。备份候选的既有 64796 会话
  22:41:14 exit 0，但最终独立报告/事后全清单未闭合。这些是旧结果，
  不计作 11 日的新测试。
- 本轮静态核对方式：读取 Git ref/HEAD/tree/status；核对 goal 和报告哈希；
  Library 文本读取及 bundle 字节比较；阅读现有源码、测试名、标签、
  环境门与脚本；只修改文档。具体静态文档校验结果在完成后另记。
- 已发现可独立的窄工作候选：CLI `inspect` 的打开调用仍使用
  `sqlite.Open`，丢失 caller context；对应 G1、§3.6、§12.2。
  拟仅改该调用点及其 AST/普通只读回归，不改 pager、WAL、恢复、
  backup、rekey、ANN、性能作业或任何冻结候选。测试必须明确命名选择，
  保持完整 fixture 和正常关闭，不做故障/namespace/进程强杀。
- 后续独立候选：`inspect` 对已有扩展方法/版本、tokenizer 配置和
  generation 的只读元数据呈现（§3.7、§12.2），先审阅同快照与预算设计；
  不实现只读 WAL、不改变索引格式或算法。若范围不能保持独立则停止并
  记录，不将广 SQL 扩展用作绕过受阻的恢复/备份工作。
- 本段记录的初始文档阶段尚未执行上述候选；先完成范围报告，再进行
  开始普通实现/验证。固定未发布 a314/planner/WAL/backup/performance/
  ANN/retained-directory 源码本轮仅核对状态，不整合或恢复其执行链路。
- 真实代码缺口仍包括公开恢复、WAL 尾部 native publisher、明文备份、
  只读 plaintext-WAL CLI、既有 SQL 差异、部分故障 harness；详细矩阵不
  将这些归为“只差手工”。长测没启动；10 日发现的 tmpfs 余量及
  fsync=volatile 条件在未来测量前必须重新核验，不能推断已消除。
  新 OS/performance runner 与计费读取仍需单独许可。
- 下一步最小任务：校验并审阅中文文档的真实测试名/参数/前提，持久保存
  文档检查点；随后只推进上列经过报告和范围审阅的独立 CLI 小切片。


#### 中文手册静态复核发现与修正

- 手册中的 48 个真实测试引用（含 39 个 focused 普通选择）已静态核对；
  唯一不存在的 TestCrash 是显式禁止冒充入口的反例。新文档链接可解析，
  fenced blocks 配对，原 goal 与既有 checkpoint JSON 条目语义保持不变。
  此核对只读取文本，不执行手册命令。
- 独立文档审阅确认旧源码/main/候选边界、CI/备份身份和四类矩阵；将
  /tmp 容量判断明确为 10 日针对拟议长测的历史预检，未来必须重新核验。
- 命令级复核发现并修正文档规程：sparse-exact 的错误可返回 0，要求空
  stderr 与完整 JSON/结果断言；write 示例只限写入 smoke，其 Close/log.Fatal
  不证明资源退休；五个现成示例错误时也可能清理私有 fixture，不能保证
  完整失败现场。未伪称这些示例源码缺口已经修复。
- 为 maintenance/manual 命令添加独立 capture 名称与失败停止，源清单管道
  启用 pipefail，所有身份/hash 输出检查错误；说明开发空间交接资料并非
  仓库依赖。这些是未来用户的命令模板，没有在本轮执行。
- 获明确范围批准后，inspect context 的一行修正只在独立副本进行普通
  定名/只读验证。基线 AST 确认原调用丢失 ctx，运行回归原本通过；不把
  此发现夸大成实测取消时限故障。结果和精确修正树将在独立审阅后单列。
