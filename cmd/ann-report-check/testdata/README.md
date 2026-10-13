# Synthetic input provenance

`synthetic-schema3-hnsw.json` is a complete, compact JSON candidate used by the
stdin command-contract test and available as a user-callable example. Its
metadata is constructed. It is NOT a new benchmark, source/host attestation,
performance or recall acceptance, or a converter for old aggregate reports.

Its unchanged raw observations come from the archived V3 test fixture
`benchmarks/ann/testdata/schema2-hnsw.json` (90,643 bytes; SHA-256
`14f6fb98d01d05b650f08e54273278c9297c3db214c79f48a14875fd748009da`). The preserved
copy also lives under `benchmarks/ann/reportcheck/testdata` for legacy controls.

The source-only construction follows `modelMethodReport` in reportcheck tests:
schema/revisions are changed; admission wire estimates are added; the fixed
native limits, query/timing policies and canonical algorithm envelope replace
their schema2 forms; retained paths and the one-minute UTC interval are
synthetic; unavailable hardware notes are explained; workload/hardware/build
SHA-256 values are mechanically recalculated using the same ordered JSON
identity fields. Two explicit SYNTHETIC/provenance notes are appended. Raw
observations, summaries, measured-number fields and dataset hashes are copied,
not regenerated, rerun or claimed as current evidence.

This data transformation used JSON parsing/serialization and SHA-256 only.
No archived Go fixture constructor, validator, Go toolchain, native algorithm,
database, hardware inspection or measurement was executed to produce it.

The initial source-only candidate is 43,809 bytes, SHA-256
`d567c3bc01e42f4fb6de83dcc0de4ae50293f38e42f6638587234c3cb5dd6f41`.
Semantic validation is still NOT_RUN at this source checkpoint; the proposed
`TestANNReportCheckSyntheticStdinRoundTrip` exercises the complete
`run -> DecodeReport -> ValidatePass` path when separately authorized.

Subsequent verification 2026-10-13: complete stdin round-trip passed normal/race
with the same JSON bytes. See docs/evidence/20261013-ann-report-check.
It remains a synthetic example.
