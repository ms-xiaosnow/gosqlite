package sqlite

import (
	"context"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

// Centroids reads only this index's bounded centroid range. It never scans
// source vectors or list memberships. Each logical list has one ordinary row.
func (s *annStore) Centroids(ctx context.Context, maxBytes int64) ([]ann.Centroid, error) {
	cfg := s.x.ann
	need := int64(cfg.Lists) * (int64(cfg.Dimensions)*4 + 160)
	if need > maxBytes {
		return nil, ErrMemoryLimit
	}
	prefix := []any{s.x.id}
	cur, err := btree.NewIndexRangeCursor(ctx, s.reader, s.x.catalog.roots[annCentroidsLookup], btree.KeyInfo{Columns: 2}, prefix, prefix)
	if err != nil {
		return nil, err
	}
	defer cur.Close()
	var result []ann.Centroid
	for {
		key, err := cur.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(key) != 3 || key[0] != s.x.id {
			return nil, extensionCorrupt("IVF centroid lookup key")
		}
		listID, lok := key[1].(int64)
		physicalID, pok := key[2].(int64)
		if !lok || !pok || listID != int64(len(result)+1) || listID > int64(cfg.Lists) || physicalID <= 0 {
			return nil, extensionCorrupt("IVF centroid list identity")
		}
		row, exists, err := btree.SeekTable(ctx, s.reader, s.x.catalog.roots[annCentroidsName], physicalID)
		if err != nil {
			return nil, err
		}
		centroid, _, err := s.decodeCentroid(row, exists, int(listID))
		if err != nil {
			return nil, err
		}
		result = append(result, centroid)
	}
	if len(result) != cfg.Lists {
		return nil, extensionCorrupt("IVF trained centroid count")
	}
	return result, nil
}
func (s *annStore) decodeCentroid(row []any, exists bool, listID int) (ann.Centroid, int64, error) {
	if !exists || len(row) != 5 || row[0] != nil || row[1] != s.x.id || row[2] != int64(listID) {
		return ann.Centroid{}, 0, extensionCorrupt("IVF centroid table/index mismatch")
	}
	encoded, ok := row[3].([]byte)
	count, cok := row[4].(int64)
	if !ok || !cok || count < 0 || count > s.x.ann.MaxNodes {
		return ann.Centroid{}, 0, extensionCorrupt("IVF centroid row shape")
	}
	vec, err := vector.Decode(encoded)
	if err != nil || vec.Dims() != s.x.ann.Dimensions {
		return ann.Centroid{}, 0, fmt.Errorf("%w: IVF centroid vector", ErrCorrupt)
	}
	centroid := ann.Centroid{ID: listID, Vector: vec}
	if err := ann.ValidateCentroid(centroid, *s.x.ann); err != nil {
		return ann.Centroid{}, 0, fmt.Errorf("%w: IVF centroid: %w", ErrCorrupt, err)
	}
	return ann.Centroid{ID: listID, Vector: vec}, count, nil
}
func (s *annStore) putCentroid(ctx context.Context, centroid ann.Centroid, count int64) error {
	if centroid.ID < 1 || centroid.ID > s.x.ann.Lists || count < 0 || count > s.x.ann.MaxNodes {
		return extensionCorrupt("IVF centroid identity/count")
	}
	blob, err := centroid.Vector.MarshalBinary()
	if err != nil {
		return err
	}
	values := []any{nil, s.x.id, int64(centroid.ID), blob, count}
	id, _, found, err := s.row(ctx, annCentroidsName, annCentroidsLookup, 2, []any{s.x.id, int64(centroid.ID)})
	if err != nil {
		return err
	}
	if found {
		return s.tx.replaceExtensionRow(ctx, s.catalog, annCentroidsName, id, values)
	}
	_, err = s.tx.insertExtensionRow(ctx, s.catalog, annCentroidsName, values)
	return err
}
func (s *annStore) centroidCount(ctx context.Context, listID int) (int64, error) {
	_, row, exists, err := s.row(ctx, annCentroidsName, annCentroidsLookup, 2, []any{s.x.id, int64(listID)})
	if err != nil {
		return 0, err
	}
	_, count, err := s.decodeCentroid(row, exists, listID)
	return count, err
}
func (s *annStore) changeCentroidCount(ctx context.Context, listID int, delta int64) error {
	id, row, exists, err := s.row(ctx, annCentroidsName, annCentroidsLookup, 2, []any{s.x.id, int64(listID)})
	if err != nil {
		return err
	}
	_, count, err := s.decodeCentroid(row, exists, listID)
	if err != nil {
		return err
	}
	count += delta
	if count < 0 || count > s.x.ann.MaxNodes {
		return extensionCorrupt("IVF member count overflow/underflow")
	}
	row[4] = count
	return s.tx.replaceExtensionRow(ctx, s.catalog, annCentroidsName, id, row)
}
func (s *annStore) MemberList(ctx context.Context, rowID int64) (int, bool, error) {
	key, found, err := extensionRangeOne(ctx, s.reader, s.x.catalog.roots[annMembersReverse], 2, []any{s.x.id, rowID})
	if err != nil || !found {
		return 0, found, err
	}
	if len(key) != 3 {
		return 0, false, extensionCorrupt("IVF reverse member shape")
	}
	physicalID, ok := key[2].(int64)
	if !ok || physicalID <= 0 {
		return 0, false, extensionCorrupt("IVF reverse physical identity")
	}
	row, exists, err := btree.SeekTable(ctx, s.reader, s.x.catalog.roots[annMembersName], physicalID)
	if err != nil {
		return 0, false, err
	}
	if !exists || len(row) != 4 || row[0] != nil || row[1] != s.x.id || row[3] != rowID {
		return 0, false, extensionCorrupt("IVF reverse member table mismatch")
	}
	listID, ok := row[2].(int64)
	if !ok || listID < 1 || listID > int64(s.x.ann.Lists) {
		return 0, false, extensionCorrupt("IVF member list identity")
	}
	return int(listID), true, nil
}
func (s *annStore) addMember(ctx context.Context, listID int, rowID int64) error {
	if listID < 1 || listID > s.x.ann.Lists {
		return extensionCorrupt("IVF assignment outside lists")
	}
	_, exists, err := s.MemberList(ctx, rowID)
	if err != nil {
		return err
	}
	if exists {
		return extensionCorrupt("duplicate IVF source membership")
	}
	_, err = s.tx.insertExtensionRow(ctx, s.catalog, annMembersName, []any{nil, s.x.id, int64(listID), rowID})
	if err != nil {
		return err
	}
	return s.changeCentroidCount(ctx, listID, 1)
}
func (s *annStore) removeMember(ctx context.Context, listID int, rowID int64) error {
	id, _, found, err := s.row(ctx, annMembersName, annMembersLookup, 3, []any{s.x.id, int64(listID), rowID})
	if err != nil {
		return err
	}
	if !found {
		return extensionCorrupt("missing IVF membership")
	}
	if err = s.tx.deleteExtensionRow(ctx, s.catalog, annMembersName, id); err != nil {
		return err
	}
	return s.changeCentroidCount(ctx, listID, -1)
}

// Members opens exactly one covering list range. It yields signed source IDs
// directly from that range; callers perform snapshot-bound reverse/base recheck.
func (s *annStore) Members(ctx context.Context, listID int) (*btree.IndexRangeCursor, error) {
	if listID < 1 || listID > s.x.ann.Lists {
		return nil, extensionCorrupt("IVF probe outside lists")
	}
	prefix := []any{s.x.id, int64(listID)}
	return btree.NewIndexRangeCursor(ctx, s.reader, s.x.catalog.roots[annMembersLookup], btree.KeyInfo{Columns: 3}, prefix, prefix)
}

// CheckMember reconciles the covering key against both physical and reverse
// identities. Missing/dangling records are corruption, never absent candidates.
func (s *annStore) CheckMember(ctx context.Context, listID int, rowID, physicalID int64) error {
	if physicalID <= 0 {
		return extensionCorrupt("IVF member physical identity")
	}
	row, found, err := btree.SeekTable(ctx, s.reader, s.x.catalog.roots[annMembersName], physicalID)
	if err != nil {
		return err
	}
	if !found || len(row) != 4 || row[0] != nil || row[1] != s.x.id || row[2] != int64(listID) || row[3] != rowID {
		return extensionCorrupt("IVF covering member table mismatch")
	}
	key, found, err := extensionRangeOne(ctx, s.reader, s.x.catalog.roots[annMembersReverse], 2, []any{s.x.id, rowID})
	if err != nil {
		return err
	}
	if !found || len(key) != 3 || key[2] != physicalID {
		return extensionCorrupt("IVF reverse member identity differs")
	}
	return nil
}
