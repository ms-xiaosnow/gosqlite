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
const MaxSchemaEntries = 32767
const schemaEntryOverhead = 256

// SchemaEntry is an on-disk sqlite_schema record. SQLNull preserves SQL NULL
// independently of empty TEXT; SQL stays empty when SQLNull is true.
type SchemaEntry struct {
	Type      string
	Name      string
	TableName string
	RootPage  uint32
	SQL       string
	SQLNull   bool
}

// SchemaUsage is the exact retained-record budget used by ReadSchema. SQL NULL
// has an empty SQL string but still charges its type, name and owning table.
// RootPage and SQLNull do not add variable bytes to the existing 256-byte charge.
// Callers must derive this value from the current complete schema, never names
// filtered for a planner/catalog cache. A zero value represents an empty schema.
type SchemaUsage struct {
	Entries int
	Bytes   int
}

// Add leaves the receiver unchanged on failure. Bounded subtraction avoids
// overflowing a sum of string lengths or accepting a malformed usage counter.
func (u *SchemaUsage) Add(entry SchemaEntry) error {
	if u.Entries < 0 || u.Entries >= MaxSchemaEntries {
		return fmt.Errorf("%w: schema entry budget", record.ErrLimit)
	}
	if u.Bytes < 0 || u.Bytes > MaxSchemaBytes {
		return fmt.Errorf("%w: schema byte budget", record.ErrLimit)
	}
	remaining := MaxSchemaBytes - u.Bytes
	for _, size := range []int{schemaEntryOverhead, len(entry.Type), len(entry.Name), len(entry.TableName), len(entry.SQL)} {
		if size > remaining {
			return fmt.Errorf("%w: schema byte budget", record.ErrLimit)
		}
		remaining -= size
	}
	u.Entries++
	u.Bytes = MaxSchemaBytes - remaining
	return nil
}

func ReadSchema(ctx context.Context, r PageReader) ([]SchemaEntry, error) {
	entries := make([]SchemaEntry, 0)
	usage := SchemaUsage{}
	err := Scan(ctx, r, 1, func(_ int64, v []any) error {
		if len(entries) >= MaxSchemaEntries {
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
		entry := SchemaEntry{Type: typ, Name: name, TableName: table, RootPage: uint32(root), SQL: sql, SQLNull: v[4] == nil}
		if err := usage.Add(entry); err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
