# Experimental authenticated-record primitives

This package is a **memory-only M8 prerequisite**. `sqlite.Open`, the driver and
all existing database/journal/WAL files are still plaintext. There is no encrypted
storage adapter, manifest authority, WAL publication, rekey workflow, encrypted
backup/restore or plaintext import/export implementation in this checkpoint.
`NewRekeyEpoch` generates identity/salt values; it does not rekey a database.
No SEE file compatibility, cryptographic audit or production security is claimed.

## Key lifecycle and suite

- AES256GCM with96-bit nonce and full128-bit tag, using Go's standard library.
- HKDFSHA256 separates master, file role and record role keys.
- Explicit raw32-byte key or Argon2id password credential kind; no length-based
  auto-detection or password fallback. Password1..4096bytes, raw key exactly32.
- Argon2id version0x13 from pinned golang.org/x/crypto v0.58.0. Default/minimum
  memory64MiB, passes3, default lanes4. Bounds: maximum256MiB,6passes,8lanes,
  combined work cap1,572,864KiB-passes; lanes1..8; memory is divisible by4×lanes. Local policy can lower
  maxima but never lower production minima. A nil/zero DerivationBudget shares
  a process-wide one-derivation semaphore; explicit budgets1..8 must be shared
  by an application. Context is checked before/after synchronous Argon2, which
  cannot be interrupted mid-derivation without violating the resource bound.
- Code-configured KeyProvider.Acquire supplies borrowed material. The library
  validates public costs first, copies bounded material, calls Release exactly
  once on nonnil material even on error, and does not wrap/format provider error
  contents. Providers remain trusted code. Credentials never belong in DSNs,
  SQL, argv, logs or repository files.

NewEpoch creates random DBID, epochID, root salt and password salt where needed.
NewRekeyEpoch retains the logical DBID but creates fresh epoch/salts. DeriveMaster
returns opaque owned state; handle copies share Close/allocation state. Master
Close prevents new file handles but does not revoke already-derived reader/writer
keys; those handles also require Close. Clearing owned buffers is best effort;
Go/crypto/runtime may retain copied or expanded keys. No absolute erasure claim.

## Write-once files and authenticated locations

NewFileWriter takes immutable FileDescriptor geometry/commit identity and allocates
fresh256-bit FileID plus random32-bit nonce prefix internally. No public API
accepts/resumes an old writer identity or counter. File-role/record-role keys
are derived with HKDF. Nonce is prefix||uint64BEsequence, allocated and burned
before each Seal. All copied handles share mutex/counter. Any Seal error poisons
the writer; retry must use a fresh writer. File IDs are tracked within an opaque
master handle and limited to2^20; cross-process collision resistance depends on
cryptographic randomness. Readers can never become writers.

The experimental canonical bootstrap is512bytes, magic GSQLENC1, version1.0,
suite1, zero reserved bytes. It records public IDs, credential/KDF parameters,
page geometry, record count, file role, commit sequence, transaction and parent
identity. These are not trusted merely because decoding succeeds. A24-byte
record header carries role, sequence and plaintext length. AAD binds the complete
bootstrap SHA256, full record header and independently supplied expected page/
frame identity. GCM authenticates this before returning plaintext.

OpenFile requires expected FileIdentity and the metadata record, authenticating
both before returning a read-only handle. Expected child identity must come from
a trusted/authenticated parent, not just be copied from the child's untrusted
header. Open requires a complete expected RecordIdentity; page/frame routing is
not exposed in the public record header. Failure returns no plaintext. Root
has one metadata record; base/backup descriptor has metadata, ordered page slots,
footer; WAL descriptor has metadata, consecutive frames with unique page numbers,
footer. Metadata/footer payload semantics remain opaque and unimplemented here.
Finish verifies allocated record count only; it is not a durable publication or
whole-file completeness validator.

Hard bounds: page512..65536 power-of-two,<=2^20pages,<=64GiBlogical extent,
<=2^20+2records and<=1MiB record plaintext. Limits can only lower those maxima.
Record length and format are checked before allocation. Strict unknown versions,
roles, flags and padding reject. The future adapter must additionally validate
root authority, stream exact extent/footer, directory uniqueness, source database
structure, bounded aggregate work and publication/recovery protocols.

## Evidence and threat-model boundary

Memory-only tests include NIST AES256GCM and RFC5869 HKDF known answers, direct
pinned upstream RFC9106 Argon2 vectors, minimum-production-cost password derivation,
wrong passwords/keys/salts, every-byte header/record/tag mutations, truncation and
extra bytes, cross-file/page/role/epoch/frame substitutions, copied-handle nonce
state, poisoning, concurrent Seal/Close, provider release/redaction and cancellation.
No damaged/tampered database file or physical fault/recovery workflow is created.
Focused unit/race/vet and source review are scoped engineering evidence, not the
independent M10 cryptographic review. Detailed final gates are retained separately.

A valid historical root/snapshot can still authenticate; AEAD alone cannot prevent
whole-snapshot rollback without trusted external state. Public extent/IDs/KDF costs,
access patterns, process memory, denial of service and hardware/OS compromise
remain relevant. Journals, WAL, spill, checkpoint, backup and rekey need actual
integration before any database confidentiality claim. This format is experimental,
not yet a stable supported encrypted database format.
