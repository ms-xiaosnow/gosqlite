# Public recovery harness documentation composition

Result: eight-path documentation composition and static patch replay PASS. No Go tool, test binary, fixture generator, database operation, source/CI edit, main edit or physical scenario was executed by this worker.

## Deliverable and exact base

- Candidate documentation: `source/docs/` under this directory. Copy only the eight paths in `owned-paths.txt`, or apply `documentation.patch`; do not copy this entire candidate tree over main.
- Base commit: `50b627e11e9dbce7f2fde8df35c4634c52a06d52`.
- Base tree: `8f2709293287a3517a52b3f989888ded8230ef53`, independently reconstructed from all 3,077 archived file identities.
- Eight-document-only candidate tree: `5e2d7581f8517b53abfcf82867775f3af32bbea4`. This deliberately excludes the lead-owned new harness, topical manual, selected evidence and checkpoint updates; it is not the final composition or V3 compiler-reviewed tree.
- Patch SHA-256: `faf402a638549860a7f5b9677e06a7559e2d4ea8fa74a40c669f269bddb1945a`.
- The manifest records every owned path's exact base Git blob, candidate blob, byte count, SHA-256 and Git mode. Base-blob equality must hold before copying into mutable main.
- Main was read-only throughout this assignment. Its clean status observed at 09:10:26 was an instantaneous observation; the lead's later additions were neither assumed absent nor treated as defects.

## Changes

- Current acceptance, compatibility, restriction and closeout rows now identify the four-case public Open harness as implemented and independently source/compiler-only qualified.
- Current pointers and appended Chinese section 20 distinguish nil-default/input-preservation, opt-in/read-only reopen, journal quota refusal and NoCreate absence. Every physical case remains BLOCKED / NOT_RUN.
- Public recovery and English manual addenda preserve their original P0/namespace text and commands as historical records while linking the exact new entrypoint, review evidence and dedicated steps.
- `docs/progress.md` preserves its complete 50b base prefix byte-for-byte and appends one new entry. Original goal remains 43,100 bytes with SHA-256 `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`.
- Driver Connector physical harness, wider journal/platform support, native WAL-tail publisher and complete fault/crash harness remain genuine gaps. Existing unknown/ANN V4/41-target/physical execution restrictions remain unchanged.
- P0 terminal CI evidence already recorded in 50b remains unchanged. Namespace exact-restore report was read: its 207-command limited source/model/selection/compile/symbol PASS completed at 09:14:09 UTC. Namespace CI 38127196102 has no verified terminal outcome in this worker's evidence; current text retains last-known in_progress, not SUCCESS.
- Per the lead's follow-up, the original V3 manual hash refers only to `v3-reviewed-manual.md.txt`; the published topical manual may change its status paragraph only. Commands/assertions stay unchanged. This worker did not edit that lead-owned file.

## Static checks actually performed

- Exact-base archive: 3,077 files verified by Git blob, bytes, SHA-256 and Git mode; candidate changes exactly the eight owned docs, all 3,069 other files unchanged.
- `git apply --check` and independent static patch application onto a separate exact-base archive: PASS. Replayed 3,077-file bytes/Git identities match candidate; file and directory POSIX modes match the candidate.
- All original code/command fences in the eight docs remain byte-identical. Forty-five shell fences passed `bash -n` only. No fence contents executed.
- Reviewed V3 manual's three shell blocks also passed `bash -n`; its fail-closed checker has no inverted grep pattern. Exact source first-line and first-statement environment gate were inspected as text only.
- Read-only original-input hash checks match source dd5a63e2..., original V3 manual a49ae188..., and V3 review e59001ae....
- 180 relative links and anchors verified against the candidate plus explicit lead-owned new-source/manual/evidence read-only overlays. Five links point to the lead-owned summary `docs/evidence/public-recovery-harness-20261011.md`, which was not present at the check time. They remain pending until the lead creates that exact summary and reruns the link check. No other missing link was accepted.

## Integration notes

The parent owns all additions outside these eight docs, including `docs/checkpoints.json`, the single Go harness, published topical status paragraph, evidence summary and selected evidence. Preserve their independently recorded identities. Final composition review must cover those additions and recheck links. Neither this static documentation result nor the P0-base V3 compiler PASS supplies the new composition's runtime, restore or CI result.

Evidence files here include `evidence/static-result.json`, `evidence/patch-replay.json`, complete base/final identity manifests, `evidence/docs-only-tree-identity.json`, `evidence/relative-links.json`, `evidence/unchanged-command-fences.json`, and reviewed-input identities. The generated candidate/replay source directories contain no Git metadata and were not used to execute code.
