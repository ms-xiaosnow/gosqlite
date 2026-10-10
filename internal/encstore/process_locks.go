package encstore

import "errors"

// processByteLocks is Darwin's process-fcntl ledger. Per-lease logical holds
// remain distinct from the aggregate successful kernel mode. It is pure state:
// the callback is native only in the Darwin adapter, and synthetic in tests.
type processByteOwner struct{ held [3]uint8 }
type processByteLocks struct {
	owners   map[*processByteOwner]struct{}
	kernel   [3]uint8
	poisoned error
}

func (l *processByteLocks) attach(o *processByteOwner) {
	if l.owners == nil {
		l.owners = make(map[*processByteOwner]struct{})
	}
	l.owners[o] = struct{}{}
}
func (l *processByteLocks) poison(err error) error {
	l.poisoned = errors.Join(l.poisoned, ErrPoisoned, err)
	return l.poisoned
}
func (l *processByteLocks) any() bool { return l.kernel != [3]uint8{} }
func (l *processByteLocks) change(o *processByteOwner, offset int64, shared, unlock bool, call byteLockCall) error {
	if l.poisoned != nil {
		return l.poisoned
	}
	return l.transition(o, offset, shared, unlock, call)
}
func (l *processByteLocks) transition(o *processByteOwner, offset int64, shared, unlock bool, call byteLockCall) error {
	if offset < lifecycleByte || offset > captureByte {
		return ErrUnsupported
	}
	if _, ok := l.owners[o]; !ok {
		return l.poison(ErrUnsupported)
	}
	want := uint8(2)
	if shared {
		want = 1
	}
	if unlock {
		want = 0
	}
	if o.held[offset] == want {
		return nil
	}
	other := uint8(0)
	for p := range l.owners {
		if p != o && p.held[offset] > other {
			other = p.held[offset]
		}
	}
	if !unlock && (other == 2 || want == 2 && other != 0) {
		return ErrBusy
	}
	aggregate := want
	if other > aggregate {
		aggregate = other
	}
	if l.kernel[offset] != aggregate {
		if err := call(offset, aggregate == 1, aggregate == 0); err != nil {
			if !unlock && errors.Is(err, ErrBusy) {
				return err
			}
			return l.poison(err)
		}
		l.kernel[offset] = aggregate
	}
	o.held[offset] = want
	return nil
}

// Cleanup may retry a represented failed unlock even after poisoning. It never
// fabricates an unlock/relock conversion or removes a successful hold on BUSY.
func (l *processByteLocks) close(o *processByteOwner, call byteLockCall) error {
	for off := captureByte; off >= lifecycleByte; off-- {
		if err := l.transition(o, off, false, true, call); err != nil {
			return err
		}
	}
	delete(l.owners, o)
	return nil
}
