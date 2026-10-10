package btree

import (
	"context"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// RootSpec identifies a validated catalog object's physical tree. The catalog
// itself (page 1) is implicit and must not be supplied again.
type RootSpec struct {
	Page    uint32
	Kind    TreeKind
	KeyInfo KeyInfo
}

// CheckOwnership verifies that every logical page is owned by exactly one
// known tree (including overflow), the pager-validated freelist, or SQLite's
// reserved lock-byte page. Call before the first mutation of an untrusted
// existing database, while retaining its transaction snapshot. This is not an
// index-to-table consistency check or a substitute for crash qualification.
//
// freePages must include both trunk and leaf pages from the validated freelist.
// The page budget bounds all retained ownership metadata. Cursors already
// validate within-tree ordering, bounds, cycles and record/overflow encodings.
func CheckOwnership(ctx context.Context, r PageReader, roots []RootSpec, freePages []uint32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h := r.Header()
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 {
		return fmt.Errorf("%w: ownership page size", pager.ErrCorrupt)
	}
	if h.PageCount == 0 {
		return fmt.Errorf("%w: empty ownership extent", pager.ErrCorrupt)
	}
	if h.PageCount > MaxVisitedPages {
		return fmt.Errorf("%w: database ownership page budget", record.ErrLimit)
	}
	lockPage := uint32(0x40000000/h.PageSize + 1)
	owners := make(map[uint32]uint32)
	claim := func(page, owner uint32) error {
		if page == 0 || page > h.PageCount || page == lockPage {
			return corrupt("invalid owned page %d", page)
		}
		if previous, ok := owners[page]; ok {
			return corrupt("page %d has multiple owners (%d and %d)", page, previous, owner)
		}
		owners[page] = owner
		return nil
	}
	for _, page := range freePages {
		if page == 1 {
			return corrupt("schema root listed as free")
		}
		if err := claim(page, 0); err != nil {
			return err
		}
	}
	all := make([]RootSpec, 0, len(roots)+1)
	all = append(all, RootSpec{Page: 1, Kind: TableTree})
	all = append(all, roots...)
	seenRoots := make(map[uint32]bool)
	for _, root := range all {
		if err := ctx.Err(); err != nil {
			return err
		}
		if seenRoots[root.Page] {
			return corrupt("duplicate catalog root %d", root.Page)
		}
		seenRoots[root.Page] = true
		tracked := ownershipReader{PageReader: r, claim: func(page uint32) error {
			if previous, ok := owners[page]; ok && previous == root.Page {
				return nil
			}
			return claim(page, root.Page)
		}}
		switch root.Kind {
		case TableTree:
			c, err := NewCursor(ctx, tracked, root.Page)
			if err != nil {
				return err
			}
			for {
				_, _, err = c.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					c.Close()
					return err
				}
			}
			c.Close()
		case IndexTree:
			c, err := NewIndexCursor(ctx, tracked, root.Page, root.KeyInfo)
			if err != nil {
				return err
			}
			for {
				_, err = c.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					c.Close()
					return err
				}
			}
			c.Close()
		default:
			return fmt.Errorf("%w: unknown ownership tree kind", pager.ErrUnsupported)
		}
	}
	for page := uint32(1); page <= h.PageCount; page++ {
		if page == lockPage {
			continue
		}
		if _, ok := owners[page]; !ok {
			return corrupt("page %d has no known owner", page)
		}
		if page%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

// Record ownership while pages are read; cursors intentionally release their
// own visited maps at EOF, and no live cursor internals need to be retained.
type ownershipReader struct {
	PageReader
	claim func(uint32) error
}

func (r ownershipReader) ReadPage(ctx context.Context, page uint32) ([]byte, error) {
	if err := r.claim(page); err != nil {
		return nil, err
	}
	return r.PageReader.ReadPage(ctx, page)
}
