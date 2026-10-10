# File formats

M0 contains no database format implementation. Plaintext target: SQLite format 3, according to the [official specification](https://www.sqlite.org/fileformat2.html), with tested subsets recorded in compatibility.md. Future extension data must occupy legal transactional SQLite structures. Unknown feature bits, container versions, encodings, and index versions must be rejected instead of guessed.

Encrypted target: independent authenticated container, distinct from SQLite plaintext and SEE. No encrypted format is currently frozen or supported. Every format change requires an ADR, preserved snapshot, explicit migration and round-trip verification.
