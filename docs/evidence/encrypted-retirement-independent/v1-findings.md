# v1 findings (retained after correction)

Frozen source: ../encrypted-cleanup-freeze-v1.
Owned-source manifest SHA-256: 76878f419ec4933d06bff108037c9f4194f6fa49080d8a55888484f8a04730f3.
All 32 manifest entries checked successfully; tracked comparison against base 69008bf658611318154a112178913c693b679a2b found no unowned changes. sqlite/sqlite.go contains only encrypted classification, not the concurrent Query change.

1. Active encrypted pager Close erases session options on BUSY. CloseWithError unconditionally sets opts=Options{} even though retireEncryptedSession's active-lease branch leaves the session open. This can lose KeepWALOnClose and cause a subsequent checkpoint. Independently reproduced using an empty Store and active Tx, with no native calls. See v1-busy-options-probe.go.txt and v1-busy-options-failure.txt (expected FAIL).
2. Store.Check and checkAuthority collapse all absent-root probe errors to ErrChanged. On Windows those errors can contain native probe cleanup tokens. The original error and nested cleanup edge are lost before enclosing retirement. Source-only finding; no native negative branch executed.
3. Nested error adoption occurs too late at some constructor/temporary boundaries. Open/CreateFrom join resultErr to s.Close after selected resources have begun retiring. inventory directly returns platformDirectoryCheck's independent probe error before its enumeration close; similar standalone path boundaries require adopt-before-close. Source-only finding. Owner confirmed and is broadening the bounded ownership correction, not merely wrapping text.
4. Admission's reapCompleted takes a group TryLock but then blocks in resource RetirementComplete, which acquires managed lease/native gate locks. A live Read/Write need not hold group.mu. Thus an unrelated reserve can wait behind a live resource's status lock. Independently reproduced with descriptor-free channel-blocked status callback. See v1-admission-probe.go.txt and v1-admission-failure.txt (expected FAIL).
5. Pre-existing diagnostic gap: readExact replaces underlying ReadAt errors with encryption.ErrFormat. This is distinct from retention ownership, but unqualified preservation-of-original-cause claims would be too broad. No native error executed.

The owner received each finding immediately. v1 focused allowed tests, Linux CGO0 build/vet and linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 whole-tree builds plus encstore/pager/sqlite/driver test compilation passed. Those passes do not invalidate the independent negative source/model findings. v1 is not approved.
