package sqlite

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

// Query-only defaults depend on the selected index; an explicit probe count
// cannot silently affect HNSW or be clamped to an IVF index's list count.
func annIndexQueryOptions(x *extensionIndex, o VectorSearchOptions) (VectorSearchOptions, error) {
	if x.method != ann.IVFFlat {
		if o.Probes != 0 || o.MaxProbes != 0 {
			return o, fmt.Errorf("%w: probes require an IVFFlat index", vector.ErrOptions)
		}
		return o, nil
	}
	if o.Probes == 0 {
		o.Probes = 1
	}
	if o.MaxProbes == 0 {
		o.MaxProbes = x.ann.Lists
	}
	if o.Probes < 1 || o.Probes > x.ann.Lists || o.MaxProbes < o.Probes || o.MaxProbes > x.ann.Lists {
		return o, fmt.Errorf("%w: probes must be 1..lists and max_probes >= probes", vector.ErrOptions)
	}
	return o, nil
}

// Graph distance evaluations and centroid scores are disjoint diagnostics.
// Both consume the same cumulative component allowance as exact rescoring and
// any later exact fallback. They are never hidden in base-row scoring counts.
func annDistanceComponents(s *QueryStats, dims int) uint64 {
	return uint64(s.ANNDistanceEvaluations+s.ANNCentroidScores) * uint64(dims)
}

// IVF memberships contain source identity, not a duplicate vector. Guards and
// schema epochs protect source changes; the full checker verifies assignment.
// HNSW additionally checks the original node-vector bits using expected.
func decodeANNCandidate(value any, dims int, expected *vector.Vector) (vector.Vector, error) {
	if expected != nil {
		return decodeANNVector(value, dims, *expected)
	}
	v, null, err := decodeVectorColumn(value, dims)
	if err != nil {
		return vector.Vector{}, fmt.Errorf("%w: invalid IVF source vector: %w", ErrCorrupt, err)
	}
	if null {
		return vector.Vector{}, extensionCorrupt("IVF membership refers to NULL source vector")
	}
	return v, nil
}

// runIVF scores the bounded centroid set exactly once and opens only selected
// covering list ranges. Filter compensation extends that ordered list prefix,
// never rereads an earlier list, and cannot return a successful partial result.
func (a *annCandidateQuery) runIVF(ctx context.Context, store *annStore, state ann.State, visit func(int64, []any, *vector.Vector) (float64, bool, error)) ([]vector.Hit, string, error) {
	cfg := *a.index.ann
	if a.options.Probes == cfg.Lists {
		a.plan.ANNStopReason = "all_lists_requested"
		return nil, "probes equals lists", nil
	}
	if state.Live == 0 {
		a.plan.ANNStopReason = "empty"
		return nil, "", nil
	}
	retain := a.retain
	if state.Live < int64(retain) {
		retain = int(state.Live)
	}
	reserved := int64(a.query.Dims())*4 + int64(retain)*128 + a.retainedBytes()
	memory := min(cfg.MaxMemoryBytes, a.options.ANNLimits.MaxBytes, a.db.opts.MaxSortBytes-reserved)
	if memory < 1 {
		return nil, "", ErrMemoryLimit
	}
	if cfg.Lists > a.options.ANNLimits.MaxCentroidReads-int(a.stats.ANNCentroidReads) {
		return nil, "", fmt.Errorf("%w: cumulative centroid reads", vector.ErrLimit)
	}
	usedComponents := uint64(a.components()) + annDistanceComponents(a.stats, a.query.Dims())
	if usedComponents > a.limits.MaxComponents {
		return nil, "", fmt.Errorf("%w: cumulative vector components", vector.ErrLimit)
	}
	remainingDistances := a.options.ANNLimits.MaxDistanceEvaluations - int(a.stats.ANNDistanceEvaluations+a.stats.ANNCentroidScores)
	remainingDistances = int(min(uint64(max(0, remainingDistances)), (a.limits.MaxComponents-usedComponents)/uint64(a.query.Dims())))
	if remainingDistances < cfg.Lists {
		return nil, "", fmt.Errorf("%w: cumulative centroid scores/components", vector.ErrLimit)
	}
	centers, err := store.Centroids(ctx, memory)
	if err != nil {
		return nil, "", err
	}
	a.stats.ANNCentroidReads += int64(len(centers))
	ordered, work, err := ann.Probe(ctx, a.query, centers, cfg, ann.Limits{MaxVisited: min(cfg.MaxVisited, a.options.ANNLimits.MaxCentroidReads), MaxDistanceEvaluations: min(cfg.MaxDistanceEvaluations, remainingDistances), MaxBytes: min(cfg.MaxMemoryBytes, memory)})
	a.stats.ANNCentroidScores += int64(work.DistanceEvaluations)
	a.stats.ANNVisited += int64(work.Visited)
	a.stats.ANNPeakBytes = max(a.stats.ANNPeakBytes, work.PeakBytes+reserved)
	a.plan.ANNStopReason = work.StopReason
	if err != nil {
		return nil, "", err
	}
	// Include both the storage reader's conservative reservation and the
	// algorithm's centroid/probe-order reservation, without counting twice.
	centroidBytes := max(ann.CentroidMemoryBytes(cfg), int64(cfg.Lists)*int64(4*cfg.Dimensions+160))
	seen := make(map[int64]int)
	retained := annHitHeap{}
	memoryCheck := func(next bool) error {
		n := len(seen)
		if next {
			n++
		}
		bookkeeping := centroidBytes + int64(a.query.Dims())*4 + int64(retain)*128 + int64(n)*128
		actual := bookkeeping + a.retainedBytes()
		a.stats.ANNPeakBytes = max(a.stats.ANNPeakBytes, actual)
		if bookkeeping > min(cfg.MaxMemoryBytes, a.options.ANNLimits.MaxBytes) || actual > a.db.opts.MaxSortBytes {
			return ErrMemoryLimit
		}
		return nil
	}
	if err = memoryCheck(false); err != nil {
		return nil, "", err
	}
	opened, target := 0, a.options.Probes
	for {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if a.stats.ANNIterations >= int64(a.options.ANNLimits.MaxIterations) {
			return nil, "", fmt.Errorf("%w: ANN iterations", vector.ErrLimit)
		}
		a.stats.ANNIterations++
		for opened < target {
			listID := ordered[opened].ListID
			cur, err := store.Members(ctx, listID)
			if err != nil {
				return nil, "", err
			}
			a.stats.ANNListsOpened++
			opened++
			a.plan.ANNEffectiveProbes = opened
			err = func() error {
				defer cur.Close()
				for {
					key, err := cur.Next()
					if err == io.EOF {
						return nil
					}
					if err != nil {
						return err
					}
					a.stats.ANNMemberKeysRead++
					if a.stats.ANNMemberKeysRead > int64(a.options.ANNLimits.MaxMemberKeysRead) {
						return fmt.Errorf("%w: cumulative IVF member keys", vector.ErrLimit)
					}
					if len(key) != 4 || key[0] != a.index.id || key[1] != int64(listID) {
						return extensionCorrupt("IVF member covering key shape")
					}
					rowID, rok := key[2].(int64)
					physicalID, pok := key[3].(int64)
					if !rok || !pok || physicalID <= 0 {
						return extensionCorrupt("IVF member covering identity")
					}
					if _, duplicate := seen[rowID]; duplicate {
						return extensionCorrupt("duplicate IVF source identity")
					}
					if err := memoryCheck(true); err != nil {
						return err
					}
					seen[rowID] = listID
					if int64(len(seen)) > state.Live {
						return extensionCorrupt("IVF members exceed live count")
					}
					a.stats.Candidates++
					a.stats.ANNIdentityLookups++
					if err = store.CheckMember(ctx, listID, rowID, physicalID); err != nil {
						return err
					}
					if a.stats.BaseRowFetches >= int64(a.limits.MaxCandidates) {
						return fmt.Errorf("%w: cumulative source candidates", vector.ErrLimit)
					}
					raw, found, err := btree.SeekTable(ctx, a.reader, a.table.root, rowID)
					if err != nil {
						return err
					}
					if !found {
						return extensionCorrupt("IVF membership refers to missing source row")
					}
					distance, keep, err := visit(rowID, raw, nil)
					if err != nil {
						return err
					}
					if err = ctx.Err(); err != nil {
						return err
					}
					if err = memoryCheck(false); err != nil {
						return err
					}
					if keep {
						hit := vector.Hit{RowID: rowID, Distance: distance}
						if len(retained) < retain {
							heap.Push(&retained, hit)
						} else if annHitBetter(hit, retained[0]) {
							retained[0] = hit
							heap.Fix(&retained, 0)
						}
					}
				}
			}()
			if err != nil {
				return nil, "", err
			}
		}
		if opened == cfg.Lists && int64(len(seen)) != state.Live {
			return nil, "", extensionCorrupt("IVF exhausted memberships do not match live count")
		}
		if len(retained) >= retain {
			a.plan.ANNStopReason = "eligible_limit_satisfied"
			if retain < a.retain {
				a.plan.ANNStopReason = "all_live_identities_rechecked"
			}
			break
		}
		if opened == cfg.Lists {
			a.plan.ANNStopReason = "all_lists_exhausted"
			return nil, "all IVF lists exhausted before eligible limit", nil
		}
		if opened >= a.options.MaxProbes {
			a.plan.ANNStopReason = "max_probes"
			return nil, "insufficient eligible IVF hits at max_probes", nil
		}
		// Doubling bounds iteration overhead; each additional range remains in
		// the exact nearest-centroid order and is opened at most once.
		target = min(a.options.MaxProbes, max(opened+1, opened*2))
	}
	hits := append([]vector.Hit(nil), retained...)
	sort.Slice(hits, func(i, j int) bool { return annHitBetter(hits[i], hits[j]) })
	return hits, "", ctx.Err()
}

// A clean ANN catalog may tighten exact-fallback heap admission. Verify that
// trusted snapshot cardinality before each heap insertion and at exhaustion;
// a contradictory catalog must not underreserve memory or hide missing rows.
// A negative bound means native exact mode, which has no ANN metadata bound.
func checkANNExactSourceCount(count, bound int64, exhausted bool) error {
	if bound >= 0 && (count > bound || (exhausted && count != bound)) {
		return extensionCorrupt("ANN exact fallback source cardinality mismatch")
	}
	return nil
}
