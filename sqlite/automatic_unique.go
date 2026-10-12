package sqlite

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// automaticUniqueIndexes derives only the admitted rowid/BINARY ASC UNIQUE
// profile. Every declaration is validated before deduplication. INTEGER PRIMARY
// KEY itself consumes no suffix; a separate UNIQUE on that column does.
func automaticUniqueIndexes(def *sqlast.CreateTable, tab *writeTable) ([]writeIndex, error) {
	if len(def.UniqueConstraints) > sqlast.MaxUniqueConstraints {
		return nil, fmt.Errorf("%w: UNIQUE constraint count budget", ErrUnsupported)
	}
	var indexes []writeIndex
	seen := map[string]bool{}
	terms, bytes := 0, 0
	for _, constraint := range def.UniqueConstraints {
		if len(constraint.Columns) == 0 || len(constraint.Columns) > sqlast.MaxColumns || len(constraint.Columns) > sqlast.MaxUniqueTerms-terms {
			return nil, fmt.Errorf("%w: UNIQUE key-term budget", ErrUnsupported)
		}
		terms += len(constraint.Columns)
		idx, err := writableIndex(&sqlast.CreateIndex{Unique: true, Columns: constraint.Columns}, tab, 0)
		if err != nil {
			return nil, err
		}
		var key strings.Builder
		for _, column := range idx.columns {
			key.WriteString(strconv.Itoa(column))
			key.WriteByte('/')
		}
		if seen[key.String()] {
			continue
		}
		seen[key.String()] = true
		idx.name = "sqlite_autoindex_" + tab.name + "_" + strconv.Itoa(len(indexes)+1)
		idx.constraintOwned = true
		charge := 256 + len(idx.name) + 8*len(idx.columns)
		if charge > btree.MaxSchemaBytes-bytes {
			return nil, ErrMemoryLimit
		}
		bytes += charge
		indexes = append(indexes, idx)
	}
	return indexes, nil
}

// implicitIndexBinding is statement-local authority derived from table SQL,
// never a prefix exemption or a fabricated CREATE INDEX declaration.
type implicitIndexBinding struct {
	table   *writeTable
	ordinal int
}

func bindAutomaticUnique(entry btree.SchemaEntry, binding implicitIndexBinding) error {
	idx := &binding.table.indexes[binding.ordinal]
	if entry.Type != "index" || entry.Name != idx.name || entry.TableName != binding.table.name || entry.RootPage < 2 || idx.root != 0 {
		return fmt.Errorf("%w: implicit UNIQUE ownership mismatch", ErrCorrupt)
	}
	if !entry.SQLNull {
		if entry.SQL == "" {
			// SQLite's loader also accepts empty TEXT. This narrow profile emits
			// and admits canonical NULL only; empty TEXT is not called corrupt.
			return fmt.Errorf("%w: noncanonical empty-TEXT implicit index SQL", ErrUnsupported)
		}
		return fmt.Errorf("%w: implicit UNIQUE index has explicit SQL", ErrCorrupt)
	}
	if entry.SQL != "" {
		return fmt.Errorf("%w: inconsistent nullable schema SQL", ErrCorrupt)
	}
	idx.root = entry.RootPage
	return nil
}

// automaticTableSchemaRows describes precisely the rows a compound CREATE will
// persist. Admission counts deduplicated physical indexes; parser/descriptor
// budgets independently retain and validate every declared constraint.
func automaticTableSchemaRows(def *sqlast.CreateTable, tab *writeTable) []btree.SchemaEntry {
	rows := make([]btree.SchemaEntry, 0, len(tab.indexes)+1)
	rows = append(rows, btree.SchemaEntry{Type: "table", Name: def.Name, TableName: def.Name, SQL: def.SQL})
	for _, idx := range tab.indexes {
		rows = append(rows, btree.SchemaEntry{Type: "index", Name: idx.name, TableName: def.Name, SQLNull: true})
	}
	return rows
}

// admitSchemaEntries is a pure full-catalog model. Include all raw schema rows,
// even extension guard triggers omitted from writeCatalog.names.
func admitSchemaEntries(ctx context.Context, existing, additions []btree.SchemaEntry) error {
	usage := btree.SchemaUsage{}
	for _, rows := range [][]btree.SchemaEntry{existing, additions} {
		for _, entry := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := usage.Add(entry); err != nil {
				return err
			}
		}
	}
	return nil
}

// admitSchemaRows is always based on this transaction's current reader. It is
// deliberately not a cache: rollback, rollback-to, multiple CREATE statements
// and retries observe their actual schema rows without counter repair logic.
func (t *Tx) admitSchemaRows(ctx context.Context, additions []btree.SchemaEntry) error {
	entries, err := btree.ReadSchema(ctx, t.reader)
	if err != nil {
		return err
	}
	return admitSchemaEntries(ctx, entries, additions)
}
