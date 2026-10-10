package sqlite

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// HNSWIndexOptions selects persisted construction parameters. Zero fields use
// fixed defaults. Dimensions and representation come from the declared typed
// column. Numeric kinds default to L2; bit defaults to Hamming. Opclass
// must match the representation/metric.
// Cosine indexes impose a zero-norm constraint on every indexed non-NULL value.
// Limits apply to construction and each algorithm operation; ordinary pager
// dirty-page and connection memory limits can reject a statement earlier.
type HNSWIndexOptions struct {
	Metric                             vector.MetricKind
	Opclass                            string
	M, EFConstruction                  int
	Seed                               uint64
	MaxNodes, MaxTombstones            int64
	MaxVisited, MaxDistanceEvaluations int
	MaxMemoryBytes                     int64
}

func (d *DB) CreateHNSWIndex(ctx context.Context, name, table, column string, options ...HNSWIndexOptions) error {
	return d.extensionWrite(ctx, "create HNSW index", func(t *Tx, c *writeCatalog) error {
		return t.createHNSWIndex(ctx, c, name, table, column, options, false)
	})
}
func (t *Tx) CreateHNSWIndex(ctx context.Context, name, table, column string, options ...HNSWIndexOptions) error {
	if err := checkContext(ctx); err != nil {
		return wrap("create HNSW index", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return wrap("create HNSW index", t.extensionWriteLocked(ctx, func(c *writeCatalog) error { return t.createHNSWIndex(ctx, c, name, table, column, options, false) }))
}
func (t *Tx) createHNSWIndex(ctx context.Context, c *writeCatalog, name, table, column string, options []HNSWIndexOptions, ifNotExists bool) error {
	if len(options) > 1 {
		return fmt.Errorf("at most one HNSWIndexOptions is accepted")
	}
	var o HNSWIndexOptions
	if len(options) == 1 {
		o = options[0]
	}
	tab := c.tables[fold(table)]
	if tab == nil {
		return fmt.Errorf("no such table: %s", table)
	}
	ordinal, ok := tab.index[fold(column)]
	if !ok {
		return fmt.Errorf("no such column: %s", column)
	}
	kind, dims, ok := annColumnType(tab.columns[ordinal].Type)
	if !ok {
		return fmt.Errorf("%w: ANN requires a declared vector column", ErrMismatch)
	}
	metric, err := annCreationMetric(kind, o.Metric, o.Opclass)
	if err != nil {
		return err
	}
	cfg, err := (ann.TypedConfig{Kind: kind, Config: ann.Config{Method: ann.HNSW, Dimensions: dims, Metric: metric, Opclass: o.Opclass, M: o.M, EFConstruction: o.EFConstruction, Seed: o.Seed, MaxNodes: o.MaxNodes, MaxTombstones: o.MaxTombstones, MaxVisited: o.MaxVisited, MaxDistanceEvaluations: o.MaxDistanceEvaluations, MaxMemoryBytes: o.MaxMemoryBytes}}).Normalize()
	if err != nil {
		return err
	}
	canonical, err := cfg.Canonical()
	if err != nil {
		return err
	}
	return t.createMethodIndex(ctx, c, name, table, []string{column}, ann.HNSW, cfg.Version, canonical, ifNotExists)
}
func (t *Tx) createSQLHNSWIndex(ctx context.Context, c *writeCatalog, s *sqlast.CreateIndex) error {
	if len(s.Columns) != 1 {
		return fmt.Errorf("%w: HNSW requires one vector field", ErrUnsupported)
	}
	for _, desc := range s.Desc {
		if desc {
			return fmt.Errorf("%w: descending HNSW field", ErrUnsupported)
		}
	}
	o := HNSWIndexOptions{}
	for key, value := range s.Options {
		integer := func() (int64, error) {
			v, ok := value.(int64)
			if !ok {
				return 0, fmt.Errorf("HNSW %s must be integer", key)
			}
			return v, nil
		}
		switch key {
		case "metric":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("HNSW metric must be text")
			}
			o.Metric = vector.MetricKind(v)
		case "opclass":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("HNSW opclass must be text")
			}
			o.Opclass = v
		case "m", "ef_construction", "max_nodes", "max_tombstones", "max_visited", "max_distance_evaluations", "max_memory_bytes", "seed":
			v, err := integer()
			if err != nil {
				return err
			}
			if v < 0 {
				return fmt.Errorf("HNSW %s must not be negative", key)
			}
			if v == 0 && key != "seed" {
				return fmt.Errorf("HNSW %s must be positive when specified", key)
			}
			if int64(int(v)) != v && (key == "m" || key == "ef_construction" || key == "max_visited" || key == "max_distance_evaluations") {
				return ann.ErrConfig
			}
			switch key {
			case "m":
				o.M = int(v)
			case "ef_construction":
				o.EFConstruction = int(v)
			case "max_nodes":
				o.MaxNodes = v
			case "max_tombstones":
				o.MaxTombstones = v
			case "max_visited":
				o.MaxVisited = int(v)
			case "max_distance_evaluations":
				o.MaxDistanceEvaluations = int(v)
			case "max_memory_bytes":
				o.MaxMemoryBytes = v
			case "seed":
				o.Seed = uint64(v)
			}
		default:
			return fmt.Errorf("%w: HNSW option %q", ErrUnsupported, key)
		}
	}
	// A specified operator class selects its built-in metric unless an explicit
	// metric is also present, in which case canonical validation requires equality.
	if o.Metric == "" && o.Opclass != "" {
		switch o.Opclass {
		case "vector_l2_ops", "halfvec_l2_ops", "sparsevec_l2_ops":
			o.Metric = vector.L2
		case "vector_l1_ops", "halfvec_l1_ops", "sparsevec_l1_ops":
			o.Metric = vector.L1
		case "vector_ip_ops", "halfvec_ip_ops", "sparsevec_ip_ops":
			o.Metric = vector.NegativeIP
		case "vector_cosine_ops", "halfvec_cosine_ops", "sparsevec_cosine_ops":
			o.Metric = vector.Cosine
		}
	}
	return t.createHNSWIndex(ctx, c, s.Name, s.Table, s.Columns[0], []HNSWIndexOptions{o}, s.IfNotExists)
}
