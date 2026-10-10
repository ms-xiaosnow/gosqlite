# Lead JOIN integration evidence

The implementation/reviewer final27-path source remains unchanged at manifest
4dd07287c3687ee466b51b7d6c343261aeb997ff81c01f75bdfe0188b85a9397.
Independent review README SHA2566889d178d40aad8a9716441353fc27f65010e335684410b5d57a93a077da5ce9
records seven scoped PASS suites and604 oracle comparisons. These are scoped
source checks, not the final full aggregate or exact commit CI.

A lead preflight at18:30Z refused an unannounced changed candidate manifest.
No tests ran. The original preflight trace was not captured; the retained shell
log records its consequent missing archive-directory error. The owner explained
the reserve-cancellation fix and supplied a new immutable4dd directory, which
the independent reviewer verified. No hash mismatch was ignored.

The first full CGO0 suite on tree2f1ab3df86cdf5aaf97143d6ad53d186da836b3f
failed at18:36Z because sqlite.TestRejectionsAndCodes still rejected plain JOIN,
a feature now intentionally implemented. Its exact log and exit1 are retained
in relational-lead-first-aggregate-failure.txt/.exit. No later stages of that
set-e script ran.

The lead replaced only that negative query with JOIN ... USING(id), which
remains unsupported, and added TestRelationalReadonlyFixtureJoin, asserting
exact[[int64(1)]] for SELECT id FROM samples JOIN numbers WHERE id=1 LIMIT1.
All other rejection assertions remain intact. An independent source reviewer
checked this diff and confirmed it preserves the negative contract while adding
the new positive one. The guard and both focused fixture tests passed0.007s.
Gofmt then inserted only the standard blank line before the new test comment.

Final frozen source is baseline69008bf658611318154a112178913c693b679a2b plus
28 explicit paths. Its manifest is relational-lead-source-sha256.txt, SHA256
c64078230751713f951fcc3dd1a71f4470657d2eaf1e18e82035c89b826abb0b; Git tree
ad73b8d6425cd86d1c15caccb0f858048df63ef9. It excludes concurrent encryption
and full-table migration changes, including the separate sqlite.go wrap hunk.
The full rerun began18:37:35Z with fresh guard. Exact commands/output and final
result are recorded in relational-lead-aggregate.txt; any captured intermediate
log explicitly reports pending later stages.

Only intact ordinary database/oracle/driver workflows and descriptor-free
parser/math/budget/ownership laws are selected. Newly added test sources/helpers
were behaviorally audited; the source-reference guard is supplemental. Existing
manual_namespace/manual_recovery sources are unchanged/excluded. No restricted
physical/native failures or native non-Linux qualification were executed.

Git diff --check initially reported only literal context-prefix spaces in the two preserved .patch evidence artifacts; excluding those patch-text artifacts passes. Their original bytes remain unchanged.
