package sqlite

import (
	"context"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func (d *DB) checkIVFIndex(ctx context.Context, reader btree.PageReader, x *extensionIndex) error {
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
	cfg := *x.ann
	if cfg.MaxMemoryBytes > d.opts.MaxSortBytes {
		cfg.MaxMemoryBytes = d.opts.MaxSortBytes
	}
	if cfg.MaxMemoryBytes < 1024 {
		return ErrMemoryLimit
	}
	ready := state.TrainingState == "ready"
	var centers []ann.TypedCentroid
	var counts []int64
	if ready {
		countBytes := int64(cfg.Lists) * 16
		if countBytes > cfg.MaxMemoryBytes {
			return ErrMemoryLimit
		}
		counts = make([]int64, cfg.Lists)
		centers, err = store.Centroids(ctx, cfg.MaxMemoryBytes-countBytes)
		if err != nil {
			return err
		}
	}
	cur, err := btree.NewCursor(ctx, reader, tab.root)
	if err != nil {
		return err
	}
	defer cur.Close()
	var sourceRows, live, work int64
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
		v, null, err := decodeANNColumn(values[x.fields[0].ordinal], cfg)
		if err != nil {
			return err
		}
		list, found, err := store.MemberList(ctx, id)
		if err != nil {
			return err
		}
		if null {
			if found {
				return extensionCorrupt("NULL source has IVF membership")
			}
			continue
		}
		if err = ann.ValidateValue(v, cfg); err != nil {
			return fmt.Errorf("%w: IVF source vector: %w", ErrCorrupt, err)
		}
		live++
		if !ready {
			if found {
				return extensionCorrupt("untrained IVF has memberships")
			}
			continue
		}
		if !found {
			return extensionCorrupt("IVF source missing membership")
		}
		remaining := (cfg.MaxTrainingWork - work) / int64(max(1, v.WorkUnits()))
		if remaining < 1 {
			return annWriteError(fmt.Errorf("%w: IVF integrity assignment work", ann.ErrLimit))
		}
		limit := cfg.MaxDistanceEvaluations
		if remaining < int64(limit) {
			limit = int(remaining)
		}
		expected, stats, err := ann.AssignValue(ctx, v, centers, cfg, ann.TypedLimits{Limits: ann.Limits{MaxDistanceEvaluations: limit}})
		work += stats.ComponentWork
		if err != nil {
			return annWriteError(err)
		}
		if expected != list {
			return extensionCorrupt("IVF source assigned to wrong nearest centroid")
		}
		key, found, err := extensionRangeOne(ctx, reader, x.catalog.roots[annMembersLookup], 3, []any{x.id, int64(list), id})
		if err != nil {
			return err
		}
		if !found || len(key) != 4 {
			return extensionCorrupt("IVF source missing covering membership")
		}
		physical, ok := key[3].(int64)
		if !ok {
			return extensionCorrupt("IVF member physical identity")
		}
		if err = store.CheckMember(ctx, list, id, physical); err != nil {
			return err
		}
		counts[list-1]++
	}
	if sourceRows != x.documentCount || live != state.Live {
		return extensionCorrupt("IVF source/state counts differ")
	}
	if err = cur.Close(); err != nil {
		return err
	}
	expectedCenters, expectedMembers := int64(0), int64(0)
	if ready {
		expectedCenters = int64(cfg.Lists)
		expectedMembers = live
		for i, count := range counts {
			actual, err := store.centroidCount(ctx, i+1)
			if err != nil {
				return err
			}
			if actual != count {
				return extensionCorrupt("IVF centroid/member count differs")
			}
		}
	}
	for _, v := range []struct {
		table, index string
		columns      int
		expected     int64
	}{
		{annStateName, annStateLookup, 1, 1},
		{annNodesName, annNodesLookup, 2, 0}, {annLiveName, annLiveLookup, 2, 0},
		{annCentroidsName, annCentroidsLookup, 2, expectedCenters},
		{annMembersName, annMembersLookup, 3, expectedMembers}, {annMembersName, annMembersReverse, 2, expectedMembers},
		{extensionPostingsName, extensionPostingsLookup, 6, 0}, {extensionFrequencyName, extensionFrequencyLookup, 5, 0},
	} {
		count, err := countExtensionRange(ctx, reader, x.catalog.roots[v.index], v.columns, x.id)
		if err != nil {
			return err
		}
		if count != v.expected {
			return extensionCorrupt("IVF shadow index count differs")
		}
		var actual int64
		err = btree.Scan(ctx, reader, x.catalog.roots[v.table], func(_ int64, row []any) error {
			if len(row) < 2 {
				return extensionCorrupt("IVF shadow table shape")
			}
			if row[1] == x.id {
				actual++
			}
			return nil
		})
		if err != nil {
			return err
		}
		if actual != v.expected {
			return extensionCorrupt("IVF orphan shadow table rows")
		}
	}
	return nil
}
