//go:build windows && amd64

package encstore

// CanonicalWindowsPath is the public SQL connection's Windows prelude. Reuse
// the backend's managed, bounded metadata probes and exact canonical path so
// DB.path and KeyRequest.Path describe the same file family. It performs no
// key-provider call, lock creation, data write or publication. The subsequent
// storage open repeats its own checks under the actual retained namespace.
func CanonicalWindowsPath(path string) (string, error) { return canonicalPath(path) }
