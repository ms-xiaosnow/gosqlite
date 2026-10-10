# Independent encrypted backup/rekey CLI review

## Conclusion

No blocking findings in the reviewed CLI slice. Suitable for integration as an experimental encrypted-only CLI increment, within the runtime and security boundaries below. This is an engineering/security-boundary review, not independent cryptographic, legal, native-failure, crash-durability, or full-release acceptance.

Reviewed final tree: `d9969679165e42e22d8de5fe12ab57ee873b6c52`.
Baseline: `7d3184f87165e11e12560884ec6e4f92c9bfd039` (`plain-final-verify`).
Owner freeze: `../cli-backup-rekey-freeze-v2`.
Exact independent copy: `../cli-backup-rekey-independent-v2`.

The seven-path delta contains six new files and one modified existing file. There are 1,252 baseline files and 1,258 final files. No core encryption/storage/API-layout file changed. The complete source patch independently applies to the baseline and reconstructs the exact final bytes, modes and path inventory. Patch SHA256: `c7893f8a1c6f686cf137cca15dbfa4cdf670e986625f2bf863e01caf80e8c359`.

## Source-review findings

1. **Credential selection is explicit and narrow.** `cmd/gosqlite/key_provider.go:39–70` admits only env-hex, env-password, stdin-hex and stdin-password forms. Variable names use the ASCII identifier alphabet with a 128-byte ceiling. `Acquire` considers request kind but never chooses a different secret using untrusted request paths/IDs. Environment lookup uses exactly the configured name. There is no shell/provider command, URL, DSN, implicit environment name or raw-secret argument path.
2. **Input extent and ownership are controlled.** `key_provider.go:99–162` checks env length before copying; raw keys require 64 ASCII hex digits, passwords require 1–4,096 exact bytes. Stdin hex permits one LF/CRLF and passwords are not trimmed. V2 uses one fixed 67-byte or 4,097-byte input allocation and clears the complete allocation after success, invalid encoding, or synthetic input error. The fixed buffer replaced V1's growing io.ReadAll allocation. Provider acquisition is cached once; each borrow is independently owned, and close/Release clear controlled buffers. Formatters redact provider and material values.
3. **Stdin is a noninteractive channel.** `maintenance.go:100–110` permits only a regular file or pipe; no terminal prompt/echo changes or detached reader exist. Two explicit stdin descriptors are rejected before acquisition/open. Backup's omitted output provider deliberately reuses its one explicit source provider, including a cached stdin input. This is documented same-credential encryption, not a hidden key default.
4. **Parsing and diagnostics avoid disclosure.** `maintenance.go:29–98` rejects unknown/repeated/missing options without echoing any submitted token. Mode choices are exact snapshot/wal/shared, with snapshot as the documented default. `maintenance.go:196–225` returns fixed categories/static sentinels and discards arbitrary original error chains; uncertainty, poison and recovery precede BUSY. JSON success contains only operation/status/format/mode/geometry/history-policy fields. The code never constructs a credential-bearing DSN or shell invocation.
5. **Native ownership and publication remain authoritative.** `maintenance.go:159–193` opens existing sources with NoCreate, read-only for backup, and calls DB.BackupEncrypted or DB.Rekey/VerifyEncrypted. There is no raw file copy, plaintext spill, custom replacement/cleanup protocol, fallback, or retry loop. WAL connections keep WAL on close. Connection close is joined before JSON success. Native `sqlite/encryption_copy.go`, `internal/pager/encrypted_operation.go`, and `internal/encstore/backup.go`, `object.go`, `publish.go`, `shared.go` retain pinned selected-head validation, native exclusive destination creation, stable file ownership, synchronization and same-mode rekey publication. A destination alias to an existing source cannot bypass native exclusive creation. These protections were source-reviewed; adversarial namespace and failure scenarios were deliberately not executed.
6. **Plaintext and restoration boundaries are accurate.** A backup invocation without a source provider explicitly returns unsupported before database access. Archives are portable encrypted archives, not live DB roots. Native RestoreEncrypted is documented and exercised; no restore CLI or generic online plaintext backup is claimed. Rekey retains prior ciphertext/history and does not revoke old credentials' access to that history.

## Independent execution evidence

Go 1.27.2, Linux/amd64, offline module cache, GOPROXY/GOSUMDB off. CGO was disabled normally and enabled only for selected race checks. All test bodies/helpers and the runnable example were read before execution; `go run ./internal/testscope` preceded selected normal/race/probe runs.

Passed on the exact V2 frozen copy:
- `go test ./cmd/gosqlite -run '^(TestCLIProviderMemory.*|TestCLIMaintenance.*)$' -count=1 -timeout=5m -v`
- The same selected set with `-race` and CGO1
- `go vet ./cmd/gosqlite ./examples/encrypted-cli`
- Built the actual CLI with trimpath and ran the documented disposable native-restore example
- `go mod verify`

Additional reviewer-only tests were added in a separate probe copy, never to owner/main/frozen source. Passed normal and race:
- Exhaustive 256-byte env-name alphabet and 128/129-byte name boundaries; rejected descriptor forms; exact env selection independent of request path
- Exact 67/4,097-byte oversized-reader lookahead and no reacquisition after a failed attempt
- One-byte and 4,096-byte exact passwords including NUL/newline; borrow independence; cache/input backing cleanup
- Uppercase hex and permitted LF/CRLF endings; private error-chain removal and poison/recovery precedence
- Actual subprocess backup/rekey for snapshot, exclusive-WAL and shared-WAL, using generated correct keys via env source and stdin target only
- Native restore and indexed row/BLOB/Unicode readback for every mode; independent restore DB identity; source root unchanged by backup; archive unchanged by restore
- Shared-mode backup while an idle cooperative writer connection remains open
- Successful rekey preserving source DB ID/mode, increasing sequence once and selecting zero WAL segments

The independently built V2 binary is byte-identical to the owner's V2 binary, SHA256 `5cf11e5aeac9e200228a8eea44f00b7aaafc58452fce4c430e44a3bf166a970f`.

V1 evidence is retained. V1 selected normal/race and the real subprocess example passed; the owner then supplied the bounded-allocation V2 hardening. No selected behavioral test/probe failed. No result was silently rerun or replaced. Owner-reported full builds and cross-builds are separate evidence; this review makes no non-Linux runtime claim.

## Deliberate limits and residual risks

- This does not complete goal §8.3 or full goal §12.2/release acceptance. No wrong-key DB open, ciphertext tamper/corruption, missing selected file, namespace substitution/race, native I/O/locking/unmap/close/Sync/shortwrite/diskfull/permission fault, abrupt termination, crash/hot recovery or interrupted rekey/publication was run.
- No manual tags, broad old-baseline suites, external SQLite process/oracle, network fetch, commit, upload or remote write were used.
- Input memory is bounded, but a pipe producer that does not close can block the OS read. Context checks do not interrupt that read. The documentation states this accurately; do not claim a time-bounded credential read or cleanup while it remains blocked.
- Environment values, Go strings, runtime/crypto copies and privileged host inspection can retain/expose credentials. Controlled buffer clearing is best effort, not absolute erasure.
- Errors after native publication or stdout failure need not mean nothing changed. Existing partial encrypted output/history is retained; this CLI supplies no retry, repair, recovery, revocation or secure-erasure guarantee.
- Native platform restrictions, Windows shared rekey rejection, quota policies and retained-history semantics remain unchanged.

## Evidence index

- `source-audit.txt`: selected-body and supplemental-probe safety audit
- `v2-inventory-proof.json`, `v2-inventory-checks.txt`: exact baseline/delta/reconstruction proof
- `v2-frozen-checks.sh/.txt/.exit`: exact frozen selected tests, race, vet, build/example, module verification
- `review_boundaries_test.go`, `v2-probe-checks.sh/.txt/.exit`: independent probes and execution
- `v2-final-readback.txt`: post-test frozen and reviewer-copy inventory check
- `evidence-sha256.txt`: evidence hashes
