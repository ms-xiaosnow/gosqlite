package main

import (
	"fmt"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type readOnlyWALFlags struct {
	walExclusive, walShared, walReadOnly, extensions bool
}

// Parse all leading flags so duplicate/conflicting modes cannot become paths.
func parseReadOnlyWALFlags(args []string, inspect bool) (readOnlyWALFlags, []string, error) {
	var flags readOnlyWALFlags
	modeSeen := false
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "--wal-exclusive", "--wal-shared", "--wal-readonly":
			if modeSeen {
				return flags, nil, fmt.Errorf("duplicate or conflicting WAL modes")
			}
			modeSeen = true
			flags.walExclusive = args[0] == "--wal-exclusive"
			flags.walShared = args[0] == "--wal-shared"
			flags.walReadOnly = args[0] == "--wal-readonly"
		case "--extensions":
			if !inspect || flags.extensions {
				return flags, nil, fmt.Errorf("unexpected or duplicate --extensions")
			}
			flags.extensions = true
		default:
			return flags, nil, fmt.Errorf("unknown flag %q", args[0])
		}
		args = args[1:]
	}
	return flags, args, nil
}
func (flags readOnlyWALFlags) queryOptions(selectOnly bool) (sqlite.Options, error) {
	if flags.walReadOnly && !selectOnly {
		return sqlite.Options{}, fmt.Errorf("%w: --wal-readonly requires SELECT", sqlite.ErrReadOnly)
	}
	return sqlite.Options{
		ReadOnly: selectOnly && !flags.walExclusive && !flags.walShared,
		NoCreate: selectOnly,
		WALExclusive: flags.walExclusive,
		WALShared: flags.walShared || flags.walReadOnly,
	}, nil
}
