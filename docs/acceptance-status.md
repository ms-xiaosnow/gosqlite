# Required work and acceptance categories

This is a current engineering work matrix, not final delivery or external review.
The unchanged goal.md is the contract. The detailed dated source audit is
[evidence/goal-acceptance-audit-20261010.md](evidence/goal-acceptance-audit-20261010.md).
That audit preserves its own earlier source/time identities; current commits and
terminal results are recorded separately in checkpoints.json and progress.md.

| Category | Goal clauses | Current concrete work / evidence |
|---|---|---|
| 必须实现 | G1, §§3/4.1, M3 | Aligned finite-count multi-segment memory replay and minimal native recovery ownership are integrated with ordinary byte-only/source evidence; physical recovery remains unexecuted. Fix the newly observed rollback reader-open stat/SHARED timing race, finish shared initial-absence provenance, define safe public recovery integration and supported WAL-tail handling. Physical execution remains separately restricted |
| 必须实现 | §4.2, §§11.1/11.3 | Correct known supported-query error/order differences. Planner candidate fixes420 old cases but retains16+2 failures and is held; grouped/counter/cached-preparation code awaits a separately passing combined tree. Phased optional grammar is lower priority |
| 必须实现 | §§8.1/12.2 | Safe usable encrypted backup/rekey CLI v2 passed independent ordinary review and awaits combined integration. Generic plaintext backup and native whole-database check are narrower scope decisions, not implied by extension checking or official integrity_check |
| 必须实现 | §9.2(2/5) | Caller-dataset norms/exact-distance/Top-K verification v1 is held: an independently reproduced 16,000-dimensional signed-IP case exceeds its too-narrow numerical bound by 14.0724 times. A derived correction is in progress; count/PK/component fidelity already exists. ANN recall remains separate |
| 必须实现 | §5.2, M10/§11.3 | Add NFC-normalized randomized input corpus, grams build/space/P50/P95 harness, transaction-throughput/real-duration soak harness and variance-aware performance CI. The first rollback 20-second soak failed after 14.0159 seconds with ErrChanged and is retained; shared/encrypted short profiles passed. Fix the reader race before acceptance. Do not call repeated unit tests a long soak |
| 必须实现 | §§8/12.3 | Windows shared encrypted rekey and Darwin Unicode inventory remain code gaps. Current architecture/security/notice pointers are reconciled without rewriting historical evidence |
| 可自动验收 | §§0/11, M0–M10 ordinary scope | Exact-source no-CGO build/unit/vet, relevant race, full pinned ordinary oracle, intact SQL/BUSY/cancel/close/reopen/checkpoint and correct-key lifecycle, math/parser/model fuzz, measured benchmark/soak workloads, verified Git/Library restore and exact-commit CI |
| 受限待人工 | §4.1, §8.3, §§11.1/11.3 | Physical crash/hot recovery, native failure, wrong-key/tampered disk, namespace/missing-selected-file and interrupted-publication/rekey gates remain NOT_RUN. Keep the user-requested concrete commands in manual-verification.md; agent execution/relocation and fabricated acceptance are prohibited |
| 需要真实外部环境 | §11.3, M3/M8/M9/M10 | Real native target-OS execution; independently qualified data-safety/crypto/legal/API review; an authorized disposable PostgreSQL/Neon environment for remote migration claims; controlled hardware if shared-cloud measurements cannot support release claims |

The goal says at least10,000 normalized randomized LIKE combinations, not10,000
query executions. Existing100×128 memberships have legitimate counting evidence;
explicit NFC input-corpus coverage is being added without changing LIKE_RAW_V1.
The goal supplies no numeric large-benchmark size or soak duration: predeclare a
justified workload and report actual data/clock/confidence, not flag maximums.
No additional user action is currently needed for permitted code, harnesses or
ordinary tests. Eventual real access/review decisions cannot be replaced by docs.
