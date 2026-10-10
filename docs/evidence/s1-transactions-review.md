# Independent S1 engineering review

2026-10-10 UTC. Source review and named ordinary tests found no remaining
production blocker in the bounded S1 slice. This is not an independent release,
security or physical durability audit.

Added regressions:
- exact aggregate dirty-page/freelist/name accounting and mark-ID overflow;
- detached substring-backed savepoint names;
- canceled/panicking statements and rejected nested controls preserving marks;
- constant SELECT and first-access BUSY retry;
- failed lazy freelist replay releasing its lease while retaining logical marks;
- per-call Result/ordinary-index behavior through repeated rollback-to.

Reviewed existing tests also cover deferred timing, BEGIN modes, repeated names,
DDL, failed outer RELEASE, quotas, native entry points, grams/HNSW/IVF undo,
correct-key encrypted normal reopen, pool reset and failed driver Commit cleanup.
An initial review test used unsupported CREATE UNIQUE INDEX; it was corrected to
supported CREATE INDEX and a primary-key failure. Production code was unchanged.

Reported actual scoped results with Go1.27.2, offline modules and writable cache:
- mandatory `go run ./internal/testscope`: PASS before each sequence;
- named existing/review CGO0 cases: pager 0.008s, sqlite 4.606s, driver 0.010s;
- final `go test ./internal/pager ./sqlite -run '^TestReview(Savepoint|LazySavepoint)' -count=1 -v`: PASS, pager0.011s/sqlite0.019s;
- separate selected CGO1 race: pager1.022s/sqlite20.649s/driver1.047s;
- `go vet ./internal/pager ./sqlite ./driver`: exit0.

Final test SHA256:
- internal/pager/savepoint_review_test.go: fb7dd87a81add86f1e2783dda7bae53fc1a4919144372cd8d70d48eb123d69ce
- sqlite/transaction_control_review_test.go: 5b811fc2b97a50490d5a565978a7348ee0d80bd21d84915cc766899b0bc7726c

Savepoint names are cloned only after quota checks; the pager stores identifiers
rather than name strings. Connection-level changes()/last_insert_rowid() SQL
counters do not exist, so no counter-parity claim follows. Active-Rows restrictions
remain; cursor relaxation is later code. No manual-tag tests or restricted
physical error/namespace/recovery workflows were executed for this review.
