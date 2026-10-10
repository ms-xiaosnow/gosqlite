package btree

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// RangeLimits bounds a cursor's cumulative work, including its initial seek.
// Zero selects the documented finite default; negative values are invalid.
// Records and PayloadBytes include binary-search probes and repeated decodes,
// not only returned records. Work counts page reads, binary-search comparisons, record decodes,
// and bytes reserved by endpoint-copy and page-parser allocation admission. These are logical counters,
// not physical I/O, wall time, or exact Go heap usage.
type RangeLimits struct {
	PageReads, Work, Records, PayloadBytes int64
}

// RangeStats reports admitted work. Failed underlying ReadPage calls count;
// requests rejected by a limit do not. Returned counts successful Next results.
type RangeStats struct {
	PageReads, Work, Records, PayloadBytes, Returned int64
}

type rangeBudget struct {
	reader PageReader
	ctx    context.Context
	limits RangeLimits
	stats  RangeStats
}

func newRangeBudget(ctx context.Context, r PageReader, limits RangeLimits) (*rangeBudget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limits.PageReads < 0 || limits.Work < 0 || limits.Records < 0 || limits.PayloadBytes < 0 {
		return nil, fmt.Errorf("%w: negative range limit", record.ErrUnsupported)
	}
	if limits.PageReads == 0 {
		limits.PageReads = MaxVisitedPages
	}
	if limits.Work == 0 {
		limits.Work = 16 << 20
	}
	if limits.Records == 0 {
		limits.Records = 1 << 20
	}
	if limits.PayloadBytes == 0 {
		limits.PayloadBytes = 64 << 20
	}
	return &rangeBudget{reader: r, ctx: ctx, limits: limits}, nil
}
func (b *rangeBudget) Header() pager.Header { return b.reader.Header() }
func (b *rangeBudget) work(n int64) error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if n < 0 || n > b.limits.Work-b.stats.Work {
		return fmt.Errorf("%w: range work budget", record.ErrLimit)
	}
	b.stats.Work += n
	return nil
}
func (b *rangeBudget) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.stats.PageReads >= b.limits.PageReads {
		return nil, fmt.Errorf("%w: range page-read budget", record.ErrLimit)
	}
	if err := b.work(1); err != nil {
		return nil, err
	}
	b.stats.PageReads++
	p, err := b.reader.ReadPage(ctx, n)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p, nil
}
func (b *rangeBudget) BeforeRecord(n int) error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if n < 0 || b.stats.Records >= b.limits.Records || int64(n) > b.limits.PayloadBytes-b.stats.PayloadBytes {
		return fmt.Errorf("%w: range record/payload budget", record.ErrLimit)
	}
	if err := b.work(1); err != nil {
		return err
	}
	if prior, ok := b.reader.(interface{ BeforeRecord(int) error }); ok {
		if err := prior.BeforeRecord(n); err != nil {
			return err
		}
	}
	b.stats.Records++
	b.stats.PayloadBytes += int64(n)
	return b.ctx.Err()
}
func (b *rangeBudget) BeforePage(n int64) error {
	if err := b.work(n); err != nil {
		return err
	}
	if prior, ok := b.reader.(interface{ BeforePage(int64) error }); ok {
		if err := prior.BeforePage(n); err != nil {
			return err
		}
	}
	return b.ctx.Err()
}
