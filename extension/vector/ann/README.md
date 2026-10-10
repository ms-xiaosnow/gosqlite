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
