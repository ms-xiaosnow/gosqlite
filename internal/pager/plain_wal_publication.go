package pager

import (
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// This narrow adapter adds no registry slot and changes no generic close order.
// Native allocation is memory-only; install precedes every Acquire syscall.
type plainWALPublicationResource interface {
	plainResource
	TransferWAL(*vfs.File, *vfs.File) (*vfs.File, error)
}
func (g *plainRetirement) reserveWALPublication(main, old *vfs.File, mainInfo, oldInfo os.FileInfo) (*vfs.WALPublication, error) {
	if err := g.reserveTemporary(); err != nil { return nil, err }
	owner := vfs.NewWALPublication(main, old, mainInfo, oldInfo)
	if err := g.installWALPublication(owner); err != nil { return nil, err }
	return owner, nil
}
func (g *plainRetirement) installWALPublication(owner plainWALPublicationResource) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started || g.done || g.cause != nil || !g.temporaryReserved || g.temporary != nil || owner == nil { return ErrPoisoned }
	g.temporary = owner
	g.temporaryReserved = false
	return nil
}
func (g *plainRetirement) transferWALPublication(owner plainWALPublicationResource, old *vfs.File) (*vfs.File, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	main, mainOK := g.main.(plainFile)
	current, walOK := g.wal.(plainFile)
	if g.started || g.done || g.cause != nil || g.temporaryRequested || g.walRequested ||
		g.temporary != owner || !mainOK || !walOK || main.file == nil || current.file != old || old == nil || main.file == old {
		return nil, ErrPoisoned
	}
	// Memory-only checked exchange. Both file owners stay reachable under this
	// mutex; the composite retains the superseded child before its directory.
	next, err := owner.TransferWAL(main.file, old)
	if err != nil { return nil, nativeFileError(err) }
	if next == nil || next == old || next == main.file { return nil, ErrPoisoned }
	g.wal = plainFile{next}
	return next, nil
}
