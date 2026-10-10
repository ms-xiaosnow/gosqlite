# Security and release gates

This is a development project, not production-ready. M1 reads supported SQLite files without modifying them; all writes remain disabled. Future plaintext writes must reject unsupported layouts, preserve all owned pages and obtain correct filesystem locks. No security or legal review has been completed.

Encryption is planned as an independent AES-256-GCM authenticated container with unique nonces, authenticated database identity/page role/page number/generation, explicit raw-key versus passphrase modes, and bounded Argon2id parameters. Keys must enter only through code-configured providers, never argv, DSNs, logs, repository files, or examples with usable static secrets. No SEE private source is accessed.

AEAD cannot prevent whole-file historical rollback without external state; metadata sizes, access patterns, process memory and Go GC copies remain risks. Journals, WAL, spill, backups and rekey must participate in the threat model. Publishing a production-security claim requires independent review and the full negative/recovery gate.

Known platform-restricted crash/fault/process-exit workflows must not run here or be redirected to CI/another executor. Ordinary deterministic unit/read-only/race checks run at M1. Later ordinary read/write transaction checks may run when implemented. Manual verification instructions and unexecuted gates remain explicit.
