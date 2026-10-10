package encstore

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

const (
	DefaultMaxGenerations      uint64 = 256
	DefaultMaxRetainedBytes    uint64 = 1 << 30
	DefaultMaxDirectoryEntries uint32 = 16384
	HardMaxGenerations         uint64 = 1 << 20
	HardMaxRetainedBytes       uint64 = 1 << 40
	HardMaxDirectoryEntries    uint32 = 1 << 20
)

func (o Options) retentionLimits() (generations, bytes uint64, entries uint32, err error) {
	generations, bytes, entries = o.MaxGenerations, o.MaxRetainedBytes, o.MaxDirectoryEntries
	if generations == 0 {
		generations = DefaultMaxGenerations
	}
	if bytes == 0 {
		bytes = DefaultMaxRetainedBytes
	}
	if entries == 0 {
		entries = DefaultMaxDirectoryEntries
	}
	if generations > HardMaxGenerations || bytes > HardMaxRetainedBytes || entries > HardMaxDirectoryEntries {
		return 0, 0, 0, encryption.ErrLimit
	}
	return
}
func familyEntry(name, base string) (bool, error) {
	if name == base || name == base+".gse-lock" {
		return true, nil
	}
	prefix := base + ".gse-"
	if !strings.HasPrefix(name, prefix) {
		return false, nil
	}
	tail := strings.TrimPrefix(name, prefix)
	if len(tail) < 65 {
		return false, ErrUnsupported
	}
	for _, c := range tail[:64] {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false, ErrUnsupported
		}
	}
	switch tail[64:] {
	case ".base", ".retained", ".pending":
		return true, nil
	default:
		return false, ErrUnsupported
	}
}
func charge(total, amount, maximum uint64) (uint64, error) {
	if total > maximum || amount > maximum-total {
		return 0, encryption.ErrLimit
	}
	return total + amount, nil
}

// inventory uses a bounded batched directory scan and charges every physical
// family object, including unreferenced candidates from previous attempts.
// Unknown family names, links, symlinks and nonregular entries fail closed.
// A dedicated directory avoids quota refusal caused by unrelated entry counts.
func inventory(ctx context.Context, path string, maxBytes uint64, maxEntries uint32) (uint64, error) {
	dir := filepath.Dir(path)
	f, err := openDirectory(dir)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	di, err := f.Stat()
	if err != nil || !di.IsDir() {
		if err != nil {
			return 0, err
		}
		return 0, ErrUnsupported
	}
	base := filepath.Base(path)
	var total uint64
	var scanned uint32
	for {
		if err = ctx.Err(); err != nil {
			return 0, err
		}
		n := 64
		if remain := uint64(maxEntries) - uint64(scanned) + 1; remain < 64 {
			n = int(remain)
		}
		batch, e := f.ReadDir(n)
		for _, entry := range batch {
			scanned++
			if scanned > maxEntries {
				return 0, encryption.ErrLimit
			}
			member, e := familyEntry(entry.Name(), base)
			if e != nil {
				return 0, e
			}
			if !member {
				continue
			}
			obj, info, e := checkedFile(filepath.Join(dir, entry.Name()), os.O_RDONLY)
			if e != nil {
				return 0, e
			}
			e = obj.Close()
			if e != nil {
				return 0, e
			}
			if info.Size() < 0 {
				return 0, encryption.ErrFormat
			}
			total, e = charge(total, uint64(info.Size()), maxBytes)
			if e != nil {
				return 0, e
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return 0, e
		}
	}
	// Detect a changed parent entry before returning. Filesystem adversaries can
	// still race path-based operations; this is not a pinned-dirfd namespace API.
	named, err := os.Lstat(dir)
	if err != nil {
		return 0, err
	}
	if !named.IsDir() || !os.SameFile(di, named) {
		return 0, ErrChanged
	}
	return total, nil
}
func publicationBudget(ctx context.Context, path string, o Options, g Geometry, next uint64, hasRoot bool) error {
	generations, maxBytes, maxEntries, err := o.retentionLimits()
	if err != nil {
		return err
	}
	if next > generations {
		return encryption.ErrLimit
	}
	// The brand-new-family preflight runs before acquire creates its lock.
	// Reserve that known entry so repeating the inventory under the new lease
	// cannot fail solely because of our own intervening lock creation.
	if !hasRoot {
		if _, e := os.Lstat(path + ".gse-lock"); os.IsNotExist(e) {
			if maxEntries == 0 {
				return encryption.ErrLimit
			}
			maxEntries--
		} else if e != nil {
			return e
		}
	}
	used, err := inventory(ctx, path, maxBytes, maxEntries)
	if err != nil {
		return err
	}
	// Charge the complete new base and root candidate, and conservatively reserve
	// a copy of the current root while its old pathname still exists.
	reserve := objectSize(g) + rootSize
	if hasRoot {
		reserve += rootSize
	}
	_, err = charge(used, reserve, maxBytes)
	return err
}
