//go:build !linux

package vfs

// Non-Linux construction remains memory-only; every operation fails closed.
type unsupportedBackupPublication struct{}
func newNativeBackupPublication() backupPublicationIO { return unsupportedBackupPublication{} }
func (unsupportedBackupPublication) openDirectory(string) (backupFact, error) { return backupFact{}, ErrUnsupported }
func (unsupportedBackupPublication) absentFamily(string) error { return ErrUnsupported }
func (unsupportedBackupPublication) createCandidate(string) error { return ErrUnsupported }
func (unsupportedBackupPublication) lockCandidate() error { return ErrUnsupported }
func (unsupportedBackupPublication) validateCandidate() (uint64, error) { return 0, ErrUnsupported }
func (unsupportedBackupPublication) writeCandidate([]byte, int64) (int, error) { return 0, ErrUnsupported }
func (unsupportedBackupPublication) syncCandidate() error { return ErrUnsupported }
func (unsupportedBackupPublication) publish(string, string) error { return ErrUnsupported }
func (unsupportedBackupPublication) validatePublished() error { return ErrUnsupported }
func (unsupportedBackupPublication) syncDirectory() error { return ErrUnsupported }
func (unsupportedBackupPublication) seal() error { return nil }
func (unsupportedBackupPublication) closeDirectory() error { return nil }
func (unsupportedBackupPublication) directoryComplete() bool { return true }
func (unsupportedBackupPublication) closeMain() error { return nil }
func (unsupportedBackupPublication) mainComplete() bool { return true }
func (unsupportedBackupPublication) ownedFiles() []*File { return nil }
