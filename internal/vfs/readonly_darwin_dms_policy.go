package vfs

// darwinDMSPeer describes a successful external F_GETLK observation. The zero
// value is deliberately unknown, never evidence that initialization is safe.
// Keep this policy portable: it owns no descriptor and performs no native I/O.
type darwinDMSPeer uint8

const (
	darwinDMSUnknown darwinDMSPeer = iota
	darwinDMSUnheld
	darwinDMSShared
	darwinDMSExclusive
)

// readonlyDarwinDMSDecision is consumed by nativeAttachDMS before its exclusive
// acquisition or truncate. The native process registry handles a tracked local
// shared hold first because F_GETLK does not report this process's own locks.
// Shared admission does not initialize or prove immutability. Public read-only
// shared WAL remains Linux-only; this is internal Darwin backend groundwork.
func readonlyDarwinDMSDecision(writable bool, peer darwinDMSPeer) (initialize bool, err error) {
	switch peer {
	case darwinDMSUnheld:
		if !writable {
			return false, ErrUnsupported
		}
		return true, nil
	case darwinDMSShared:
		return false, nil
	case darwinDMSExclusive:
		return false, ErrBusy
	default:
		return false, ErrUnsupported
	}
}
