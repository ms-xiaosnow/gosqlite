//go:build !(windows && amd64)

package vfs

import (
	"os"
	"path/filepath"
)

// Lstat observes a pathname without following its final component. On Windows
// the corresponding helper owns every metadata handle and its close outcome.
func Lstat(path string) (os.FileInfo, error) { return os.Lstat(path) }

// SameFile compares snapshots obtained through this package. It performs no
// lazy pathname opens on Windows and compares that platform's full file IDs.
func SameFile(a, b os.FileInfo) bool { return os.SameFile(a, b) }

// ResolvePath resolves aliases before selecting the database/sidecar family.
// Linux and Darwin keep their existing filepath.EvalSymlinks behavior.
func ResolvePath(path string) (string, error) { return filepath.EvalSymlinks(path) }

func nativeStat(f *File) (os.FileInfo, error) { return f.raw.Stat() }
func nativeParentInfo(f *File) (os.FileInfo, error) {
	return os.Stat(filepath.Dir(f.path))
}
func statParentPath(path string) (os.FileInfo, error) { return os.Stat(path) }
