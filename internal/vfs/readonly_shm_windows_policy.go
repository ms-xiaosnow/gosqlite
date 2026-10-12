package vfs

// windowsDMSObservation describes this backend's actual handshake observation,
// not a Unix peer-lock query. Zero and unrecognized values grant no authority.
// LocalAttached means an existing registry attachment, not this owner's lock.
// ProbeContended permits a shared attempt; it does not mean that attempt worked.
type windowsDMSObservation uint8

const (
	windowsDMSUnknown windowsDMSObservation = iota
	windowsDMSLocalAttached
	windowsDMSProbeAcquired
	windowsDMSProbeContended
)

// readonlyWindowsDMSDecision is descriptor-free. The caller must retire an
// acquired exclusive probe even when this helper refuses it, preserving any
// cleanup error. Only successful shared acquisition may publish attachment.
func readonlyWindowsDMSDecision(writable bool, observation windowsDMSObservation) (initialize bool, err error) {
	switch observation {
	case windowsDMSLocalAttached, windowsDMSProbeContended:
		return false, nil
	case windowsDMSProbeAcquired:
		if writable {
			return true, nil
		}
		return false, ErrUnsupported
	default:
		return false, ErrUnsupported
	}
}
