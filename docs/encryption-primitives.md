# Experimental authenticated-record primitives

The `encryption` package provides **memory-only building blocks**. The
experimental M8 storage uses them for authenticated snapshots and dirty-page WAL
through `sqlite.Options.Encryption`, including rekey, encrypted backup/restore,
and explicitly named plaintext import/export APIs. See
[encryption-storage.md](encryption-storage.md) for that integration's actual API,
root/base format, quotas, lifetime leases and limitations. Nil encryption options
still select the plaintext engine; native journals/WAL are not encrypted, and
encrypted snapshot mode rejects native WAL options. Explicit EncryptionWAL uses a separate authenticated segment format, not native SQLite WAL.

`NewRekeyEpoch` only generates identity/salt values; `sqlite.DB.Rekey` performs
the storage rewrite and retains older ciphertext. Full M8 acceptance, SEE file
compatibility, an independent cryptographic audit and production security are
not claimed. This document preserves the primitive-specific design and evidence;
the primitives themselves do not perform disk I/O or durable publication.

## Key lifecycle and suite

- AES-256-GCM with 96-bit nonce and full 128-bit tag, using Go's standard library.
- HKDF-SHA-256 separates master, file role and record role keys.
- Explicit raw 32-byte key or Argon2id password credential kind; no length-based
  auto-detection or password fallback. Password 1–4,096 bytes, raw key exactly 32.
- Argon2id version 0x13 from pinned golang.org/x/crypto v0.58.0. Default/minimum
  memory 64 MiB, passes 3, default lanes 4. Bounds: maximum 256 MiB, 6 passes,
  8 lanes, combined work cap 1,572,864 KiB-passes; lanes 1–8; memory is divisible
  by 4 × lanes. Local policy can lower maxima but never lower production minima.
  A nil/zero DerivationBudget shares
  a process-wide one-derivation semaphore; explicit budgets 1–8 must be shared
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
fresh 256-bit FileID plus random 32-bit nonce prefix internally. No public API
accepts/resumes an old writer identity or counter. File-role/record-role keys
are derived with HKDF. Nonce is prefix || uint64BE(sequence), allocated and burned
before each Seal. All copied handles share mutex/counter. Any Seal error poisons
the writer; retry must use a fresh writer. File IDs are tracked within an opaque
master handle and limited to 2^20; cross-process collision resistance depends on
cryptographic randomness. Readers can never become writers.

The experimental canonical bootstrap is 512 bytes, magic GSQLENC1, version 1.0,
suite 1, zero reserved bytes. It records public IDs, credential/KDF parameters,
page geometry, record count, file role, commit sequence, transaction and parent
identity. These are not trusted merely because decoding succeeds. A 24-byte
record header carries role, sequence and plaintext length. AAD binds the complete
bootstrap SHA-256, full record header and independently supplied expected page/
frame identity. GCM authenticates this before returning plaintext.

OpenFile requires expected FileIdentity and the metadata record, authenticating
both before returning a read-only handle. Expected child identity must come from
a trusted/authenticated parent, not just be copied from the child's untrusted
header. Open requires a complete expected RecordIdentity; page/frame routing is
not exposed in the public record header. Failure returns no plaintext. Root
has one metadata record; base/backup descriptor has metadata, ordered page slots,
footer; WAL descriptor has metadata, consecutive frames with unique page numbers,
footer. Metadata/footer payload semantics remain opaque to this primitive
package; the snapshot adapter now defines and validates root/base/backup payloads,
exact whole-object extent and canonical reserved lock-page slots. A primitive WAL
file role is not an implemented encrypted WAL storage or publication protocol.
Finish verifies allocated record count only; it is not a durable publication or
whole-file completeness validator.

Hard bounds: page size 512–65,536 power-of-two, <= 2^20 pages, <= 64 GiB logical
extent, <= 2^20 + 2 records and <= 1 MiB record plaintext. Limits can only lower
those maxima. Record length and format are checked before allocation. Strict unknown versions,
roles, flags and padding reject. The snapshot adapter adds authenticated root
authority, exact extent/footer validation, source structural checks, bounded
directory work, aggregate retention quotas and normal publication. These
additional layers do not supply external freshness/identity expectations or
qualify the still-unexecuted crash/fault/recovery protocols.

## Evidence and threat-model boundary

Memory-only tests include NIST AES-256-GCM and RFC 5869 HKDF known answers, direct
pinned upstream RFC 9106 Argon2 vectors, minimum-production-cost password derivation,
wrong passwords/keys/salts, every-byte header/record/tag mutations, truncation and
extra bytes, cross-file/page/role/epoch/frame substitutions, copied-handle nonce
state, poisoning, concurrent Seal/Close, provider release/redaction and cancellation.
No damaged/tampered database file or physical fault/recovery workflow is created.
Focused unit/race/vet and source review are scoped engineering evidence, not the
independent M10 cryptographic review. Detailed final gates are retained separately.

A valid historical root/snapshot can still authenticate; AEAD alone cannot prevent
whole-snapshot rollback without trusted external state. Complete-family
substitution under a reused credential also requires an external expected
identity to detect. Public extent/IDs/KDF costs, access patterns, process memory,
denial of service and hardware/OS compromise remain relevant. The snapshot
integration avoids plaintext journals, WAL and spill, but plaintext export is an
explicit downgrade, and rekey does not revoke old credentials' access to retained
history or securely erase it. Live Linux family operations use a pinned parent-directory descriptor; initial
resolution and copy helpers remain path-based, with no complete hostile-namespace
defense or executed namespace-race qualification. See the storage
document for integration-specific evidence and unexecuted gates. This format is
experimental, not yet a stable supported encrypted database format.
