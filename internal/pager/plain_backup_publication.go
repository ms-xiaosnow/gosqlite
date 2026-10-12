package pager

import "github.com/ms-xiaosnow/gosqlite/internal/vfs"

// The entire destination is one registry resource. Its interior native File
// tokens are never observed/adopted by the outer plaintext cleanup group.
type plainBackupPublication struct{ publication *vfs.BackupPublication }
func (o plainBackupPublication) Close() error { return nativeFileError(o.publication.Close()) }
func (o plainBackupPublication) RetirementComplete() bool { return o.publication.RetirementComplete() }

func newPlainBackupPublication() (*plainRetirement, *vfs.BackupPublication, error) {
	cleanup, err := plainRetirements.reserve()
	if err != nil { return nil, nil, err }
	publication := vfs.NewBackupPublicationOwner() // memory only
	cleanup.main = plainBackupPublication{publication: publication}
	return cleanup, publication, nil
}
