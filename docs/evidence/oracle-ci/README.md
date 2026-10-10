# Ordinary oracle CI correction evidence

Base: `883999c35b29ed31b4c4e0e3d66712b4a8ed996c`. Isolated archive; no production
changes, no staging/commit/upload. The original lead full-oracle failure remains
in the unchanged `relational-lead-v2-checks.txt` outside this slice.

## Final verified results

- `fresh-builder-v2.txt` and `.exit`: exact final builder, exit 0, fresh output
  directory, explicit existing official archive input. Required 14,881,448 bytes,
  streaming SHA3-256 verification of source ZIP and generated amalgamation, exact
  executable version/source ID all passed. No upstream test target was built or
  run. `sqlite-configure-v2.log` and `sqlite-build-v2.log` retain the build output.
- `final-builder-source-sha256.txt`: builder hash recorded while the final fresh
  build was in progress and rechecked unchanged after completion.
- `focused-fresh-oracle.txt` and `.exit`: Go 1.27.2, offline module/toolchain
  settings and shared Go caches, source guard PASS, then full
  `TestOracleOrdinaryFreshFixtures` and `TestOracleErrorsAndResources` with the
  newly built final oracle. Exit 0; testkit 0.090s. The exact original JOIN now
  passes positive pinned comparison; unsupported USING retains and passes the
  explicit `ErrUnsupported` assertion.
- `focused-existing-oracle.txt` and `.exit`: same focused cases also passed
  against the previously verified pinned CLI (testkit 0.085s).
- `script-contract-checks.py` and `.txt`: syntax checks and synthetic shell-tool
  routing checks passed. Missing oracle, wrong Go, relative/missing executable,
  failed guard, wrong SQLite identity, exactly complete final Go command,
  incorrect archive size, existing builder destination, and exactly two ordinary
  CI jobs were checked. These mocks do not represent real Go-suite execution.
- `manual-tag-refusal.txt`: actual guard command rejected `manual_namespace` in
  GOFLAGS with expected exit 1 before any Go tests. This runs only the guard,
  not any manually tagged test body.
- `behavioral-audit.md`: scope, helper review and review of all JOIN rejections.

Focused command (preceded by the guard and the recorded behavioral review):

```sh
CGO_ENABLED=0 go run ./internal/testscope
CGO_ENABLED=0 go test -count=1 -timeout=30m -tags oracle ./internal/testkit \
  -run '^(TestOracleOrdinaryFreshFixtures|TestOracleErrorsAndResources)$' -v
```

## Iteration history retained without relabeling

`fresh-builder.txt` is an earlier successful candidate build before the bounded
streaming hash and diagnostic improvements. A second candidate invocation in
`fresh-builder-final.txt` returned exit 2 because its shell script was edited
while its C build was still in flight, invalidating that invocation's source
identity and shell parsing position. Its raw log/exit are retained; it is not
final acceptance. No database process was interrupted. The final stable builder
was then run independently in a new directory and completed successfully as
`fresh-builder-v2` above.

`script-contract-checks-initial.txt` found that `set -e` does not exit on the first
failed term of `test -f && test -x`; a missing oracle therefore reached the guard
before failing at invocation. The final script uses an explicit conditional and
exit, and all shell-contract cases were rerun successfully.

The complete corrected unit/vet/build/race/full-tagged-oracle suite and hosted CI
for the integrated exact source are the lead's remaining acceptance steps. This
worker did not run those aggregate suites and does not claim a hosted CI result.
