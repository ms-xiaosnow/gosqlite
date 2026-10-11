# Independent review: finite example fixture retention v1

Result: SCOPED PASS. No candidate-source blocker found. This is not a full-repository, failure-fixture-retention, recovery, durability, production-readiness, or milestone acceptance.

## Fixed source and identity

- Baseline: 1352ae524575e4d6fe8b5434fd930c3b07be4ac8 at /workspace/scratch/5e06a74e5f65/durable-restart/restore-library-inspect-metadata.
- Candidate: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-freeze-v1.
- Candidate manifest: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-freeze-v1.json; SHA256 7c3906728139107b7ea58729d2f2f868e641a170e249b8c7435323194c856478.
- Candidate patch: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-final-v1.patch; SHA256 07069a393c06be6d40295f882cc6a1aab9ab902d153922cb8e99d046a04c38e4.
- Owned SHA list: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-owned-v1.sha256; SHA256 fcc86fe01c4b7cace7513fd8bde49accffe2256ff2cbd41a261e230840d1ed82.
- Independent copy: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-review-work-v1.
- Exactly 10 changed paths: README.md, docs/progress.md, five example main.go files, and three new examples/internal/fixturecleanup files.
- Independently compared the actual immutable baseline and fixed candidate against their manifests. All 353 existing test files and all 39 non-run function blocks retained byte content; all existing test modes retained.
- Candidate and independent copy contain 2,907 regular files and 139 directories, no symlinks. Full relative file membership, SHA256 contents, file modes, and directory membership match before and after execution. The fixed candidate was rechecked after the independent run.
- Source-before SHA256: 0e281c2e7aadff621268a43f40a0c7c9e0bb9ac2d94fdd73675dd7cdfa981c84.
- Source-after SHA256: 0e281c2e7aadff621268a43f40a0c7c9e0bb9ac2d94fdd73675dd7cdfa981c84.

## Source findings

1. Each finalizer is registered after the successful MkdirTemp guard and before all subsequent resource defers. The closure reads the named result and completion flag when deferred execution occurs, after ordinary later defers have contributed their errors.
2. Finalize calls no removal callback when completion is false or result is non-nil; it joins a retained-directory diagnostic with any original error, preserving errors.Is matches. An incomplete nil-error result becomes a non-nil diagnostic. A completed nil-error result calls removal exactly once. A removal error is wrapped with %w; no retry occurs.
3. The completion flag is assigned true only at the final ordinary successful return. All ordinary output calls in the five run functions are checked before completion. Native-check's run now has a named error result; its existing JSON output check remains intact.
4. Existing database/Rows Close aggregation, provider/key creation and clearing, helper functions, and child failure diagnostic handling are unchanged. The new diagnostics add a quoted temporary-directory path and no credential or child-output logging.
5. Helper, README, and progress wording correctly warns that failed removal can be partial, remaining ciphertext may not remain decryptable, and cleanup is not secure erasure. The policy is explicitly finite and ordinary; it is not a panic, process-exit, crash, or arbitrary-fault guarantee.
6. The helper tests use only callbacks, call counts, and sentinel errors. The five-file AST guard only reads source. Existing WAL/shared tests execute their ordinary successful workflows. WAL's original t.TempDir/TMPDIR cleanup cannot prove retention after real failure.

## Authorized verification

Pinned Go 1.27.2 linux/amd64; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false, GOMAXPROCS=2, dedicated GOPATH/GOCACHE, TMPDIR/TMP/TEMP unset (default /tmp; the unchanged WAL test internally sets TMPDIR to t.TempDir).

- CGO0 ordinary: exact five top-level tests TestFinalizeCallbackPolicy, TestFinalizeDelayedResult, TestExampleFixtureFinalizersSource, TestEncryptedWALExample, TestEncryptedSharedExampleOrdinary; PASS.
- CGO1 race: the same exact five tests; PASS.
- Before each selected run, TestExampleFixtureFinalizersSource was separately run in the corresponding CGO/race mode; PASS.
- CGO0 scoped go vet and go build: helper plus encrypted, encrypted-wal, encrypted-shared, encrypted-cli, native-check packages; PASS. Build only for snapshot/CLI/native-check.
- All ten recorded check commands exited naturally with status 0, all stderr files empty, no gate failure, denial, or retry. Complete source identity checks passed before ordinary, before race, and after execution. Source and main worktree were not edited; no Git or remote operations occurred.
- One preliminary read looked for a nonexistent Makefile and emitted a missing-file diagnostic. This was not a verification gate, was not retried, and no Makefile was used; repository instructions were read from README and the supplied finite scope controlled execution.

## Coverage limits

No real failure retention was tested. No snapshot/CLI/native-check main/run(binary) or built example binary was executed. No new subprocess scenario, wrong-key, tamper, native I/O fault, process-exit, crash, recovery, namespace, interrupted-rekey, ANN V4, 41-target fuzz, old held chain, new OS, performance, long test, full suite, CI, Git, or remote gate was run. No held gate is released by this result. Existing successful WAL cleanup is observed only within its original test scope; shared ordinary success is not a direct retained-directory assertion.

## Evidence

- Directory: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-review-evidence-v1
- Script: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-review-checks-v1.sh
- Identity review: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-review-evidence-v1/source-review-identity.json
- Full before/after identity: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-review-evidence-v1/source-before.json and /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-review-evidence-v1/source-after.json
- Each named check has .command, .started, .finished, .exit, .stdout, and .stderr files.
- Evidence integrity index: /workspace/scratch/5e06a74e5f65/durable-restart/example-retention-review-evidence-v1/SHA256SUMS (all evidence files except the index itself).
