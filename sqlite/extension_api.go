package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// GramsIndexOptions selects the configured MATCH namespace. Its zero value uses
// DefaultMatchConfig. LIKE always uses the independent raw SQL-LIKE namespace.
type GramsIndexOptions struct{ MatchConfig grams13.Config }

func gramsConfig(options []GramsIndexOptions) ([]byte, error) {
	if len(options) > 1 {
		return nil, fmt.Errorf("at most one GramsIndexOptions is accepted")
	}
	config := grams13.DefaultMatchConfig()
	if len(options) == 1 && options[0].MatchConfig != (grams13.Config{}) {
		config = options[0].MatchConfig
	}
	return grams13.ConfigBytes(config)
}

// CreateGramsIndex builds and publishes both persistent namespaces in one
// cancellable statement sharing the source transaction and dirty-page budget.
func (d *DB) CreateGramsIndex(ctx context.Context, name, table string, columns []string, options ...GramsIndexOptions) error {
	config, err := gramsConfig(options)
	if err != nil {
		return wrap("create grams index", err)
	}
	return d.CreateMethodIndex(ctx, name, table, columns, grams13.MethodName, grams13.MethodVersion, config)
}
func (t *Tx) CreateGramsIndex(ctx context.Context, name, table string, columns []string, options ...GramsIndexOptions) error {
	config, err := gramsConfig(options)
	if err != nil {
		return wrap("create grams index", err)
	}
	return t.CreateMethodIndex(ctx, name, table, columns, grams13.MethodName, grams13.MethodVersion, config)
}

// CreateMethodIndex builds a registered inverted method or a known built-in ANN
// method. Caller-supplied canonical configuration is validated; persistent
// metadata never loads code. Registry handlers are trusted Go application code.
func (d *DB) CreateMethodIndex(ctx context.Context, name, table string, columns []string, method string, version int, canonical []byte) error {
	return d.extensionWrite(ctx, "create index", func(t *Tx, c *writeCatalog) error {
		return t.createMethodIndex(ctx, c, name, table, columns, method, version, canonical, false)
	})
}
func (t *Tx) CreateMethodIndex(ctx context.Context, name, table string, columns []string, method string, version int, canonical []byte) error {
	if err := checkContext(ctx); err != nil {
		return wrap("create index", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return wrap("create index", t.extensionWriteLocked(ctx, func(c *writeCatalog) error {
		return t.createMethodIndex(ctx, c, name, table, columns, method, version, canonical, false)
	}))
}

// Reindex rebuilds a persistent extension index. Supplying GramsIndexOptions
// atomically changes grams13 configuration; failed rebuild preserves old data.
func (d *DB) Reindex(ctx context.Context, name string, options ...GramsIndexOptions) error {
	return d.extensionWrite(ctx, "reindex", func(t *Tx, c *writeCatalog) error { return t.reconfigureAndReindex(ctx, c, name, options) })
}
func (t *Tx) Reindex(ctx context.Context, name string, options ...GramsIndexOptions) error {
	if err := checkContext(ctx); err != nil {
		return wrap("reindex", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return wrap("reindex", t.extensionWriteLocked(ctx, func(c *writeCatalog) error { return t.reconfigureAndReindex(ctx, c, name, options) }))
}
func (t *Tx) reconfigureAndReindex(ctx context.Context, c *writeCatalog, name string, options []GramsIndexOptions) error {
	if len(options) > 0 {
		config, err := gramsConfig(options)
		if err != nil {
			return err
		}
		for _, x := range c.extensions.indexes {
			if fold(x.name) == fold(name) {
				if x.method != grams13.MethodName {
					return fmt.Errorf("%w: GramsIndexOptions on another method", ErrUnsupported)
				}
				method, ok := t.db.methods.Lookup(x.method, x.methodVersion)
				if !ok {
					return ErrUnsupported
				}
				instance, err := method.Configure(config)
				if err != nil {
					return err
				}
				x.instance = instance
				x.canonical = config
				break
			}
		}
	}
	return t.reindexExtension(ctx, c, name)
}
func (d *DB) extensionWrite(ctx context.Context, op string, action func(*Tx, *writeCatalog) error) error {
	if err := checkContext(ctx); err != nil {
		return wrap(op, err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return wrap(op, ErrClosed)
	}
	if d.tx != nil {
		return wrap(op, d.tx.extensionWriteLocked(ctx, func(c *writeCatalog) error { return action(d.tx, c) }))
	}
	if d.opts.ReadOnly {
		return wrap(op, ErrReadOnly)
	}
	tx, err := d.beginLocked(ctx, TxOptions{})
	if err != nil {
		return wrap(op, err)
	}
	err = tx.extensionWriteLocked(ctx, func(c *writeCatalog) error { return action(tx, c) })
	if err == nil {
		err = tx.commitLocked(ctx)
	}
	if err != nil {
		err = errors.Join(err, tx.rollbackLocked())
	}
	return wrap(op, err)
}
func (t *Tx) extensionWriteLocked(ctx context.Context, action func(*writeCatalog) error) error {
	if err := t.check(); err != nil {
		return err
	}
	if t.readOnly {
		return ErrReadOnly
	}
	if t.activeRows != 0 {
		return fmt.Errorf("%w: close transaction rows before executing a write", ErrBusy)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return err
	}
	return t.storage.Statement(ctx, func() error {
		c, err := t.validateCatalog(ctx)
		if err != nil {
			return err
		}
		if err = t.checkWriteOwnership(ctx, c); err != nil {
			return err
		}
		if err = t.storage.PromoteWrite(ctx); err != nil {
			return err
		}
		before := t.reader.Header().SchemaCookie
		if err = action(c); err != nil {
			return err
		}
		return t.refreshExtensionSchemaCookies(ctx, c, before)
	})
}
func (t *Tx) checkWriteOwnership(ctx context.Context, c *writeCatalog) error {
	if t.ownershipValidated {
		return nil
	}
	var roots []btree.RootSpec
	for _, tab := range c.tables {
		roots = append(roots, btree.RootSpec{Page: tab.root, Kind: btree.TableTree})
		for _, idx := range tab.indexes {
			roots = append(roots, btree.RootSpec{Page: idx.root, Kind: btree.IndexTree, KeyInfo: btree.KeyInfo{Columns: len(idx.columns)}})
		}
	}
	if err := btree.CheckOwnership(ctx, t.reader, roots, t.storage.FreePages()); err != nil {
		return err
	}
	t.ownershipValidated = true
	return nil
}
func (t *Tx) createSQLMethodIndex(ctx context.Context, c *writeCatalog, s *sqlast.CreateIndex) error {
	if s.Unique {
		return fmt.Errorf("%w: UNIQUE extension indexes", ErrUnsupported)
	}
	if fold(s.Method) == "ivfflat" {
		return t.createSQLIVFIndex(ctx, c, s)
	}
	if fold(s.Method) == "hnsw" {
		return t.createSQLHNSWIndex(ctx, c, s)
	}
	if fold(s.Method) != grams13.MethodName {
		return fmt.Errorf("%w: SQL index method %q; use native CreateMethodIndex for registered custom methods", ErrUnsupported, s.Method)
	}
	for _, desc := range s.Desc {
		if desc {
			return fmt.Errorf("%w: descending grams fields", ErrUnsupported)
		}
	}
	config := grams13.DefaultMatchConfig()
	for key, value := range s.Options {
		switch key {
		case "min_gram":
			if value != int64(1) {
				return fmt.Errorf("%w: grams13 min_gram must be 1", ErrUnsupported)
			}
		case "max_gram":
			if value != int64(3) {
				return fmt.Errorf("%w: grams13 max_gram must be 3", ErrUnsupported)
			}
		case "case_fold":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("case_fold must be text")
			}
			config.CaseFold = v
		case "whitespace":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("whitespace must be text")
			}
			config.Whitespace = v
		case "punctuation":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("punctuation must be text")
			}
			config.Punctuation = v
		default:
			return fmt.Errorf("%w: grams13 option %q", ErrUnsupported, key)
		}
	}
	canonical, err := grams13.ConfigBytes(config)
	if err != nil {
		return err
	}
	return t.createMethodIndex(ctx, c, s.Name, s.Table, s.Columns, grams13.MethodName, grams13.MethodVersion, canonical, s.IfNotExists)
}
