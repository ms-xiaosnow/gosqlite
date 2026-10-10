# ADR0010: Write-once authenticated-record prerequisite

Status: experimental memory-only prerequisite. No encrypted database adapter or
stable encrypted format is implemented/accepted.

Use standard-library AES256GCM and HKDFSHA256; use pinned official
Go x/crypto Argon2id implementation rather than invent a KDF. This does not assert
that this project's composition has an independent audit. Separate explicit raw
keys/passwords and check all cost bounds before provider or expensive work.

Construct only fresh write-once file identities. Copies share nonce/counter state,
errors poison writers, and authenticated read handles cannot resume writing.
Derive independent file-role/record-role keys, authenticate full bootstrap plus
expected page/frame location, and return no plaintext on authentication failure.
Header identity for a child must be authenticated by a parent. Random FileIDs
and burned per-file sequences avoid resumed-counter reuse; whole-history rollback
still requires external trusted state to detect.

KeyProvider is a trusted code interface, never a DSN/SQL/argv secret mechanism.
Release borrowed material on every outcome, redact provider errors, bound password
copies and concurrent Argon2, and clear owned buffers best-effort. Go runtime and
expanded-key copies prevent an absolute erasure claim.

The bootstrap and record codec are deliberately separate from future root/footer,
WAL directory, commit completeness and filesystem publication semantics. A page
AEAD wrapper alone would leave direct journal/WAL/checkpoint and page-one paths
plaintext. Actual integration must cover all such paths or explicitly reject
unsupported modes. No codec test qualifies encrypted storage or crash safety.

Known-answer, race and1,400,866 actual pure-memory fuzz executions passed. No
physical tamper/fault/recovery workflow was executed. Independent cryptographic,
legal, native-OS and release gates remain open.
