# Security and release gates

This is a development project, not production-ready. M2 can modify a supported SQLite subset. It validates supported catalog grammar and whole-file page ownership, holds native Linux locks, bounds dirty pages and refuses layouts it cannot preserve. No crash/recovery durability qualification exists. Never use irreplaceable data with this development build. No security or legal review has been completed.

Encryption is planned as an independent AES-256-GCM authenticated container with unique nonces, authenticated database identity/page role/page number/generation, explicit raw-key versus passphrase modes, and bounded Argon2id parameters. Keys must enter only through code-configured providers, never argv, DSNs, logs, repository files, or examples with usable static secrets. No SEE private source is accessed.

AEAD cannot prevent whole-file historical rollback without external state; metadata sizes, access patterns, process memory and Go GC copies remain risks. Journals, WAL, spill, backups and rekey must participate in the threat model. Publishing a production-security claim requires independent review and the full negative/recovery gate.

Known platform-restricted crash/fault/process-exit workflows must not run here or be redirected to CI/another executor. Ordinary deterministic unit/read/write/rollback/BUSY/race and cross-process lock checks run at M2. Memory-only malformed-input checks do not exercise corrupt disk files. Manual verification instructions and unexecuted gates remain explicit.


A writable connection resolves its database path once; symlink/cwd changes must not redirect later operations to a different database. Physical pathname/inode stability is checked while a transaction is open. Shared snapshots are held by Rows/Tx lifetimes, not by idle connections. Index maintenance is atomic at statement-overlay level; operational failures during physical publication are conservatively poisoned and remain unqualified without restricted manual evidence. Journals are plaintext and mode0600; M2 provides no encryption/confidentiality protection.
