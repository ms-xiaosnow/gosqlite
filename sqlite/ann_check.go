package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func (d *DB) checkANNIndex(ctx context.Context, reader btree.PageReader, x *extensionIndex) error {
	if x.method == ann.IVFFlat {
		return d.checkIVFIndex(ctx, reader, x)
	}
	source := x.catalog.entries[fold(x.sourceTable)]
	parsed, err := sqlast.Parse(source.SQL)
	if err != nil {
		return err
	}
	def, ok := parsed.(*sqlast.CreateTable)
	if !ok {
		return ErrUnsupported
	}
	tab, err := writableTable(def, source.RootPage)
	if err != nil {
		return err
	}
	helper := &Tx{db: d}
	if err = helper.validateANNField(x, tab); err != nil {
		return err
	}
	store := newANNStore(reader, x)
	state, err := store.GetState(ctx)
	if err != nil {
		return err
	}
	if err = state.Validate(*x.ann); err != nil {
		return fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	var sourceRows, liveRows int64
	cur, err := btree.NewCursor(ctx, reader, tab.root)
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
		sourceRows++
		values, err := tab.values(id, raw)
		if err != nil {
			return err
		}
		vec, null, err := decodeVectorColumn(values[x.fields[0].ordinal], x.ann.Dimensions)
		if err != nil {
			return err
		}
		nodeID, found, err := store.LiveNode(ctx, id)
		if err != nil {
			return err
		}
		if null {
			if found {
				return extensionCorrupt("NULL source has ANN live mapping")
			}
			continue
		}
		if err = ann.ValidateVector(vec, *x.ann); err != nil {
			return fmt.Errorf("%w: ANN source value: %w", ErrCorrupt, err)
		}
		if !found {
			return extensionCorrupt("ANN source missing live mapping")
		}
		node, err := store.GetNode(ctx, nodeID)
		if err != nil {
			return err
		}
		if !node.Live || node.RowID != id {
			return extensionCorrupt("ANN source/live node identity differs")
		}
		a, _ := vec.MarshalBinary()
		b, _ := node.Vector.MarshalBinary()
		if !bytes.Equal(a, b) {
			return extensionCorrupt("ANN source/node vector differs")
		}
		liveRows++
	}
	if sourceRows != x.documentCount || liveRows != state.Live {
		return extensionCorrupt("ANN source/state count differs")
	}
	cur.Close()
	var nodes, live, dead int64
	maxLevel := -1
	err = btree.Scan(ctx, reader, x.catalog.roots[annNodesName], func(physicalID int64, raw []any) error {
		if len(raw) != 8 || raw[0] != nil {
			return extensionCorrupt("ANN node table shape")
		}
		if raw[1] != x.id {
			return nil
		}
		nodes++
		if nodes > x.ann.MaxNodes {
			return ErrMemoryLimit
		}
		nodeID, ok := raw[2].(int64)
		if !ok || nodeID >= state.NextID {
			return extensionCorrupt("ANN node identity exceeds state")
		}
		indexedID, _, found, err := store.row(ctx, annNodesName, annNodesLookup, 2, []any{x.id, nodeID})
		if err != nil {
			return err
		}
		if !found || indexedID != physicalID {
			return extensionCorrupt("ANN orphan node table row")
		}
		node, err := store.GetNode(ctx, nodeID)
		if err != nil {
			return err
		}
		if node.Level > maxLevel {
			maxLevel = node.Level
		}
		mapped, exists, err := store.LiveNode(ctx, node.RowID)
		if err != nil {
			return err
		}
		if node.Live {
			live++
			if !exists || mapped != node.ID {
				return extensionCorrupt("ANN live node missing reverse source mapping")
			}
			_, sourceExists, err := btree.SeekTable(ctx, reader, tab.root, node.RowID)
			if err != nil {
				return err
			}
			if !sourceExists {
				return extensionCorrupt("ANN live source missing")
			}
		} else {
			dead++
			if exists && mapped == node.ID {
				return extensionCorrupt("ANN tombstone remains live-mapped")
			}
		}
		for layer, ids := range node.Neighbors {
			for _, id := range ids {
				target, err := store.GetNode(ctx, id)
				if err != nil {
					return err
				}
				if target.Level < layer {
					return extensionCorrupt("ANN edge target lacks layer")
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if nodes != state.NextID-1 || live != state.Live || dead != state.Tombstones || maxLevel != state.MaxLevel {
		return extensionCorrupt("ANN graph state totals differ")
	}
	if state.Entry != 0 {
		entry, err := store.GetNode(ctx, state.Entry)
		if err != nil {
			return err
		}
		if entry.Level != state.MaxLevel {
			return extensionCorrupt("ANN entry does not occupy maximum layer")
		}
	}
	for _, v := range []struct {
		table, index string
		columns      int
		expected     int64
	}{
		{annStateName, annStateLookup, 1, 1}, {annNodesName, annNodesLookup, 2, nodes}, {annLiveName, annLiveLookup, 2, live},
		{annCentroidsName, annCentroidsLookup, 2, 0}, {annMembersName, annMembersLookup, 3, 0},
		{annMembersName, annMembersReverse, 2, 0},
		{extensionPostingsName, extensionPostingsLookup, 6, 0}, {extensionFrequencyName, extensionFrequencyLookup, 5, 0},
	} {
		count, err := countExtensionRange(ctx, reader, x.catalog.roots[v.index], v.columns, x.id)
		if err != nil {
			return err
		}
		if count != v.expected {
			return extensionCorrupt("ANN shadow index count differs")
		}
		var tableCount int64
		err = btree.Scan(ctx, reader, x.catalog.roots[v.table], func(_ int64, row []any) error {
			if len(row) < 2 {
				return extensionCorrupt("ANN shadow table shape")
			}
			if row[1] == x.id {
				tableCount++
			}
			return nil
		})
		if err != nil {
			return err
		}
		if tableCount != v.expected {
			return extensionCorrupt("ANN orphan shadow table row")
		}
	}
	return nil
}
