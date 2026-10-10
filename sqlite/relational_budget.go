package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// ErrQueryWorkLimit means the complete relational operation exceeded its work
// quota. Earlier streaming rows are not a complete answer; always check Rows.Err.
var ErrQueryWorkLimit = errors.New("relational query work limit exceeded")

// RelationalLimits bounds the multi-source FROM executor. Zero uses 16,777,216
// work units and Options.MaxSortBytes respectively. MaxBytes can only lower that
// connection memory cap. These limits do not change the legacy single-table or
// ANN limits. Cancellation applies in addition to the quotas, including child
// reopen, ON/filter/projection evaluation and sorting. No spill is performed.
type RelationalLimits struct{ MaxWork, MaxBytes int64 }
type relationalLimitsKey struct{}

// WithRelationalLimits attaches limits to one operation context. Negative limits
// are rejected by Query, before opening child streams. It creates no goroutine.
func WithRelationalLimits(ctx context.Context, limits RelationalLimits) context.Context {
	return context.WithValue(ctx, relationalLimitsKey{}, limits)
}

// RelationalStats describes actual multi-source work, including discarded join
// pairs and eager sort work. Bytes are conservative charged state, not RSS.
type RelationalStats struct {
	Work, SourceOpens, SourceRows, PageReads, JoinPairs, NullExtensions int64
	CurrentBytes, PeakBytes                                             int64
}

func (r *Rows) RelationalStats() RelationalStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.relational == nil {
		return RelationalStats{}
	}
	return r.relational.stats
}

type relationalBudget struct {
	failed                   error
	ctx                      context.Context
	maxWork, maxBytes        int64
	stats                    RelationalStats
	scalarBytes, resultBytes int64
}

func newRelationalBudget(ctx context.Context, maxBytes int64) (*relationalBudget, error) {
	limits, _ := ctx.Value(relationalLimitsKey{}).(RelationalLimits)
	if limits.MaxWork < 0 || limits.MaxBytes < 0 {
		return nil, fmt.Errorf("relational limits must be nonnegative")
	}
	if limits.MaxWork == 0 {
		limits.MaxWork = 1 << 24
	}
	if limits.MaxBytes == 0 || limits.MaxBytes > maxBytes {
		limits.MaxBytes = maxBytes
	}
	return &relationalBudget{ctx: ctx, maxWork: limits.MaxWork, maxBytes: limits.MaxBytes}, nil
}
func (b *relationalBudget) step(n int64) error {
	if b.failed != nil {
		return b.failed
	}
	if err := b.ctx.Err(); err != nil {
		b.failed = err
		return err
	}
	if n < 0 || n > b.maxWork-b.stats.Work {
		b.failed = ErrQueryWorkLimit
		return b.failed
	}
	b.stats.Work += n
	return nil
}
func (b *relationalBudget) reserve(n int64) error {
	if b.failed != nil {
		return b.failed
	}
	if err := b.ctx.Err(); err != nil {
		b.failed = err
		return err
	}
	if n < 0 || n > b.maxBytes-b.stats.CurrentBytes {
		b.failed = ErrMemoryLimit
		return b.failed
	}
	b.stats.CurrentBytes += n
	b.stats.PeakBytes = max(b.stats.PeakBytes, b.stats.CurrentBytes)
	return nil
}
func (b *relationalBudget) release(n int64) { b.stats.CurrentBytes -= n }

// Pages are conservatively retained for the lifetime of each cursor. This
// covers its path, parsed cell arrays and visited-page map, even after a page
// leaves that path. Payload/decode allocations have a separate preflight.
type relationalReader struct {
	base   btree.PageReader
	budget *relationalBudget
	held   int64
}

func (r *relationalReader) Header() pager.Header { return r.base.Header() }
func (r *relationalReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := r.budget.step(int64(r.Header().PageSize)); err != nil {
		return nil, err
	}
	charge := int64(r.Header().PageSize) + 512
	if err := r.budget.reserve(charge); err != nil {
		return nil, err
	}
	r.held += charge
	r.budget.stats.PageReads++
	return r.base.ReadPage(ctx, n)
}
func (r *relationalReader) close() { r.budget.release(r.held); r.held = 0 }

func (b *relationalBudget) Step(n int64) error { return b.step(n) }
func (b *relationalBudget) Allocate(n int64) error {
	if err := b.reserve(n); err != nil {
		return err
	}
	b.scalarBytes += n
	return nil
}
func (b *relationalBudget) clearScalar() { b.release(b.scalarBytes); b.scalarBytes = 0 }
func (b *relationalBudget) clearResult() { b.release(b.resultBytes); b.resultBytes = 0 }

// Schema cursors use the same pre-allocation gate as execution leaves.
func (r *relationalReader) BeforeRecord(n int) error {
	if err := r.budget.step(int64(n)); err != nil {
		return err
	}
	cost := int64(n)*128 + 512
	if err := r.budget.reserve(cost); err != nil {
		return err
	}
	r.held += cost
	return nil
}

// B-tree parse allocations are charged by their exact preflighted cell/span
// capacities, separate from the page bytes and visited-map/frame allowance.
func (r *relationalReader) BeforePage(n int64) error {
	if err := r.budget.step(n); err != nil {
		return err
	}
	if err := r.budget.reserve(n); err != nil {
		return err
	}
	r.held += n
	return nil
}

// One budget is created after the globally bounded parser and before any
// engine-owned parameter clone. Query's snapshot and all children use it.
type relationalRootBudgetKey struct{}

func (d *DB) bindSelect(ctx context.Context, sel *sqlast.Select, args []any) (context.Context, []any, error) {
	if sel.Relation == nil {
		params, err := bind(sel, args)
		return ctx, params, err
	}
	budget, err := newRelationalBudget(ctx, d.opts.MaxSortBytes)
	if err != nil {
		return ctx, nil, err
	}
	if err := budget.reserve(int64(len(sel.SQL))*64 + 1024); err != nil {
		return ctx, nil, err
	}
	params, err := bindBudget(sel, args, budget)
	if err != nil {
		return ctx, nil, err
	}
	return context.WithValue(ctx, relationalRootBudgetKey{}, budget), params, nil
}
