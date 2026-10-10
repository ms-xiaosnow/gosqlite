package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

func (t *Tx) ivfOperationConfig(x *extensionIndex) (ann.Config, error) {
	cfg := *x.ann
	if cfg.MaxMemoryBytes > t.db.opts.MaxSortBytes {
		cfg.MaxMemoryBytes = t.db.opts.MaxSortBytes
	}
	if cfg.MaxMemoryBytes < 1024 {
		return ann.Config{}, ErrMemoryLimit
	}
	return cfg, cfg.Validate()
}
func (t *Tx) buildIVF(ctx context.Context, c *writeCatalog, x *extensionIndex, tab *writeTable) error {
	if err := t.validateANNField(x, tab); err != nil {
		return err
	}
	cfg, err := t.ivfOperationConfig(x)
	if err != nil {
		return err
	}
	sampler, err := ann.NewIVFSampler(cfg)
	if err != nil {
		return annWriteError(err)
	}
	state := ann.State{NextID: 1, MaxLevel: -1, TrainingState: "untrained"}
	x.documentCount = 0
	cur, err := btree.NewCursor(ctx, t.reader, tab.root)
	if err != nil {
		return err
	}
	defer cur.Close()
	for {
		id, raw, err := cur.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if x.documentCount == math.MaxInt64 {
			return ErrFull
		}
		x.documentCount++
		values, err := tab.values(id, raw)
		if err != nil {
			return err
		}
		v, null, err := decodeVectorColumn(values[x.fields[0].ordinal], cfg.Dimensions)
		if err != nil {
			return err
		}
		if null {
			continue
		}
		if err = ann.ValidateVector(v, cfg); err != nil {
			return fmt.Errorf("%w: indexed IVF vector: %w", ErrConstraint, err)
		}
		if state.Live >= cfg.MaxNodes {
			return annWriteError(fmt.Errorf("%w: IVF eligible rows", ann.ErrLimit))
		}
		state.Live++
		if err = sampler.Add(ctx, id, v); err != nil {
			return annWriteError(fmt.Errorf("indexed IVF vector: %w", err))
		}
	}
	if err = cur.Close(); err != nil {
		return err
	}
	samples, err := sampler.Samples(ctx)
	if err != nil {
		return annWriteError(err)
	}
	centroids, work, ready, err := ann.Train(ctx, samples, cfg)
	if err != nil {
		return annWriteError(err)
	}
	samples = nil
	sampler = nil
	store := newANNWriteStore(t, c, x)
	if !ready {
		return store.putState(ctx, state)
	}
	for _, centroid := range centroids {
		if err = store.putCentroid(ctx, centroid, 0); err != nil {
			return err
		}
	}
	// Cluster centers are fixed before assignment, but readiness is published only
	// after every membership/count and source metadata mutation succeeds.
	cur, err = btree.NewCursor(ctx, t.reader, tab.root)
	if err != nil {
		return err
	}
	defer cur.Close()
	assigned := int64(0)
	used := work.ComponentWork
	for {
		id, raw, err := cur.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		values, err := tab.values(id, raw)
		if err != nil {
			return err
		}
		v, null, err := decodeVectorColumn(values[x.fields[0].ordinal], cfg.Dimensions)
		if err != nil {
			return err
		}
		if null {
			continue
		}
		remaining := (cfg.MaxTrainingWork - used) / int64(cfg.Dimensions)
		if remaining < 1 {
			return annWriteError(fmt.Errorf("%w: IVF assignment component work", ann.ErrLimit))
		}
		limit := cfg.MaxDistanceEvaluations
		if remaining < int64(limit) {
			limit = int(remaining)
		}
		list, stats, err := ann.Assign(ctx, v, centroids, cfg, ann.Limits{MaxDistanceEvaluations: limit})
		used += int64(stats.DistanceEvaluations) * int64(cfg.Dimensions)
		if err != nil {
			return annWriteError(err)
		}
		if err = store.addMember(ctx, list, id); err != nil {
			return err
		}
		assigned++
	}
	if assigned != state.Live {
		return extensionCorrupt("IVF source changed during build")
	}
	state.TrainingState = "ready"
	return store.putState(ctx, state)
}
func (t *Tx) applyIVFDelta(ctx context.Context, c *writeCatalog, x *extensionIndex, oldID int64, old []any, newID int64, next []any) error {
	ordinal := x.fields[0].ordinal
	var before, after vector.Vector
	beforeNull, afterNull := true, true
	var err error
	if old != nil {
		before, beforeNull, err = decodeVectorColumn(old[ordinal], x.ann.Dimensions)
		if err != nil {
			return err
		}
	}
	if next != nil {
		after, afterNull, err = decodeVectorColumn(next[ordinal], x.ann.Dimensions)
		if err != nil {
			return err
		}
	}
	if !afterNull {
		if err = ann.ValidateVector(after, *x.ann); err != nil {
			return fmt.Errorf("%w: indexed IVF vector: %w", ErrConstraint, err)
		}
	}
	if beforeNull && afterNull {
		return nil
	}
	if oldID == newID && !beforeNull && !afterNull {
		a, _ := before.MarshalBinary()
		b, _ := after.MarshalBinary()
		if bytes.Equal(a, b) {
			return nil
		}
	}
	store := newANNWriteStore(t, c, x)
	state, err := store.GetState(ctx)
	if err != nil {
		return err
	}
	if !beforeNull {
		state.Live--
	}
	if !afterNull {
		state.Live++
	}
	if state.Live < 0 {
		return extensionCorrupt("IVF eligible count underflow")
	}
	if state.Live > x.ann.MaxNodes {
		return annWriteError(fmt.Errorf("%w: IVF eligible rows", ann.ErrLimit))
	}
	if state.TrainingState == "untrained" {
		return store.putState(ctx, state)
	}
	cfg, err := t.ivfOperationConfig(x)
	if err != nil {
		return err
	}
	if !beforeNull {
		list, found, err := store.MemberList(ctx, oldID)
		if err != nil {
			return err
		}
		if !found {
			return extensionCorrupt("IVF old source missing membership")
		}
		if err = store.removeMember(ctx, list, oldID); err != nil {
			return err
		}
	}
	if !afterNull {
		centers, err := store.Centroids(ctx, cfg.MaxMemoryBytes)
		if err != nil {
			return err
		}
		list, _, err := ann.Assign(ctx, after, centers, cfg, ann.Limits{})
		if err != nil {
			return annWriteError(err)
		}
		if err = store.addMember(ctx, list, newID); err != nil {
			return err
		}
	}
	return store.putState(ctx, state)
}
