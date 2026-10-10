package sqlite

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func (t *Tx) validateANNField(x *extensionIndex, tab *writeTable) error {
	if x.ann == nil || len(x.fields) != 1 {
		return extensionCorrupt("ANN field definition")
	}
	f := x.fields[0]
	if f.ordinal < 0 || f.ordinal >= len(tab.columns) {
		return fmt.Errorf("%w: ANN field changed", ErrReindexRequired)
	}
	dims, ok := sqlast.VectorDimensions(tab.columns[f.ordinal].Type)
	if !ok || dims != x.ann.Dimensions {
		return fmt.Errorf("%w: ANN vector declaration changed", ErrReindexRequired)
	}
	return nil
}
func annWriteError(err error) error {
	if errors.Is(err, ann.ErrLimit) {
		return fmt.Errorf("%w: %w", ErrMemoryLimit, err)
	}
	if errors.Is(err, ann.ErrCorrupt) {
		return fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	return err
}
func (t *Tx) applyANNDelta(ctx context.Context, c *writeCatalog, x *extensionIndex, oldID int64, old []any, newID int64, next []any) error {
	ordinal := x.fields[0].ordinal
	var before, after vector.Vector
	var beforeNull, afterNull = true, true
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
			return fmt.Errorf("%w: indexed vector: %w", ErrConstraint, err)
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
	cfg := *x.ann
	if cfg.MaxMemoryBytes > t.db.opts.MaxSortBytes {
		cfg.MaxMemoryBytes = t.db.opts.MaxSortBytes
	}
	if cfg.MaxMemoryBytes < 1024 {
		return ErrMemoryLimit
	}
	if !beforeNull {
		nodeID, exists, err := store.LiveNode(ctx, oldID)
		if err != nil {
			return err
		}
		if !exists {
			return extensionCorrupt("ANN old source missing live mapping")
		}
		node, err := store.GetNode(ctx, nodeID)
		if err != nil {
			return err
		}
		if !node.Live || node.RowID != oldID {
			return extensionCorrupt("ANN old source identity differs")
		}
		a, _ := node.Vector.MarshalBinary()
		b, _ := before.MarshalBinary()
		if !bytes.Equal(a, b) {
			return extensionCorrupt("ANN old source vector differs")
		}
		if err = ann.Tombstone(ctx, store, &state, nodeID, cfg); err != nil {
			return annWriteError(err)
		}
		if err = store.removeLive(ctx, oldID); err != nil {
			return err
		}
	}
	if !afterNull {
		nodeID := state.NextID
		if err = ann.Insert(ctx, store, &state, newID, after, cfg); err != nil {
			return annWriteError(err)
		}
		if err = store.addLive(ctx, newID, nodeID); err != nil {
			return err
		}
	}
	return store.putState(ctx, state)
}
func (t *Tx) clearANNRows(ctx context.Context, c *writeCatalog, x *extensionIndex) error {
	for _, item := range []struct {
		table, index string
		columns      int
	}{
		{annNodesName, annNodesLookup, 2}, {annLiveName, annLiveLookup, 2},
		{annCentroidsName, annCentroidsLookup, 2}, {annMembersName, annMembersLookup, 3},
		{annStateName, annStateLookup, 1},
	} {
		if err := t.clearExtensionRows(ctx, c, x, item.table, item.index, item.columns); err != nil {
			return err
		}
	}
	return nil
}
