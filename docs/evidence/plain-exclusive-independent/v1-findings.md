# Exclusive V1 source finding

2026-10-10 UTC. V1 source freeze remains unchanged. Findings concern source control flow; no physical metadata failure, invalid-path negative, or native exceptional operation was executed.

## Public exclusive metadata discovery gap

`sqlite.OpenContext` reserves a `SharedCloseAdmission` only for WALShared (sqlite/sqlite.go:244–252), then invokes `canonicalOpenPath` (256). On error it returns immediately through `wrap` (257–258). The exclusive `pager.OpenSession` root is not reserved until the later call (289).

On Windows, the plaintext `canonicalOpenPath` path calls VFS ResolvePathForCreate and, on initial no-handle absence, additional ResolvePath/LstatForCreate calls. These use managed metadata owners. A post-acquisition inspection/retirement failure can return an error-carried retained VFS owner; nativeOpenPathError preserves it, but this early public return never adopts it into a plaintext root. The underlying VFS metadata registry still retains protection/tombstones. What is absent is a discoverable `PendingPlaintextCleanups` route when the caller discards the public failed-open error. Public canonicalization also precedes the exclusive 1024-root capacity rejection.

The owner clarified that the previously approved 1024 bound began at pager acquisition and public metadata had its separate existing cap. This is therefore an uncovered public discovery boundary rather than evidence of a violation of that earlier narrow admission contract. The lead subsequently approved extending the exclusive increment to reserve before public canonicalization, transfer the same root once into the pager Session constructor, and retire/adopt on every failed public preflight return. This V2 correction is pending independent review. Earlier Reader public canonicalization remains separately scoped; this finding does not retroactively claim that accepted ReaderV5 covered it.

Independent V1 owner/lifecycle checks are allowed to continue. A failing public NUL-path runtime test was deliberately not executed because V1 would reach actual path handling; the finding follows directly from source calls and ownership return paths. V2 can safely use a full-admission NUL path test once reservation provably returns first, supplemented by descriptor-free early-return/one-shot transfer models.
