package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Pure memory error trees only. No database, filesystem mutation or native hooks.
func TestIndependentUnexpectedJoinedErrorsRemainFatal(t *testing.T) {
	unexpected := errors.New("independent model diagnostic")
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"busy_plus_unknown", errors.Join(sqlite.ErrBusy, unexpected)},
		{"busy_plus_changed", errors.Join(sqlite.ErrBusy, sqlite.ErrChanged)},
		{"cancel_plus_unknown", errors.Join(context.Canceled, unexpected)},
		{"deadline_plus_changed", errors.Join(context.DeadlineExceeded, sqlite.ErrChanged)},
		{"first_sqlite_error_masks_later_io", errors.Join(&sqlite.Error{Code: sqlite.CodeBusy, Op: "memory", Err: sqlite.ErrBusy}, &sqlite.Error{Code: sqlite.CodeIOErr, Op: "memory", Err: unexpected})},
		{"outer_busy_masks_inner_io", &sqlite.Error{Code: sqlite.CodeBusy, Op: "memory", Err: errors.Join(sqlite.ErrBusy, &sqlite.Error{Code: sqlite.CodeIOErr, Op: "memory", Err: unexpected})}},
		{"wrapped_busy_unknown", fmt.Errorf("context: %w", errors.Join(sqlite.ErrBusy, unexpected))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := Counts{Attempts: 1}
			fatal := classify(&c, tt.err)
			if !fatal || c.Failed != 1 || c.Busy != 0 || c.Canceled != 0 || ordinaryBusy(tt.err) {
				t.Fatalf("unexpected joined diagnostic suppressed: fatal=%v counts=%+v ordinaryBusy=%v err=%v", fatal, c, ordinaryBusy(tt.err), tt.err)
			}
		})
	}
}
