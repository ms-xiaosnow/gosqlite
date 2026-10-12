//go:build !linux

package vfs

func nativeBackupOpenDirectory(*backupLease, string) error { return ErrUnsupported }
func nativeBackupObserveSource(*File, *backupLease, string) (backupFact, error) {
	return backupFact{}, ErrUnsupported
}
