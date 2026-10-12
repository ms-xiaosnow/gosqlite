# Independent bounded source review: encrypted layout-1 root negatives

Review scope: the seven-file `root-auth-overlay` against the clean `gosqlite` main 4dc3979 checkout. This review used only source/document text reads, SHA-256, textual comparison, and this report write. The reviewer did not run Go, gofmt, any Go parser/compiler/test/manual binary, native/database operations, network operations, or Git writes; the checkout was not modified. Physical execution of all six cases remains NOT_RUN.

## Disposition

Final source-review disposition: CLEAR after one narrow ReadDir diagnostic-retention correction. No unresolved blocking finding was found within the requested six-case facility. The four new pins and five existing main pins may be released only to the exact final reviewed hashes below. All model/AST/source-selection/compile execution remains a separate status. Source references in the reviewed-guarantees section use the preserved initial snapshot line numbers; the fixed final native lines are 241–243.

### R1 — Preserve the directory-read diagnostic (medium, corrected and independently re-reviewed)

`sqlite/encrypted_root_negative_manual_linux_test.go:170–171` obtains `entries, err := file.ReadDir(...)`, but calls `r.check(..., nil)`. An unexpected ReadDir error makes the test fail, yet the original diagnostic object is never added to `r.causes`, unlike the other acquisition/read/write/close failures. This violates this facility's stated original-error retention contract; it does not currently turn a simple unrelated error into a successful authentication observation.

Required narrow correction: call `r.pending(err)` immediately after ReadDir, and admit only `err == nil || err == io.EOF` in the subsequent check with nil as its diagnostic argument. The literal EOF check also avoids treating a mixed wrapped EOF as clean completion. This is a local preservation fix, not a requirement to traverse arbitrary error programs or change the production cleanup wrapper.

## Reviewed guarantees and bounded limitations

- Default exclusion and authority: native line 1 uses exactly `linux && manual_encrypted_root_negative`; entry lines 243–262 require the authorization acknowledgement as their first statement, exactly one admitted case, a non-root Linux process, an acknowledged isolated namespace, reviewed test flags, CGO disabled, and exactly the manual tag. There is no init/TestMain, subtest sweep, process launch, goroutine, recovery, rekey, alternate key retry, or automatic cleanup/delete. Tag/env values are expressly not authority.
- Fixed fixture: native lines 34–50 acquire only a source-visible disposable raw key, optionally flipping its first byte. Lease release clears borrowed bytes and detects duplicates. Native lines 121–125 and 280–301 restrict the writable seed to 512-byte pages, two pages, three commits, and a literal three-row ledger. This is test material, not a hard-coded production/security example.
- Six choices: model lines 32–48 admit control, wrong-key, root-salt, metadata-ciphertext, metadata-tag, and unknown-major only. Mutations use offsets 144, 536, 664, and 9 respectively; XOR 2 changes big-endian major 1 to 3. The 512-byte header, 24-byte record header, 128-byte manifest, and 16-byte tag give the verified 680-byte extent. Source basis: `encryption/format.go:9–18,150–169,203–230`, `internal/encstore/format.go:133–137,197–198`.
- Actual target and control: native lines 327–340 perform one candidate OpenContext with ReadOnly and NoCreate. Control requires authenticated geometry, supported Check, exact typed ordered values, three rows, EOF and nil Rows.Err. The five negatives require nil DB and their exact public class/code, then never Query or Verify the candidate. Unknown-major requires zero provider acquisitions; the other cases require one. Existing memory AEAD/root tests and healthy disk tests are acknowledged rather than claimed missing.
- Copies and retention: native lines 268–279 create new owned private directories beneath the supplied direct root; lines 185–194 use O_CREATE|O_EXCL and complete bounded writes/Sync/once-only close. Only copied in-memory root bytes are changed. Four complete seven-member snapshots bind sizes, names, hashes, inode/device, ownership/mode/link count. Model lines 131–178 reject missing/unrelated members, aliases, changed originals, post-Open changes, wrong byte counts/offsets/XOR, and zero salt. The fixed family geometry is supported by the production publication path: three bases, two retained roots, current root, and lock. No claim is made that path checks prove an adversarially changing namespace; the stated private stable namespace is an external prerequisite.
- Owner accounting: native lines 62–118 capture original results, retain pending cleanup tokens, close at most once, refuse DB/Rows parent retirement after uncertain child retirement or pending internal cleanup, and retain the run globally. Rows EOF relies on the actual auto-close behavior in `sqlite/rows.go:83–129,132–175`; it does not call Close again. Exactly 40 file owners account for one binary read, four snapshots each with one directory and seven files, and seven O_EXCL writes. Model lines 185–201 bind the DB/Rows ownership relationships, absent negative DB, and exact inventory.
- Error classification: native lines 228–240 inspect the original direct public `*sqlite.Error` with Op `open`, public code, errors.Is/errors.As classifications, and known unrelated I/O/cleanup/context/format/provider failures. Authentication requires CodeNotADB (26); unsupported requires both encryption and public unsupported with CodeError (1). Production source supports these outcomes in `internal/encstore/store.go:193–214` and `sqlite/sqlite.go:72–121,321–328`. `internal/encstore/cleanup.go:204–218` intentionally preserves Is/As without Unwrap; the new caller respects that real contract. This review does not require or claim proof for arbitrarily fabricated error implementations.
- Output safety: the receipt contains scalar facts, public family names/identities and hashes; original error objects stay in memory. No key/provider/options object, raw error text, or queried values are formatted. Family bytes and the JSON receipt are scanned for the fixed plaintext marker and SQLite header; the separate retained-directory locator is a caller-selected fixture pathname. No durability, nonce-restart, physical safety, whole-A8 or production-readiness result is inferred from a receipt.
- Pure and source-only closure: the ordinary model uses only fmt/reflect/strings; its tests use scalar records and contain no storage/key/SQL operations. The AST guard inspects inert source and finite critical statements, restricts imports/functions/calls, checks immediate owner/error capture and counts, rejects copied manual capabilities/consumers, and applies full-source pins. Its finite repinned counterexamples cover the gate/tag, readonly/no-create, original-result capture, original preservation, error fabrication, ledger bypass, retry and rekey. This is a finite aid plus independent source review, not a type/dataflow proof for arbitrary Go programs. AST tests have not been executed by this reviewer.

## Whole-main review and pin release

The entire overlay `internal/testscope/main.go:1–357` was read. A byte/text comparison independently established that removing only the five-line per-source root-negative inspection block (175–179) and nine-line repository-contract block (344–352) exactly recovers the clean baseline main. All prior inspections, hard errors, findings propagation, full-repository contracts and final failed exit remain present; `inspectSource` and its old narrow exclusions are unchanged. The new per-source hook precedes normal/manual exclusion decisions and the repository hook detects missing pinned paths. No old helper exemption was broadened.

- Baseline main SHA-256: `e31c233fed8976fddff3236a8804f84d493e2567d6b6d85d11355de2ca0da52b`
- Reviewed overlay whole-main SHA-256: `6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2`
- The same reviewed literal may replace the five old literals in `encryption_upgrade_manual.go:30`, `wal_tail_publication.go:77`, `rollback_fault_sql_test.go:307`, `rollback_fault_test.go:162`, and `rollback_restart_test.go:235` together. Their existing behavior and assertions must otherwise remain unchanged.
- The four new pins were intentionally unreleased in both reviewed snapshots. Their exact final values are now cleared below; only the pin literals may change. Do not reuse pre-format hashes.

## Initial reviewed identity

The seven-file manifest was independently SHA-256 checked. Sorted relative-path/TAB/file-hash/LF aggregate: `8a1abc7898981e15e2322410930fe6fcb5c1c701a971c1b8f089133c34a6acbc`.

| Path | Initial reviewed SHA-256 |
|---|---|
| docs/testing/encrypted-root-negative-manual-zh.md | 8c9edf1e27cac4635ef6282ee878d70016c54290d06100960fe88e01dadb47f3 |
| internal/testkit/encryptedroot/contract.go | 64df16c9c22501cd9b51356d640ed4d801ff5671eb4c12860053367f49ee67cb |
| internal/testkit/encryptedroot/contract_test.go | 6953c1f572fe347c0587164c98daddc7a3a11a03d237792ba7380ccb11a471c6 |
| internal/testscope/encrypted_root_negative.go | 6163c19433e25651b501f6d396d708b6a0067004ed2d6464a95a4844dd2fba69 |
| internal/testscope/encrypted_root_negative_test.go | 0b6b4f9dfaf6e9782e6ea2131797275602948c6c43352021de57ce164f3287e8 |
| internal/testscope/main.go | 6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2 |
| sqlite/encrypted_root_negative_manual_linux_test.go | f014c880be517ce6d1b06d6a30562b4076571dcb48fc7012d567a614c220bc38 |

## Scope stop

Only the real A8 root-negative caller gap is reviewed. No new obligations for page/base/segment replacement, DBID/parent swaps, layout 2/3, password salt, restart/rollback/rekey nonce tests, interrupted publication, physical persistence or recovery throughput are added. Those broader original-goal questions remain separate. The six physical cases, including control, remain NOT_RUN and this platform's execution restriction is unchanged. Source review/pin release does not authorize running the manual binary.

## Final correction and formatted identity, 2026-10-12 21:26 UTC

R1 is corrected at final native lines 241–243: the immediate `r.pending(err)` retains the original ReadDir result before a nil/literal-EOF check. The preceding unformatted correction diff contains only those changes. The implementer performed its separately parent-authorized single fixed gofmt pass; the reviewer did not run gofmt. The retained initial source snapshot SHA-256 is `f9ad40ff38656f3a813acdbd7f83ec81d4f1e650f49ca87360f01b31e55cc28c`.

The reviewer independently compared initial snapshot content to all seven final files using a bounded text tokenizer preserving comment and literal content while ignoring whitespace and separator semicolons. All reviewed tokens match after applying exactly R1; no Go scanner/parser was run. Every manifest file hash and the aggregate were independently recomputed. Main is byte-identical to the initially reviewed two-hook main.

Final pre-pin-release aggregate: `a4c08e7fa590b3dd5d15585d361279813093b3b2619a2765180d9d161669a72d`.

| Path | Final reviewed SHA-256 |
|---|---|
| docs/testing/encrypted-root-negative-manual-zh.md | 8c9edf1e27cac4635ef6282ee878d70016c54290d06100960fe88e01dadb47f3 |
| internal/testkit/encryptedroot/contract.go | 21713928744c666e6bccc08e053d6f18ab4e3a92740c655db5896c38f3fd1a3f |
| internal/testkit/encryptedroot/contract_test.go | 4fcfaaa2b47fc8fbaa58e096b0db56f6be2c01c9b75c021590ed91e2e59facbf |
| internal/testscope/encrypted_root_negative.go | 220345befdca8d39f6c478be2374a34bd21a1ac9391036aa91abbda75cafa234 |
| internal/testscope/encrypted_root_negative_test.go | a7131758eb24d5824bf0b6c461aa3efd39c360e66c4a902bcd25e9180fb8faac |
| internal/testscope/main.go | 6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2 |
| sqlite/encrypted_root_negative_manual_linux_test.go | 3e1e3640ff321a7673d9f39c29d32cfe73ec2d4c53858205ee9a61b7738bbbc3 |

Approved new full-source pin values are the final native, model, model-test and main hashes in this table. `main-pin-literals-only.patch` was reviewed and changes exactly one old-main literal in each of the five specified files, preserving their other code. Parent integration must apply only those replacements against its current combined source; patch line offsets may differ if its independent post-DB work has already advanced the checkout. This report does not pin or qualify unreviewed concurrent changes.

The new guard file's hash will naturally change when its four HOLD values are replaced; that narrowly specified literal update is included in this source-review disposition. Further semantic edits require re-review. Documentation may record completed independent review and later actual validation while preserving the six PHYSICAL_NOT_RUN statuses. No full goal/M8/M10, physical authentication, durability or execution qualification is granted.
