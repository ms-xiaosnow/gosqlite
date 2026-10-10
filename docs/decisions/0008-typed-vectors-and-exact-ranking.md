# ADR0008: typed dense persistence and bounded exact ranking

Date:2026-10-10. Status: implemented ordinary paths; release acceptance conditional.

VECTOR(n) remains legal SQLite schema text. Native mutations strictly validate finite float32 values and dimensions, storing the existing versioned GSV BLOB. Valid external SQLite TEXT/BLOB values are decoded at typed interpretation; reads retain physical SQL storage class, and ordinary SQL equality remains SQLite equality. No PostgreSQL page, wire or full type semantics are imported.

Native VectorSearch owns one source snapshot and applies business filtering before eligible-vector decoding/scoring. NULL candidates are excluded; undefined cosine zero norms error. SQL distance ordering preserves NULL-first ASC and only uses a bounded heap for one recognized ASC key with finite LIMIT. Unsupported ranking shapes retain the original general sorter. Underlying grams13 source access is independent of ranking; no vector index/ANN is claimed.

Metric identity is a closed built-in kind, not arbitrary callback naming. Exact scores use the documented float64 compensated arithmetic over float32 inputs. Native and optimized SQL paths have cumulative candidate/component/result budgets, with no partial exact success on error. Typed quota errors retain NoMem classification rather than being mislabeled datatype mismatch. SQL heap and fallback validate referenced typed values consistently even when the other distance operand is NULL.

The hybrid example uses one read transaction for text/vector/business filters and preserves raw scores, distances and RRF contributions. This closes the M5 ordinary hybrid code gap without claiming exact recall outside supplied ranking lists.

Tests use independent mathematics, retained public pgvector expected outputs, scalar/operator goldens, exact heap/full-sort comparisons, transactional normal-success persistence and official SQLite legal-schema/BLOB interoperability. Restricted physical recovery/crash/fault and independent native-OS/security/legal/soak gates remain separately open. HNSW/IVFFlat are future code, not aliases for this exact scan.
