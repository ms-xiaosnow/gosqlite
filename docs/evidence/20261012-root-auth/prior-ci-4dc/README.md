# CI evidence for 4dc3979

## Verified result

Repository: ms-xiaosnow/gosqlite (private)
Workflow: ordinary-checks, run 38309383658, attempt 1
URL: https://github.com/ms-xiaosnow/gosqlite/actions/runs/38309383658
Head: 4dc3979aa8d503913b75e3c0c432c234d7bc0a40
Tree: 4fbf9a52417eedcd7dcf6e08ecb31a4147915a9f

The existing workflow reached completed/success naturally. No cancellation,
rerun, new runner, dispatch, GitHub write, or local project execution was used
to collect this evidence.

- ordinary, job 115011121005: success; started 2026-10-12T21:14:38Z;
  completed 2026-10-12T21:41:38Z.
- oracle, job 115011121204: success; started 2026-10-12T21:14:38Z;
  completed 2026-10-12T21:24:17Z.
- Workflow terminal updated_at: 2026-10-12T21:41:39Z.
- Terminal state observed: 2026-10-12T21:42:30.584Z.

The workflow API does not provide a completed_at field. The updated_at value
above is labeled as such; individual job completion times are exact API fields.

## Scope and provenance

This is independent new CI evidence for this exact commit. Existing local PASS
evidence was reused separately, with affected/new test details under
docs/evidence/20261012-presync in the source repository. It is not a reconstruction
of lost historical raw evidence and does not validate later commits.

The source-only/manual physical scenarios, including the restricted
crash/fault/process-exit scenarios, are excluded from the existing workflow.
A successful source guard or an ordinary helper unit test is not evidence that
a restricted physical scenario ran. Both logs state that behavioral review is
still required after source-reference guards pass.

The ordinary job ran ./scripts/check.sh. The oracle job built and verified the
pinned official SQLite oracle, then ran ./scripts/test-oracle.sh. The oracle log
reports SQLite 3.54.0 with source identity
be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3.

The existing source bundle is separate: Library
libfile_61f98d644b8c819189c89c27ad158961, 7,459,815 bytes, SHA-256
867144b25c8a2e61c6815f7dba95123b80c35efe89784a567ced3ab99f971789.
Its previously verified 3,368-file bare restore result was supplied with this
capture task; this worker did not repeat that source-bundle restoration.

## Files and integrity

- ordinary-115011121005.log and oracle-115011121204.log contain the complete
  decoded job-log strings returned by the supported GitHub job-log endpoint.
  They were encoded as UTF-8 without editing, preserving initial BOMs and final
  newlines. These are decoded-text captures; original downloaded transport bytes
  are not claimed.
- run-and-jobs.json retains selected run and job/step metadata, excluding
  unrelated account data and author email fields.
- workflow-at-4dc3979.yml is the workflow file fetched at the exact head.
- observations.json contains compact read-only status observations.
- capture-validation.json records log sizes, line counts, hashes, and the bounded
  credential review.
- SHA256SUMS covers every other archived member.

Credential-related lines were reviewed before archiving: 12 per job. Token and
authorization values were already masked by GitHub; no unmasked credential was
found in that bounded review or the specified secret-pattern checks. Logs are
unaltered. This review is not a proof that every possible secret format is absent.

The ZIP is a locally assembled evidence archive, not the original GitHub workflow
log ZIP. The ZIP checksum is kept outside the archive to avoid a self-reference.
