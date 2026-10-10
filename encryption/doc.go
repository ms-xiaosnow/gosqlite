// Package encryption provides experimental, memory-only authenticated-record
// building blocks for an independent encrypted container. It does not open
// databases, persist files, publish snapshots, implement WAL, or perform rekey
// or backup operations. This is not SQLite or SEE ciphertext compatibility.
//
// The suite is AES-256-GCM (96-bit nonce, full 128-bit tag), HKDF-SHA-256,
// and, for explicitly selected password credentials, bounded Argon2id v0x13.
// Each writer owns a fresh random FileID and role-separated keys. Record
// sequences are allocated internally and a writer can never be reopened or
// resumed. Copying a handle shares its state; it does not copy a nonce counter.
// An error while sealing permanently disables that writer.
//
// Callers must authenticate a file's metadata with OpenFile before interpreting
// it. Subsequent Open calls require the expected record identity from trusted
// geometry or an authenticated directory. A successful record authentication
// does not establish file completeness, root-manifest authority, currentness,
// database structural validity, or protection against whole-snapshot rollback.
// Those checks belong to the future storage adapter. Metadata/footer payloads
// are opaque here; no root manifest or WAL directory format is implemented.
//
// The versioned byte encoding is experimental and has not been reviewed or
// frozen for release. Headers disclose identity, KDF costs and logical extent.
// Page numbers and WAL frame routing are not encoded in the public record
// header; the authenticated storage layer must supply them independently.
//
// Secret buffers owned by this package are cleared on release as a best effort.
// The Go runtime and crypto implementations may retain copies or expanded keys;
// no absolute memory erasure, FIPS, independent audit, or production-security
// claim is made. Credentials must be supplied in code, never DSNs, SQL or argv.
package encryption
