# Encrypted managed cleanup handoff

Final source: `../encrypted-cleanup-freeze-v3`, reconstructed from
`69008bf658611318154a112178913c693b679a2b` plus 33 owned paths.

- Owned source manifest: `encrypted-cleanup-source-sha256.txt`
- Manifest SHA-256: `25d5541df56656535db32ff1e344e3888869e1e1c84efd49f828b823434a9fa8`
- Exact selected patch: `encrypted-cleanup-selected.patch`
- Patch SHA-256: `fe7fa5e4c1653f30e776ea636e16a20ff926c8831cad792b4f145ad0134972cc`
- Shared sqlite.go patch: `encrypted-cleanup-wrap.patch`
- Shared patch SHA-256: `8a2e4e42ce273ec999c3e15eca63be00e5570e60ac207977689f5861b7d60fe1`

The selected patch reconstructs all 33 manifest files exactly against its base.
The shared patch independently checks against durable SQL commit
`8f75ccf927b8dc0048d43c65736bdc3ad6fee20c`. Apply only its wrap classification and
Close GoDoc hunks to that newer sqlite.go; do not overwrite the SQL Query region
with the older full frozen file. All other live owned source matches v3.

## Result

Every encrypted native acquisition is charged to a bounded common owner scope
before the native call. Constructor failures retain their acquired lease or an
explicit terminal obligation. The graph preserves selected/transient children, lifecycle locks and
directory protection through staged explicit retirement and failed constructors.
Logical Store/session closure clears key-bearing state and invalidates operations
once without discarding retained native obligations. Dedicated public encrypted
cleanup tokens/retry/snapshot APIs cover no-DB and discarded-Close paths.

Admission bounds: 1024 opening/live/pending scopes and 16384 managed leases,
reserved before native acquisition. Native platform physical bounds also apply.
Admission reclaims only explicit atomic completion proofs, never native cleanup
or a blocking resource-status call. Live retries close only requested resources.
Opaque external error objects may themselves retain arbitrary references; no
stronger transitive secrets or total-RSS bound is claimed.

## Verification

Pinned Go 1.27.2; offline module settings; shared go-cache. Before each executed
suite, the source guard and behavior audit passed. The focused CGO0 and identical
race selections, vet, and package build passed on exact v3. The independent pass
also compiled the whole tree and encstore/pager/sqlite/driver tests for
linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64. Cross-target binaries
were not executed. Cipher/layout memory vectors and ordinary provider-count
fixtures remained intact. See isolated v1/v2/v3 and independent evidence.

The independent review reproduced two v1 descriptor-free failures, then verified
both probes pass on v2. v3 differs from v2 solely by a public GoDoc clarification.
Original v1 failures and source hashes are retained; they were not overwritten
with later PASS results. Independent v3 review found no remaining blocking
finding in this slice.

Actual failed native I/O/close/unmap/lock/Sync, corruption/tamper/wrong key,
missing selected files, namespace replacement, crash/kill/process-exit/hot
recovery, disk-full/permission failure and interrupted publication/rekey are
NOT_RUN. Darwin/Windows runtime qualification and full milestone acceptance
remain NOT_RUN. Plaintext Reader/Tx/import-source and WALExclusive gaps are
outside this correction.

No staging, commit, remote action or upload was performed by the implementation
owner. Lead owns composition, final aggregate checks, staging and publication.

Independent final report: `encrypted-retirement-independent/review.md`
SHA-256 `1e186f15a6df614aacdf16203ffae6b8c9b99d59f44f94faa6a188aaacc81b07`.
Independent evidence manifest SHA-256
`c88abc61b5ca93c129a98822f0672ab0a44eb3ebe4f2bf45ee46de35dede1106`
covers its 35 retained report/log/probe files.
