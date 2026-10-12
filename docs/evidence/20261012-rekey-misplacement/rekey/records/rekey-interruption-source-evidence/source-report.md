# R5 six-case rekey facility: source-only handoff

Status: SOURCE_ONLY / REVIEW_HOLD. Independent controller/observer/model source review is clear with explicit limits after the listed bug fixes. Separate guard/main review is pending. No Go tool, gofmt, parser executable, compiler, tests, source-selection program, fixture, native database operation, wrong-key Open, process exit or archive restore was run. No working-checkout file was edited. No physical result is claimed.

## Deliverables

- `../rekey-interruption-overlay/` contains exactly eight changed/new source and documentation files.
- `manifest.json` records their exact identities, current baseline identities, six closed cases, one profile, eight model roots, seven proposed guard roots, five manually tagged native role roots and all NOT_RUN statuses.
- `rekey-interruption-source-only.patch` contains only those eight paths.
- `../rekey-guard-overlay/` is separately owned by the guard worker and is not included in this patch. Its final guard/main/precise admission diff requires independent review and integration.

## Production boundary

Only `internal/encstore/publish.go` is an existing production file changed here. It gains exactly three void calls, before `attempted = true`, after the successful rename branch and after the successful final directory Sync branch. Literal source comparison after removing those three lines reproduces the current checkout file exactly. All real statements, branches, error propagation and ownership operations remain in their original order.

The default stub has the exact `!linux || !manual_rekey_interruption` constraint and an empty function. It imports only context. The manual implementation uses the complementary exact Linux/manual conjunction. No Rekey algorithm, pager, VFS, platform syscall, ordinary public API, old A5 controller or old A5 model is changed.

The manual observer carries only public metadata/ciphertext hashes, concrete provider borrow counts and a fixed channel. It never receives a Store pointer, master/key bytes, callback, arbitrary descriptor or function-valued action. Its registry snapshot does not invert cleanup lock order or call cleanup/native methods. Native capabilities are absent from the ordinary source stub; binary/symbol validation remains NOT_RUN.

## Finite facility

The six cases are control, before-root-rename, after-root-rename-before-dirsync, after-root-dirsync-before-adoption, rekey-returned-no-ack and acknowledged-exit. One admission selects one case and one new retained run root. The one profile is Linux/layout 1/raw key/4096 logical bytes. Optional old-archive restore belongs only to control.

The native controller binds complete source bytes, compile-only receipt, compiler, actual `/proc/self/exe`, same-inode spawn path, fixed role test roots and flags, empty GOFLAGS and an allowlisted environment. It owns a single Start/Wait for each fixed child and two distinct fixed pipes. A parent-granted role-ready frame precedes native role work. There is no PID-derived capability, shell, PATH lookup, Kill, Signal, CommandContext, retry campaign or child-spawning child. Live/unknown children keep their process and pipe owners and block dependent probes.

ACK is emitted only from the exact original public-nil frame received by the parent. The file uses exclusive creation, full write, file Sync and one Close, separate control-directory Sync and Close, identity/hash readback and a non-alias check, before `ack-exit` is granted. Unacknowledged and not-returned states remain separate.

Providers have fixed purpose/path/operation and independently chosen OLD/NEW public fixture labels. Their cloned borrow is released once and cleared. Observer barriers can read concrete atomic borrow counts without credentials or callbacks. Wrong-key classification retains the original error object and requires nil DB, original public Open error/code 26/authentication, no forbidden public/crypto/I/O/cleanup diagnostics and no pending registry entries.

Both successful generations require the same three baseline rows, seven independent literal SQL/index queries, real index-page work even for an empty green result, EOF/Err/one public Rows.Close, full bounded Check counters, exact authenticated generation and one DB.Close. A5's plain OLD/NEW SQL model is not used.

The fixture-family copying protocol preserves every recognized entry and rejects unexpected types, aliases, name/header mismatch or plaintext sidecars. Limits are 64 entries, 1 MiB total and 64 KiB each. The original and exited trial are never reopened, repaired, promoted, overwritten or supplemented with a Sync. Only fresh destination copies are synced. Dev/inode/length/hash manifests are rechecked.

Public nonce records cover all unique immutable original/post-exit root/base objects, including seed history, and optional archive/restore objects. Aliases are identified by whole-object hash. Public domain/sequence/nonce collisions between distinct ciphertext events are rejected. The descriptor-free model covers the selected OLD/NEW root/base domains, and has an independent native-compatible canonical JSON transport mirror. Neither scope constitutes an arbitrary-restart nonce proof.

## Claim boundary and independent review

The model always returns false from PhysicalValidated and Pass. Actual native receipts also retain PhysicalValidated=false. Testing stdout is diagnostic only, never the authority for physical success.

Still required before any finite tool validation: independently review every affected observer/controller/provider/error/owner function, source guards, exact new process admission predicate, complete inspectSource/main, native-to-model JSON schema parity, and affected current source pins. In particular, A8's whole-main pin cannot be blindly changed to silence the scanner. All old A5 restrictions and guard hooks must remain active.

After such review, finite model/AST/selection/compile-only validation needs separate authorization and exact selectors. All native cases, wrong-key pairs, archive restore, fault execution and denied/unknown chains remain NOT_RUN until actually authorized and executed within their admitted environment. Source existence or compilation cannot silently discharge physical acceptance.

Chinese concrete preparation, ownership, failure handling and later manually admitted operation steps are in `docs/testing/rekey-interruption-manual-zh.md`. That document contains no executable launch recipe or implied authorization.

## Independent review corrections in final renewed freeze

The controller now blocks while an actual owned Wait is unresolved, so returning from a failed testing root cannot abandon live process capabilities. A Wait call returning without ProcessState stays unknown and holds the controller indefinitely without another Wait. Valid actual exits require terminal event-pipe EOF before retirement; trailing bytes are rejected.

Model and native paths use fixture.gse, role generations are distinct 1–5, and the model explicitly covers both optional control-archive selections. The pure model Signal scalar is named SignalNumber to avoid changing inspectSource's existing exact A5 exception. Native nonce credential labeling compares the exact restore family directory, never an arbitrary ancestor substring.

The model's LogicalPages/PageOneCounters are synthetic value relations only. The current native caller observes full logical SQL/index/Check and authenticated encrypted generation identity, but not plaintext page-hash/counter byte equality. This remains an explicit bounded observation limitation rather than an invented native claim or a new encryption hook.

The independent reviewer confirmed no further definite native/model/observer blockers at the renewed freeze. This is source review only, not permission to compile, test or execute native cases.
