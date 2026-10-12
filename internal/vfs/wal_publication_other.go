//go:build !linux

package vfs

import "os"

type unsupportedWALPublication struct{}
func newNativeWALPublication(main, original *File, mainInfo, originalInfo os.FileInfo) walPublicationOps {
	return unsupportedWALPublication{}
}
func (unsupportedWALPublication) allowedFiles() []*File { return nil }
func (unsupportedWALPublication) acquire() error { return ErrUnsupported }
func (unsupportedWALPublication) create() error { return ErrUnsupported }
func (unsupportedWALPublication) readSource([]byte, int64) (int, error) { return 0, ErrUnsupported }
func (unsupportedWALPublication) writeCandidate([]byte, int64) (int, error) { return 0, ErrUnsupported }
func (unsupportedWALPublication) candidateInfo() (os.FileInfo, error) { return nil, ErrUnsupported }
func (unsupportedWALPublication) recheck() error { return ErrUnsupported }
func (unsupportedWALPublication) syncCandidate() error { return ErrUnsupported }
func (unsupportedWALPublication) replace() (bool, error) { return false, ErrUnsupported }
func (unsupportedWALPublication) transfer(*File, *File) (*File, error) { return nil, ErrUnsupported }
func (unsupportedWALPublication) postPublication() (os.FileInfo, error) { return nil, ErrUnsupported }
func (unsupportedWALPublication) syncDirectory() error { return ErrUnsupported }
func (unsupportedWALPublication) seal() {}
func (unsupportedWALPublication) closeChild() (bool, error) { return true, nil }
func (unsupportedWALPublication) closeDirectory() (bool, error) { return true, nil }
