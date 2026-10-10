# Transaction-owned ANN algorithms

This independently written pure Go package owns no files, SQL, transaction
commits, source-row storage, or process execution. Its HNSW reader/writer ports
and IVFFlat sampler/training/centroid routines are algorithm components. SQLite
integration is responsible for durable atomic publication and list membership.
Algorithm tests alone do not qualify persistence or physical recovery.

## IVFFlat version 1

The closed method accepts L2, negative inner product and cosine, dimensions
1..2000 and lists1..32768 (default100); L1 is rejected. Every new option and
local build policy is persisted in a method-specific canonical JSON envelope.
HNSW's previously persisted canonical bytes and hashes are unchanged. Unknown
fields, noncanonical bytes, altered policy strings and versions reject.

Creation defaults: at most10000 training samples (increased to lists when the
caller specifies more lists and leaves the sample cap unspecified),20 Lloyd
iterations,2^28 training component-work units,32MiB retained logical memory,
100000 eligible source rows,32768 centroid visits,1000000 assignment/query
centroid distance evaluations. TrainingSamples permits1..2^20 but must be at
least lists; iterations1..1000; component work1..2^40. Other inherited limits
remain independently validated. These are local resource policies, not pgvector
session settings. Quotas can explicitly reject otherwise valid large builds.

### Sampling and training

NewIVFSampler is a bounded streaming bottom-hash reservoir. Add expects
strictly increasing signed source rowids, validates all indexed vectors and
retains the smallest (SplitMix64-derived seed/rowid priority, signed rowid)
pairs. It does not allocate the sample cap up front. Samples drains it and
returns the selected original immutable vectors in signed-rowid order.

Train also accepts an arbitrary-order bounded Sample slice, sorts an internal
copy by signed rowid and rejects duplicate identities. It does not mutate
original vectors. The smallest hash/rowid chooses the first seed; subsequent
seeds are the remaining sample farthest from the nearest existing seed, with
signed-rowid ties. Distinct sample identities are selected even when vectors
are identical. This is seeded farthest-first initialization, not k-means++.

L2 uses squared Euclidean assignment and ordinary arithmetic means. IP and
cosine use nonzero sample directions normalized in float64 and normalized
arithmetic mean directions, equivalently spherical k-means. Reductions are
compensated float64 in fixed rowid order. Each iteration performs genuine
nearest-center reassignment and mean improvement; ties choose lowest list ID.
Convergence means assignments are unchanged after a previous mean update;
otherwise training stops successfully at its persisted iteration cap. Output
is float32 centroids with IDs1..lists. Spherical norms must be within1e-6 of1,
a tolerance for float32 serialization, not normalization of source vectors.

An empty cluster retains its previous centroid. An exactly canceling spherical
sum also retains its previous unit centroid. Repeated input directions may
therefore leave empty or duplicate lists; these are valid, visible outcomes.
No random or all-zero replacement centroid is manufactured. Stats count these
occurrences. Different seeds can select different valid local optima/list IDs.

Fewer trainable samples than lists returns ready=false with no centroids and
an explicit insufficient_trainable_samples reason, including empty input at
2000 dimensions without reserving the full training cap. The database caller
must persist untrained state and use explained exact searches until explicit
REINDEX. IP zeros are valid indexed values, excluded only from training; once
trained, their equal centroid scores assign them to list1. Cosine zeros are
rejected for every indexed value, preserving the exact layer's error policy.

### Assignment, probing and accounting

Assign and Probe use the exact built-in metric on original source/query values
and stored centroids, then list ID ties. Probe returns centroid order only;
SQLite must open the selected membership B-tree ranges. Final candidate rank
must use original source vectors, especially for IP: unit training directions
must never replace the original magnitudes. Exact centroid/candidate scores do
not imply exact ANN recall.

Config bounds all operations. Per-call Limits may only tighten assignment/
probe quotas. TrainingMemoryBytes includes retained original samples, copied
float64 samples, seed/assignment arrays, compensated sums, centroid arrays,
float32 output and transient conversion allowances. SampleMemoryBytes accounts
for reservoir/sorted-input overhead, including backing-array capacity. These
are conservative logical retained-memory budgets, not Go allocator/RSS claims;
the surrounding pager/storage budget is additional. Empty/insufficient training
uses only actual input-size accounting before it can reserve centroid arrays.

ComponentWork charges vector dimensions per training normalization, distance,
reduction or conversion stage; immutable-vector validation/copy bookkeeping is
not a separate charge. Sorting/heap comparison work is bounded by sample/list
caps. Assignment/probe Stats count actual centroid scores as
DistanceEvaluations and examined centroids as Visited; graph NodeReads and
EdgeReads stay zero. PeakBytes includes caller-retained centroid vectors and
ordering storage. The storage caller separately counts actual shadow reads.
Cancellation is checked before/after stages and during bounded loops. Errors
and exhausted resource budgets return no successful partial centroids/results.

### Reference and scope

Public semantic reference: pgvector v0.8.7, commit
f37c13f68b57d2c3472b2214fbcff699d6d34876. The separately pinned public
src/ivfkmeans.c SHA-256 is
cf17de636966ee4af897d439b7fcc3893b56c5ff7a5e60146b78657f849061a6;
src/ivfflat.h is
be00dd3383cc4921fa3e252c16b7bf1f6e418f32b8c8b08f2a0070cb76af399b.
The public reference confirms metric families, spherical IP/cosine training and
list limits. No C code is copied or linked. This implementation deliberately
uses bounded deterministic Lloyd/farthest-first, rather than copying upstream
Elkan/k-means++/random-empty-center behavior. No PostgreSQL binary index or
bit-for-bit centroid/topology equivalence is claimed.

## Typed ANN version 2

Dense `Config` remains method version 1. The separate `TypedConfig` embeds
Config and adds Kind; an empty Kind explicitly selects the unchanged dense v1
identity. Existing canonical HNSW
and IVF JSON, raw GSV source payloads, GHA-v1 adjacency, signed-rowid ties,
metrics and graph policies remain unchanged. Every original dense production file and preexisting algorithm test remains
unchanged. Config, Node, Sample, Centroid, Limits and Stats preserve their exact
public field order and types, including positional/unkeyed literal compatibility;
the old function signatures, defaults, memory formulas and behavior remain intact.
An external-package compile fixture fixes this source-compatibility boundary.
Separate TypedNode/TypedSample/TypedCentroid structs carry Value, while
TypedReader/TypedWriter own typed graph storage. TypedLimits and TypedStats embed
their legacy counterparts and add component-work accounting. The immutable Value
union has explicit DenseValue, HalfValue, BitValue and SparseValue constructors,
concrete accessors, validation and codec dispatch. ParseConfig stays dense-only;
ParseTypedConfig accepts either legacy dense-v1 or typed-v2 canonical identity.

Typed configurations have version 2 and a separate canonical envelope containing
value kind, value codec, identity-transform policy, zero policy, sparse NNZ cap
and centroid policy. No type or metric is inferred from a distance callback.
DecodeValue selects the exact GSH-v1 / GSB-v1 / GSS-v1 codec from the config and
bounds payload size before decoding. No bytes, fields, schema assumptions or
hashes are retroactively added to dense-v1 identities. Typed adjacency remains
GHA-v1 because graph-edge structure is unchanged; its owning typed-v2 config
and explicitly tagged value payload supply representation identity.

Supported tuples follow the pinned pgvector v0.8.7 profile:

| Kind | HNSW | IVFFlat | Indexed value bounds |
|---|---|---|---|
| vector | L2, negative IP, cosine, L1 | L2, negative IP, cosine | 1..2000 dimensions |
| halfvec | L2, negative IP, cosine, L1 | L2, negative IP, cosine | 1..4000 dimensions |
| bit | Hamming, Jaccard | Hamming | 1..64000 bits |
| sparsevec | L2, negative IP, cosine, L1 | rejected | 1..1e9 dimensions, at most 1000 nonzeros |

OpclassFor binds each tuple to vector_*, halfvec_*, bit_* or sparsevec_* operator
classes. Unsupported kinds, metrics, opclasses, versions and changed policies
fail closed. IVFFlat L1, Jaccard and sparse vectors remain unsupported. Native
empty bit values remain valid in the vector package but have no positive-width
ANN configuration. Cosine half/sparse zeros reject, matching dense policy.
Hamming/Jaccard zero bit values are included, preserving the exact bit metric's
Jaccard zero-intersection distance of one.

### Actual typed algorithms and output policy

InsertValue/SearchValue implement the same genuine deterministic multilayer HNSW,
diversity pruning, tombstone traversal and signed-rowid tie ordering as dense.
The typed path shares the original deterministic level, candidate-heap and tie
helpers without changing dense graph storage/API types. A dense bridge regression
compares complete node topology, GHA-v1 bytes, candidates and IVF centroids between
legacy and typed entry points.
Typed distances never expand sparse dimensions, convert bits to numerics, or
silently change the selected metric. Sparse node memory is 8 bytes per nonzero
plus the bounded node/adjacency allowance, even at declared dimension 1e9.

TrainValues accepts TypedSample.Value and returns TypedCentroid.Value.
NewTypedIVFSampler retains typed samples; the legacy Train and NewIVFSampler
remain unchanged. Half input
components widen exactly, then use the existing compensated float64
Lloyd/spherical implementation. Only derived centroids convert float64 to
float32 and then round to binary16, nearest/ties-even; original half source
payloads and magnitudes stay unchanged. Spherical half centroids allow norm
error 1e-3 from this output rounding, versus dense's unchanged 1e-6. Assignment
and probing use exact half metric semantics on original source/query halves
and the persisted GSH centers. No normalized training copy becomes source data.

Bit IVF performs genuine packed-Hamming farthest-first seeding and repeated
nearest-center assignment followed by per-bit majority reduction. A tied bit
retains the previous centroid bit; an empty cluster retains its full prior
centroid. This can generate a centroid absent from the inputs and is not a
nearest-exemplar substitute. All-zero samples are trainable. Bits remain packed
outside a bounded uint32 per-list/per-bit count workspace. Original packed
sources stay unchanged. AssignValue and ProbeValue score fixed stored centers;
the storage integration owns selected-list membership reads and final rescoring.

### Typed resources, cancellation and evidence

TypedStats.ComponentWork counts metric attempts using numeric dimensions, packed
bytes for bit, and the sum of the two sparse NNZ counts. This is support work,
not dimension-times-distance for sparse values. TypedLimits.MaxComponentWork adds
an optional per-call ceiling; ComponentWorkLimited makes zero an explicit
zero-work ceiling. Admission occurs before each metric call. Zero-support sparse
pairs can score under a zero component budget. Errors return no partial result.
Typed exact metric loops check cancellation between at most 256 components or
packed bytes; bit training also checks between 256 reduction/update bits.
Numeric training retains its bounded per-vector cancellation stages.

Bit training charges packed bytes for distance/copy/output stages and visited
bits for majority reduction/update. Numeric training retains dimension charging.
Validation and allocation/clearing bookkeeping are excluded, as in the legacy
contract. Typed memory helpers include larger fixed allowances for inline Value
unions: a reservoir sample reserves source component storage plus 384 bytes;
centroid probing reserves source-format bytes plus 256 per centroid and 1024
base bytes. Training separately includes original/copied samples, scratch and
output. Legacy dense memory helpers and accounting remain unchanged. TypedNodeMemoryBytes
adds 128 bytes per node to the legacy fixed allowance for its larger inline Value;
the SQLite shared typed path must charge this extra even for DenseValue. The
separate typed entry points can reject a borderline memory budget earlier. These remain logical
retained-memory bounds, not allocator/RSS guarantees.

Focused pure-memory tests cover every supported tuple, canonical metadata and
codec mismatch, max dimensions/NNZ, 1e9-dimensional support-bounded HNSW, explicit
zero/exact work ceilings, source preservation, bit majority/tie/empty-cluster
policy, typed mutation/tombstone routing, and ordinary cancellation. Existing
legacy algorithm tests remain in the suite. No physical file faults, recovery,
process crash, tamper, security or release qualification is performed here.

Seeded held-out algorithm measurements (2026-10-10; tiny synthetic data, no
latency or speedup claim): HNSW uses PCG(20261010,19), 384 rows, 16 queries,
ef=100, 12 numeric/support components or 63 packed bits. All ten typed tuples
measured recall@10=1.000, while mean node reads were 357..382 of 384, so this
is not evidence of substantial work reduction. IVF uses PCG(20261010,27), 512
rows, 16 queries, lists=24 and probes=20. Half L2 recall@10=.99375, half IP and
cosine=1.000, bit Hamming=.96250; mean original-source candidate rescoring was
429..440 of 512. This high-probe result does not establish recall for defaults,
other datasets, persistent SQL paths, or a universal recall guarantee.

Reproduce after behavioral source review with the pinned toolchain and offline
module resolution:

```
CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/tmp/typed-ann-go-cache ../toolchains/go1.27.2/bin/go run ./internal/testscope
CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/tmp/typed-ann-go-cache ../toolchains/go1.27.2/bin/go test ./extension/vector/ann -count=1 -v
```
