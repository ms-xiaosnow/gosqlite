package sqlite

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

const (
	annStateName       = extensionPrefix + "ann_state"
	annNodesName       = extensionPrefix + "ann_nodes"
	annLiveName        = extensionPrefix + "ann_live"
	annCentroidsName   = extensionPrefix + "ann_centroids"
	annMembersName     = extensionPrefix + "ann_members"
	annStateLookup     = extensionPrefix + "ann_state_lookup"
	annNodesLookup     = extensionPrefix + "ann_nodes_lookup"
	annLiveLookup      = extensionPrefix + "ann_live_lookup"
	annCentroidsLookup = extensionPrefix + "ann_centroids_lookup"
	annMembersLookup   = extensionPrefix + "ann_members_lookup"
	annMembersReverse  = extensionPrefix + "ann_members_reverse"
)

// The ANN v1 family is optional as a whole. The existing seven M4 schemas are
// unchanged; older grams-only files therefore remain readable without migration.
// All these are ordinary SQLite tables/indexes sharing the source transaction.
func annSchemas() []extensionSchemaSpec {
	return []extensionSchemaSpec{
		{"table", annStateName, annStateName, "CREATE TABLE " + annStateName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, entry_node INTEGER NOT NULL, max_level INTEGER NOT NULL, live_count INTEGER NOT NULL, tombstone_count INTEGER NOT NULL, next_node_id INTEGER NOT NULL, training_state TEXT NOT NULL)"},
		{"table", annNodesName, annNodesName, "CREATE TABLE " + annNodesName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, node_id INTEGER NOT NULL, content_rowid INTEGER NOT NULL, live INTEGER NOT NULL, level INTEGER NOT NULL, original_vector BLOB NOT NULL, adjacency BLOB NOT NULL)"},
		{"table", annLiveName, annLiveName, "CREATE TABLE " + annLiveName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, content_rowid INTEGER NOT NULL, node_id INTEGER NOT NULL)"},
		{"table", annCentroidsName, annCentroidsName, "CREATE TABLE " + annCentroidsName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, list_id INTEGER NOT NULL, centroid BLOB NOT NULL, member_count INTEGER NOT NULL)"},
		{"table", annMembersName, annMembersName, "CREATE TABLE " + annMembersName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, list_id INTEGER NOT NULL, content_rowid INTEGER NOT NULL)"},
		{"index", annStateLookup, annStateName, "CREATE INDEX " + annStateLookup + " ON " + annStateName + " (index_id)"},
		{"index", annNodesLookup, annNodesName, "CREATE INDEX " + annNodesLookup + " ON " + annNodesName + " (index_id, node_id)"},
		{"index", annLiveLookup, annLiveName, "CREATE INDEX " + annLiveLookup + " ON " + annLiveName + " (index_id, content_rowid)"},
		{"index", annCentroidsLookup, annCentroidsName, "CREATE INDEX " + annCentroidsLookup + " ON " + annCentroidsName + " (index_id, list_id)"},
		{"index", annMembersLookup, annMembersName, "CREATE INDEX " + annMembersLookup + " ON " + annMembersName + " (index_id, list_id, content_rowid)"},
		{"index", annMembersReverse, annMembersName, "CREATE INDEX " + annMembersReverse + " ON " + annMembersName + " (index_id, content_rowid)"},
	}
}

func (t *Tx) initializeANNStorage(ctx context.Context, c *writeCatalog) error {
	if c.extensions.roots[annStateName] != 0 {
		return nil
	}
	for _, s := range annSchemas() {
		kind := btree.TableTree
		if s.typ == "index" {
			kind = btree.IndexTree
		}
		root, err := btree.CreateTree(ctx, t.storage, kind)
		if err != nil {
			return err
		}
		if err = t.addSchema(ctx, s.typ, s.name, s.table, root, s.sql); err != nil {
			return err
		}
		c.extensions.roots[s.name] = root
		c.names[s.name] = s.typ
		parsed, err := sqlast.Parse(s.sql)
		if err != nil {
			return err
		}
		if s.typ == "table" {
			tab, err := writableTable(parsed.(*sqlast.CreateTable), root)
			if err != nil {
				return err
			}
			c.tables[s.name] = tab
		} else {
			tab := c.tables[s.table]
			idx, err := writableIndex(parsed.(*sqlast.CreateIndex), tab, root)
			if err != nil {
				return err
			}
			tab.indexes = append(tab.indexes, idx)
		}
	}
	return nil
}

// annStore resolves physical roots from this statement/read snapshot only. It
// never enumerates a graph for search/open and owns no mutable cross-query cache.
type annStore struct {
	reader  btree.PageReader
	x       *extensionIndex
	tx      *Tx
	catalog *writeCatalog
}

func newANNStore(reader btree.PageReader, x *extensionIndex) *annStore {
	return &annStore{reader: reader, x: x}
}
func newANNWriteStore(t *Tx, c *writeCatalog, x *extensionIndex) *annStore {
	return &annStore{reader: t.reader, x: x, tx: t, catalog: c}
}

func (s *annStore) row(ctx context.Context, table, index string, columns int, prefix []any) (int64, []any, bool, error) {
	key, ok, err := extensionRangeOne(ctx, s.reader, s.x.catalog.roots[index], columns, prefix)
	if err != nil || !ok {
		return 0, nil, ok, err
	}
	if len(key) != columns+1 {
		return 0, nil, false, extensionCorrupt("ANN lookup key shape")
	}
	id, ok := key[columns].(int64)
	if !ok || id <= 0 {
		return 0, nil, false, extensionCorrupt("ANN physical row id")
	}
	row, found, err := btree.SeekTable(ctx, s.reader, s.x.catalog.roots[table], id)
	if err != nil {
		return 0, nil, false, err
	}
	if !found || len(row) < len(prefix)+1 || row[0] != nil {
		return 0, nil, false, extensionCorrupt("ANN lookup missing table row")
	}
	for i, v := range prefix {
		if row[i+1] != v {
			return 0, nil, false, extensionCorrupt("ANN table/index mismatch")
		}
	}
	return id, row, true, nil
}
func (s *annStore) GetState(ctx context.Context) (ann.State, error) {
	_, v, ok, err := s.row(ctx, annStateName, annStateLookup, 1, []any{s.x.id})
	if err != nil {
		return ann.State{}, err
	}
	if !ok || len(v) != 8 {
		return ann.State{}, extensionCorrupt("ANN state shape")
	}
	e, eok := v[2].(int64)
	ml, mok := v[3].(int64)
	live, lok := v[4].(int64)
	dead, dok := v[5].(int64)
	next, nok := v[6].(int64)
	training, tok := v[7].(string)
	if !eok || !mok || !lok || !dok || !nok || !tok || ml < -1 || ml > ann.MaxLevel || live < 0 || dead < 0 || next < 1 || next > s.x.ann.MaxNodes+1 || live > s.x.ann.MaxNodes || dead > s.x.ann.MaxTombstones || live+dead != next-1 || e < 0 || e >= next || ((next == 1) != (e == 0)) || ((e == 0) != (ml == -1)) || training != "" {
		return ann.State{}, extensionCorrupt("ANN state values")
	}
	return ann.State{Entry: e, MaxLevel: int(ml), Live: live, Tombstones: dead, NextID: next, TrainingState: training}, nil
}
func (s *annStore) putState(ctx context.Context, state ann.State) error {
	values := []any{nil, s.x.id, state.Entry, int64(state.MaxLevel), state.Live, state.Tombstones, state.NextID, state.TrainingState}
	id, _, ok, err := s.row(ctx, annStateName, annStateLookup, 1, []any{s.x.id})
	if err != nil {
		return err
	}
	if ok {
		return s.tx.replaceExtensionRow(ctx, s.catalog, annStateName, id, values)
	}
	_, err = s.tx.insertExtensionRow(ctx, s.catalog, annStateName, values)
	return err
}
func (s *annStore) GetNode(ctx context.Context, nodeID int64) (ann.Node, error) {
	if nodeID < 1 || nodeID > s.x.ann.MaxNodes {
		return ann.Node{}, extensionCorrupt("ANN node identity outside limits")
	}
	_, v, ok, err := s.row(ctx, annNodesName, annNodesLookup, 2, []any{s.x.id, nodeID})
	if err != nil {
		return ann.Node{}, err
	}
	if !ok || len(v) != 8 {
		return ann.Node{}, extensionCorrupt("ANN missing node")
	}
	row, rok := v[3].(int64)
	live, lok := v[4].(int64)
	level, levok := v[5].(int64)
	encoded, vok := v[6].([]byte)
	adj, aok := v[7].([]byte)
	if !rok || !lok || !levok || !vok || !aok || (live != 0 && live != 1) || level < 0 || level > ann.MaxLevel {
		return ann.Node{}, extensionCorrupt("ANN node shape")
	}
	vec, err := vector.Decode(encoded)
	if err != nil {
		return ann.Node{}, fmt.Errorf("%w: ANN stored vector: %w", ErrCorrupt, err)
	}
	neighbors, err := ann.DecodeAdjacency(adj, nodeID, int(level), *s.x.ann)
	if err != nil {
		return ann.Node{}, fmt.Errorf("%w: ANN adjacency: %w", ErrCorrupt, err)
	}
	node := ann.Node{ID: nodeID, RowID: row, Live: live == 1, Level: int(level), Vector: vec, Neighbors: neighbors}
	if err = ann.ValidateNode(node, *s.x.ann); err != nil {
		return ann.Node{}, fmt.Errorf("%w: ANN node: %w", ErrCorrupt, err)
	}
	return node, nil
}
func (s *annStore) PutNode(ctx context.Context, node ann.Node) error {
	if s.tx == nil {
		return ErrReadOnly
	}
	if err := ann.ValidateNode(node, *s.x.ann); err != nil {
		return err
	}
	encoded, err := node.Vector.MarshalBinary()
	if err != nil {
		return err
	}
	adjacent, err := ann.EncodeAdjacency(node, *s.x.ann)
	if err != nil {
		return err
	}
	live := int64(0)
	if node.Live {
		live = 1
	}
	values := []any{nil, s.x.id, node.ID, node.RowID, live, int64(node.Level), encoded, adjacent}
	id, _, ok, err := s.row(ctx, annNodesName, annNodesLookup, 2, []any{s.x.id, node.ID})
	if err != nil {
		return err
	}
	if ok {
		return s.tx.replaceExtensionRow(ctx, s.catalog, annNodesName, id, values)
	}
	_, err = s.tx.insertExtensionRow(ctx, s.catalog, annNodesName, values)
	return err
}
func (s *annStore) LiveNode(ctx context.Context, rowID int64) (int64, bool, error) {
	_, v, ok, err := s.row(ctx, annLiveName, annLiveLookup, 2, []any{s.x.id, rowID})
	if err != nil || !ok {
		return 0, ok, err
	}
	if len(v) != 4 {
		return 0, false, extensionCorrupt("ANN live mapping shape")
	}
	node, nok := v[3].(int64)
	if !nok || node < 1 || node > s.x.ann.MaxNodes {
		return 0, false, extensionCorrupt("ANN live mapping identity")
	}
	return node, true, nil
}
func (s *annStore) addLive(ctx context.Context, rowID, nodeID int64) error {
	_, exists, err := s.LiveNode(ctx, rowID)
	if err != nil {
		return err
	}
	if exists {
		return extensionCorrupt("duplicate ANN live source identity")
	}
	_, err = s.tx.insertExtensionRow(ctx, s.catalog, annLiveName, []any{nil, s.x.id, rowID, nodeID})
	return err
}
func (s *annStore) removeLive(ctx context.Context, rowID int64) error {
	id, _, ok, err := s.row(ctx, annLiveName, annLiveLookup, 2, []any{s.x.id, rowID})
	if err != nil {
		return err
	}
	if !ok {
		return extensionCorrupt("missing ANN live source identity")
	}
	return s.tx.deleteExtensionRow(ctx, s.catalog, annLiveName, id)
}
