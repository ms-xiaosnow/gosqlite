//go:build darwin

package encstore

import (
	"errors"
	"io"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Compile-time ABI checks: the parser below does not cast or retain native
// pointers. Both Darwin amd64/arm64 use these 64-bit direntry field offsets.
var (
	_ [16]byte                  = [unsafe.Offsetof(unix.Dirent{}.Reclen)]byte{}
	_ [18]byte                  = [unsafe.Offsetof(unix.Dirent{}.Namlen)]byte{}
	_ [20]byte                  = [unsafe.Offsetof(unix.Dirent{}.Type)]byte{}
	_ [darwinDirNameOffset]byte = [unsafe.Offsetof(unix.Dirent{}.Name)]byte{}
)

type darwinDirectoryStream struct {
	buffer      []byte
	cursor, end int
	eof         bool
	failed      error
}

// ReadDir is an internal bounded name enumerator. It must not call os.ReadDir,
// os.File.ReadDir, unix.Getdirentries or unix.ReadDirent: those Darwin wrappers
// acquire hidden directory descriptors and can discard native close failures.
func (f *managedFile) ReadDir(n int) ([]os.DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, e := f.rawLocked()
	if e != nil {
		return nil, e
	}
	if !f.descriptor.directory || n <= 0 || n > 64 {
		return nil, ErrUnsupported
	}
	stream := &f.directoryStream
	if stream.failed != nil {
		return nil, stream.failed
	}
	if stream.buffer == nil {
		stream.buffer = make([]byte, darwinDirBufferBytes)
	}
	entries := make([]os.DirEntry, 0, n)
	for len(entries) < n {
		if stream.cursor == stream.end {
			if stream.eof {
				break
			}
			count, e := darwinReadDirectory(raw, stream.buffer)
			if e != nil {
				stream.failed = e
				return entries, e
			}
			stream.cursor, stream.end = 0, count
			if count == 0 {
				stream.eof = true
				break
			}
		}
		entry, used, skip, e := nextDarwinDirectoryName(stream.buffer[stream.cursor:stream.end])
		if e != nil {
			stream.failed = e
			return entries, e
		}
		stream.cursor += used
		if !skip {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 && stream.eof {
		return entries, io.EOF
	}
	return entries, nil
}
func darwinReadDirectory(raw *os.File, buffer []byte) (int, error) {
	var position int64 // native off_t*, not uintptr on a guessed ABI
	for {
		n, _, errno := unix.Syscall6(unix.SYS_GETDIRENTRIES64, raw.Fd(), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&position)), 0, 0)
		runtime.KeepAlive(raw)
		runtime.KeepAlive(buffer)
		runtime.KeepAlive(&position)
		if errors.Is(errno, unix.EINTR) {
			continue
		}
		if errno != 0 {
			return 0, &os.PathError{Op: "getdirentries64", Path: raw.Name(), Err: errno}
		}
		// The kernel may put extended flags at the buffer's end. The return value
		// counts directory bytes only; never parse its full capacity or flags word.
		if n > uintptr(len(buffer)) {
			return 0, ErrChanged
		}
		return int(n), nil
	}
}
