package sql

import "context"

// ConnectionState is volatile SQL connection state, not transactional data.
// Its owner serializes every evaluation and mutation with the connection lock.
// It must not be copied into a statement: last_insert_rowid() can change between
// VALUES rows, and another completed statement can affect a streaming SELECT.
type ConnectionState struct {
	Changes, TotalChanges, LastInsertRowID int64
}

type connectionStateKey struct{}

// WithConnectionState adds one bounded context node per query/write, with no
// function registry, per-row closure or per-call allocation. The scalar
// evaluator never reacquires the connection lock held by its caller.
func WithConnectionState(ctx context.Context, state *ConnectionState) context.Context {
	return context.WithValue(ctx, connectionStateKey{}, state)
}

func isConnectionFunction(name string) bool {
	return name == "changes" || name == "total_changes" || name == "last_insert_rowid"
}

func evalConnectionFunction(name string, argc int, ctx context.Context) (any, error) {
	if err := ValidateFunction(name, argc); err != nil {
		return nil, err
	}
	if ctx != nil {
		if state, ok := ctx.Value(connectionStateKey{}).(*ConnectionState); ok && state != nil {
			switch name {
			case "changes":
				return state.Changes, nil
			case "total_changes":
				return state.TotalChanges, nil
			case "last_insert_rowid":
				return state.LastInsertRowID, nil
			}
		}
	}
	return nil, &ErrUnsupported{Feature: "connection function outside SQL statement context"}
}
