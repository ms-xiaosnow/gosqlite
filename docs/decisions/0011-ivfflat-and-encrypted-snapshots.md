# ADR 0011: Persistent IVFFlat and the first encrypted storage slice

Date: 2026-10-10. Status: implemented experimental ordinary paths; full acceptance conditional.

## IVFFlat

Preserve existing HNSW v1 canonical configuration and original grams13 schemas.
Use a distinct canonical IVFFlat configuration inside the already reserved ANN
family: deterministic bounded minhash samples, ordinary L2 Lloyd centroids or
spherical cosine/IP centroids, and persistent centroid/member/reverse covering
B-trees. Original source vectors remain the authority for final distances.

Too few trainable values yield a persisted untrained index requiring REINDEX.
DML changes fixed-centroid membership transactionally, never silently retrains.
Queries score centroids once, read selected list ranges and recheck base identity,
filter and original metric. All-list probing chooses exact scan. Resource and
attempted-work counters remain cumulative when fallback occurs. Native exact is
still default; automatic SQL ANN is narrow and explicitly observable. Supported
metric/representation identity is closed rather than an arbitrary callback.

This optimizes candidate work, not a promise of latency or recall. The retained
2048-row persistent measurements show slower latency than exact for all tested
probe counts. No global >=95% IVFFlat recall guarantee is inferred.

## Encrypted snapshot integration

Keep primitive GSQLENC1 bootstrap/AEAD contracts intact. Below the logical SQLite
pager, a separate storage session publishes an authenticated root referencing
one fully authenticated immutable base. Complete page/footer/extent verification
precedes page interpretation. Fresh identities and keys apply to each attempted
publication/rekey/copy; plaintext journal/WAL publication is not reached.

A persistent empty lock inode protects the changing root identity. Linux read-only
sessions share a lifetime lock; writable sessions hold exclusive lifetime locks.
Changed transactions copy complete logical snapshots. This first slice does not
implement encrypted WAL or concurrent writers/readers and cannot substitute for
those required features.

Publication retains prior ciphertext and evidence, bounds inventory/generations/
bytes, and never chooses an older root automatically. After namespace-publication
uncertainty the session is poisoned. The source branches exist, but no prohibited
physical failure/interruption workflow was executed.

Code-configured KeyProvider supplies credentials; no DSN or SQL secret option.
Rekey changes current keys but does not revoke old keys for retained history.
Whole authenticated family substitution/replay is not prevented without external
expected identity/freshness state. Canonical parent paths are not continuously
pinned directory namespaces. See encryption-storage.md for details and actual
API/limits. This is not SEE compatibility or an independent security audit.

## Evidence and remaining work

Six-way IVFFlat plaintext SQLite interoperability and three-page-size correct-key
encrypted lifecycle/export interoperability run ordinarily. Independent scoped
engineering reviews cover source, memory math/framing, transactions and resource
limits. The first encrypted restore test exposed zero connection quotas in its
standalone validator; shared normalization fixed it and the unchanged regression
passed. This failure is retained, not concealed.

Broader SQL, recovery, native platform, encrypted WAL and migration remain code
work. Physical crash/fault/recovery, native OS execution, independent legal/
security/cryptography review and long-soak evidence remain separate release gates.
