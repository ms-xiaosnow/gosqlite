package pager

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// PlaintextBackupPlatform is deliberately narrower than native database support.
// Darwin normalization and Windows alternate/short-name family equivalence have
// not been implemented for this backup API. Cross-compilation is not capability.
func PlaintextBackupPlatform() error { return backupNamespacePlatform(runtime.GOOS) }

func backupNamespacePlatform(platform string) error {
	if platform != "linux" {
		return fmt.Errorf("%w: plaintext backup namespace profile requires Linux", ErrUnsupported)
	}
	return nil
}

// backupFamilyDisjoint is descriptor-free. sameParent must come from identities
// of the source's original managed parent and the held destination directory,
// never a comparison of parent pathname spellings. Conservative ASCII folding
// also refuses case-only collisions on case-sensitive Linux filesystems. Every main name is bounded ASCII and rejects sidecar suffixes.
// The VFS capability repeats this admission with its retained physical facts.
func backupFamilyDisjoint(platform, sourceBase, destinationBase string, sameParent bool) error {
	if err := backupNamespacePlatform(platform); err != nil {
		return err
	}
	if !vfs.BackupBasename(sourceBase) || !vfs.BackupBasename(destinationBase) {
		return fmt.Errorf("%w: backup requires bounded non-sidecar ASCII main names", ErrUnsupported)
	}
	if !sameParent {
		return nil
	}
	if !backupSimpleBasename(sourceBase) || !backupSimpleBasename(destinationBase) {
		return fmt.Errorf("%w: same-directory backup requires simple ASCII basenames", ErrUnsupported)
	}
	for _, sourceSuffix := range []string{"", "-journal", "-wal", "-shm"} {
		for _, destinationSuffix := range []string{"", "-journal", "-wal", "-shm"} {
			if strings.EqualFold(sourceBase+sourceSuffix, destinationBase+destinationSuffix) {
				return fmt.Errorf("%w: backup source and destination families overlap", ErrUnsupported)
			}
		}
	}
	return nil
}

func backupSimpleBasename(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for i := 0; i < len(name); i++ {
		b := name[i]
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == '-') {
			return false
		}
	}
	return true
}
