package sqlite

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// IVFFlatIndexOptions describes immutable deterministic training policy. Zero
// fields inherit fixed defaults. L2 uses ordinary k-means; cosine/IP use
// spherical centroids while exact results always score original source vectors.
// Insufficient trainable samples persist an untrained index requiring REINDEX.
// L1 is explicitly unsupported by this pgvector-compatible method family.
type IVFFlatIndexOptions struct {
	Metric                                     vector.MetricKind
	Opclass                                    string
	Lists, TrainingSamples, TrainingIterations int
	Seed                                       uint64
	MaxNodes                                   int64
	MaxTrainingWork                            int64
	MaxVisited, MaxDistanceEvaluations         int
	MaxMemoryBytes                             int64
}

func (d *DB) CreateIVFFlatIndex(ctx context.Context, name, table, column string, options ...IVFFlatIndexOptions) error {
	return d.extensionWrite(ctx, "create IVFFlat index", func(t *Tx, c *writeCatalog) error {
		return t.createIVFIndex(ctx, c, name, table, column, options, false)
	})
}
func (t *Tx) CreateIVFFlatIndex(ctx context.Context, name, table, column string, options ...IVFFlatIndexOptions) error {
	if err := checkContext(ctx); err != nil {
		return wrap("create IVFFlat index", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return wrap("create IVFFlat index", t.extensionWriteLocked(ctx, func(c *writeCatalog) error { return t.createIVFIndex(ctx, c, name, table, column, options, false) }))
}
func (t *Tx) createIVFIndex(ctx context.Context, c *writeCatalog, name, table, column string, options []IVFFlatIndexOptions, ifNotExists bool) error {
	if len(options) > 1 {
		return fmt.Errorf("at most one IVFFlatIndexOptions is accepted")
	}
	var o IVFFlatIndexOptions
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
	dims, ok := sqlast.VectorDimensions(tab.columns[ordinal].Type)
	if !ok {
		return fmt.Errorf("%w: IVF requires a VECTOR(n) column", ErrMismatch)
	}
	metric, err := vectorMetric(o.Metric)
	if err != nil {
		return err
	}
	if metric == vector.L1 {
		return fmt.Errorf("%w: IVFFlat does not support L1", ErrUnsupported)
	}
	cfg, err := (ann.Config{Method: ann.IVFFlat, Dimensions: dims, Metric: metric, Opclass: o.Opclass, Lists: o.Lists, TrainingSamples: o.TrainingSamples, TrainingIterations: o.TrainingIterations, Seed: o.Seed, MaxNodes: o.MaxNodes, MaxTrainingWork: o.MaxTrainingWork, MaxVisited: o.MaxVisited, MaxDistanceEvaluations: o.MaxDistanceEvaluations, MaxMemoryBytes: o.MaxMemoryBytes}).Normalize()
	if err != nil {
		return err
	}
	canonical, err := cfg.Canonical()
	if err != nil {
		return err
	}
	return t.createMethodIndex(ctx, c, name, table, []string{column}, ann.IVFFlat, ann.MethodVersion, canonical, ifNotExists)
}
func (t *Tx) createSQLIVFIndex(ctx context.Context, c *writeCatalog, s *sqlast.CreateIndex) error {
	if len(s.Columns) != 1 {
		return fmt.Errorf("%w: IVFFlat requires one vector field", ErrUnsupported)
	}
	for _, desc := range s.Desc {
		if desc {
			return fmt.Errorf("%w: descending IVFFlat field", ErrUnsupported)
		}
	}
	o := IVFFlatIndexOptions{}
	for key, value := range s.Options {
		switch key {
		case "metric":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("IVFFlat metric must be text")
			}
			o.Metric = vector.MetricKind(v)
		case "opclass":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("IVFFlat opclass must be text")
			}
			o.Opclass = v
		case "lists", "training_samples", "training_iterations", "max_nodes", "max_training_work", "max_visited", "max_distance_evaluations", "max_memory_bytes", "seed":
			v, ok := value.(int64)
			if !ok || v < 0 || (v == 0 && key != "seed") {
				return fmt.Errorf("IVFFlat %s must be a positive integer", key)
			}
			if (key == "lists" || key == "training_samples" || key == "training_iterations" || key == "max_visited" || key == "max_distance_evaluations") && int64(int(v)) != v {
				return ann.ErrConfig
			}
			switch key {
			case "lists":
				o.Lists = int(v)
			case "training_samples":
				o.TrainingSamples = int(v)
			case "training_iterations":
				o.TrainingIterations = int(v)
			case "max_nodes":
				o.MaxNodes = v
			case "max_training_work":
				o.MaxTrainingWork = v
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
			return fmt.Errorf("%w: IVFFlat option %q", ErrUnsupported, key)
		}
	}
	if o.Metric == "" && o.Opclass != "" {
		switch o.Opclass {
		case "vector_l2_ops":
			o.Metric = vector.L2
		case "vector_ip_ops":
			o.Metric = vector.NegativeIP
		case "vector_cosine_ops":
			o.Metric = vector.Cosine
		case "vector_l1_ops":
			return fmt.Errorf("%w: IVFFlat L1 is not supported", ErrUnsupported)
		}
	}
	return t.createIVFIndex(ctx, c, s.Name, s.Table, s.Columns[0], []IVFFlatIndexOptions{o}, s.IfNotExists)
}
