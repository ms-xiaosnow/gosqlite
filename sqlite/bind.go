package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func bind(stmt sqlast.Statement, args []any) ([]any, error) {
	count := stmt.ParameterCount()
	if count > 32766 {
		return nil, fmt.Errorf("too many parameter slots")
	}
	params := make([]any, count)
	bound := make([]bool, count)
	used := stmt.Parameters()
	for ordinal, arg := range args {
		name := ""
		value := arg
		if named, ok := arg.(sql.NamedArg); ok {
			name = named.Name
			value = named.Value
		}
		v, err := parameterValue(value)
		if err != nil {
			return nil, fmt.Errorf("parameter %d: %w", ordinal+1, err)
		}
		if name == "" {
			if ordinal >= count {
				return nil, fmt.Errorf("too many parameters: statement has %d slots", count)
			}
			if bound[ordinal] {
				return nil, fmt.Errorf("parameter slot %d bound twice", ordinal+1)
			}
			params[ordinal] = v
			bound[ordinal] = true
			continue
		}
		found := false
		for _, p := range used {
			if p.Name != "" && p.Name[0] != '?' && p.Name[1:] == name {
				if bound[p.Index-1] {
					return nil, fmt.Errorf("parameter slot %d bound twice", p.Index)
				}
				params[p.Index-1] = v
				bound[p.Index-1] = true
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown named parameter %q", name)
		}
	}
	// Unused gaps in ?NNN numbering may be omitted when named binding supplies
	// every used slot. Ordinary positional binding still follows SQLite slots.
	for _, p := range used {
		if !bound[p.Index-1] {
			return nil, fmt.Errorf("missing parameter at slot %d", p.Index)
		}
	}
	return params, nil
}

func parameterValue(v any) (any, error) {
	value, err := driver.DefaultParameterConverter.ConvertValue(v)
	if err != nil {
		return nil, err
	}
	switch x := value.(type) {
	case nil, int64, string:
		return x, nil
	case float64:
		if math.IsNaN(x) {
			return nil, nil
		}
		return x, nil
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	case []byte:
		if x == nil {
			return nil, nil
		}
		return append([]byte{}, x...), nil
	default:
		return nil, fmt.Errorf("%w: parameter type %T", ErrUnsupported, value)
	}
}
