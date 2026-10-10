package btree

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// MaxSchemaBytes bounds retained schema strings plus a conservative allowance
// for each entry and its slice allocation. Decode's separate payload limit
// also bounds the temporary memory used to examine one oversized record.
const MaxSchemaBytes = 16 << 20
const schemaEntryOverhead = 256

// SchemaEntry is an on-disk sqlite_schema record. SQL is empty for the NULL
// SQL field used by SQLite's internal autoindexes.
type SchemaEntry struct {
	Type      string
	Name      string
	TableName string
	RootPage  uint32
	SQL       string
}

func ReadSchema(ctx context.Context, r PageReader) ([]SchemaEntry, error) {
	entries := make([]SchemaEntry, 0)
	schemaBytes := 0
	err := Scan(ctx, r, 1, func(_ int64, v []any) error {
		if len(entries) >= 32767 {
			return fmt.Errorf("%w: schema entry budget", record.ErrLimit)
		}
		if len(v) != 5 {
			return corrupt("sqlite_schema record has %d columns", len(v))
		}
		typ, ok1 := v[0].(string)
		name, ok2 := v[1].(string)
		table, ok3 := v[2].(string)
		root, ok4 := v[3].(int64)
		if !ok1 || !ok2 || !ok3 || !ok4 || name == "" || table == "" || root < 0 || root == 1 || root > int64(r.Header().PageCount) {
			return corrupt("invalid sqlite_schema fields")
		}
		switch typ {
		case "table", "index", "view", "trigger":
		default:
			return corrupt("unknown sqlite_schema type %q", typ)
		}
		sql := ""
		if v[4] != nil {
			var ok bool
			sql, ok = v[4].(string)
			if !ok {
				return corrupt("sqlite_schema SQL is not text")
			}
		}
		size := len(typ) + len(name) + len(table) + len(sql) + schemaEntryOverhead
		if size > MaxSchemaBytes-schemaBytes {
			return fmt.Errorf("%w: schema byte budget", record.ErrLimit)
		}
		schemaBytes += size
		entries = append(entries, SchemaEntry{Type: typ, Name: name, TableName: table, RootPage: uint32(root), SQL: sql})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
