package pager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// RecoveryOptions bounds the explicit, internal rollback recovery operation.
// This operation is not wired into Open, OpenTx, or any public SQL entry point.
// Its physical recovery/failure behavior has NOT passed the acceptance gate.
type RecoveryOptions struct {
	Limits journal.ReplayLimits
}

// RecoveryResult is meaningful only on success. Recovered is false when no
// journal exists. A cold, malformed, unsupported or ambiguous journal is
// preserved and returns an error instead of pretending recovery completed.
type RecoveryResult struct {
	Recovered     bool
	RestoredPages uint32
	OriginalPages uint32
}

// RecoverRollback is the explicit internal DELETE-mode recovery operation.
// It requires a stable, single-link, regular database pathname on Linux.
// Advisory locks assume every writer follows the SQLite locking protocol;
// adversarial namespace replacement is outside this operation's guarantees.
//
// Sequence: SHARED; query RESERVED; inspect hot candidate; PENDING→EXCLUSIVE
// (never RESERVED); recheck; validate the complete bounded replay plan and
// restored header; sync journal and directory; restore pages; truncate; sync
// database; close/unlink journal; sync directory; close/unlock database.
// All error paths before unlink preserve the journal. After unlink, a sync or
// close failure is durability-uncertain. No automatic deletion of cold or
// empty journals and no master-journal interpretation is attempted.
//
// Cancellation is honored until the first database write/truncate. Once that
// phase starts it cannot be abandoned on cancellation. No call to this method
// is part of the ordinary automated test suite: physical hot recovery remains
// restricted, source-only and NOT_RUN pending independent authorized evidence.
func RecoverRollback(ctx context.Context, path string, opts RecoveryOptions) (result RecoveryResult, retErr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return result, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return result, err
	}
	db, err := openRecoveryFile(path, os.O_RDWR)
	if err != nil {
		return result, err
	}
	touched, removed := false, false
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			retErr = errors.Join(retErr, closeErr)
		}
		if retErr != nil && touched {
			retErr = errors.Join(ErrPoisoned, retErr)
			if removed {
				retErr = errors.Join(ErrCommitUncertain, retErr)
			} else {
				retErr = errors.Join(ErrRecoveryRequired, retErr)
			}
		}
		if retErr != nil {
			result = RecoveryResult{}
		}
	}()
	info, err := db.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("%w: recovery requires a regular database", ErrUnsupported)
	}
	if err = validateWritableFile(db); err != nil {
		return result, err
	}
	if err = lockShared(db); err != nil {
		return result, err
	}
	if err = recoveryCheckPath(path, db, info); err != nil {
		return result, err
	}
	if err = recoveryCheckWAL(path); err != nil {
		return result, err
	}
	jpath := path + "-journal"
	if _, err = os.Lstat(jpath); os.IsNotExist(err) {
		return result, nil
	} else if err != nil {
		return result, err
	}
	if err = recoveryCheckReserved(db); err != nil {
		return result, err
	}
	// The first-byte check is SQLite's hot-candidate test, not proof that the
	// journal is valid. Cold files are left untouched and fail closed.
	candidate, err := recoveryOpenJournal(jpath)
	if err != nil {
		return result, err
	}
	var first [1]byte
	_, readErr := candidate.ReadAt(first[:], 0)
	closeErr := candidate.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return result, errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return result, closeErr
	}
	if first[0] == 0 {
		return result, fmt.Errorf("%w: cold or empty journal preserved", ErrRecoveryRequired)
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	// lockExclusive acquires PENDING and the shared-byte range; it does not
	// acquire RESERVED. Closing db releases every acquired OFD lock on error.
	if err = lockExclusive(db); err != nil {
		return result, err
	}
	if err = recoveryCheckReserved(db); err != nil {
		return result, err
	}
	if err = recoveryCheckPath(path, db, info); err != nil {
		return result, err
	}
	if err = recoveryCheckWAL(path); err != nil {
		return result, err
	}
	info, err = db.Stat()
	if err != nil {
		return result, err
	}
	// SQLite can discard journals beside an empty database. We cannot prove
	// which database incarnation such a journal belongs to, so preserve it.
	if info.Size() == 0 {
		return result, fmt.Errorf("%w: journal beside empty database is ambiguous", ErrRecoveryRequired)
	}
	jf, err := recoveryOpenJournal(jpath)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer func() {
		if jf != nil {
			retErr = errors.Join(retErr, jf.Close())
		}
	}()
	ji, err := jf.Stat()
	if err != nil {
		return result, err
	}
	limits := opts.Limits.WithDefaults()
	if ji.Size() < 0 || uint64(ji.Size()) > limits.MaxJournalBytes || uint64(ji.Size()) > uint64(^uint(0)>>1) {
		return result, fmt.Errorf("%w: journal bytes", ErrLimit)
	}
	data := make([]byte, int(ji.Size()))
	if _, err = io.ReadFull(jf, data); err != nil {
		return result, err
	}
	plan, err := journal.ParseReplay(data, limits)
	if err != nil {
		return result, recoveryPlanError(err)
	}
	if err = plan.ValidateExtent(info.Size()); err != nil {
		return result, recoveryPlanError(err)
	}
	if err = recoveryValidateHeader(db, plan); err != nil {
		return result, err
	}
	if err = recoveryCheckUnchanged(jpath, jf, ji); err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	// The journal must be durable before replay itself could be interrupted.
	if err = jf.Sync(); err != nil {
		return result, err
	}
	if err = syncDirectory(path); err != nil {
		return result, err
	}
	if err = recoveryCheckUnchanged(jpath, jf, ji); err != nil {
		return result, err
	}
	if err = recoveryCheckPath(path, db, info); err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	touched = true
	for _, page := range plan.Pages {
		if err = writeFullAt(db, page.Data, int64(page.Number-1)*int64(plan.Header.PageSize)); err != nil {
			return result, err
		}
	}
	if err = db.Truncate(plan.DatabaseBytes); err != nil {
		return result, err
	}
	if err = db.Sync(); err != nil {
		return result, err
	}
	if err = recoveryCheckUnchanged(jpath, jf, ji); err != nil {
		return result, err
	}
	if err = recoveryCheckPath(path, db, info); err != nil {
		return result, err
	}
	err = jf.Close()
	jf = nil
	if err != nil {
		return result, err
	}
	if err = os.Remove(jpath); err != nil {
		return result, err
	}
	removed = true
	if err = syncDirectory(path); err != nil {
		return result, err
	}
	return RecoveryResult{Recovered: true, RestoredPages: uint32(len(plan.Pages)), OriginalPages: plan.Header.OriginalPages}, nil
}

func recoveryCheckReserved(db *vfs.File) error {
	reserved, err := recoveryReservedHeld(db)
	if err != nil {
		return err
	}
	if reserved {
		return fmt.Errorf("%w: active writer owns RESERVED; journal is not recoverable now", ErrBusy)
	}
	return nil
}

func recoveryCheckWAL(path string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Lstat(path + suffix); err == nil {
			return fmt.Errorf("%w: rollback recovery with WAL sidecar %s", ErrUnsupported, suffix)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func recoveryOpenJournal(path string) (*vfs.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: recovery journal must be a regular file", ErrUnsupported)
	}
	file, err := openJournal(path, os.O_RDWR)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err == nil {
		err = validateWritableFile(file)
	}
	if err == nil && (!opened.Mode().IsRegular() || !os.SameFile(info, opened)) {
		err = ErrChanged
	}
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func recoveryCheckPath(path string, file *vfs.File, original os.FileInfo) error {
	if err := validateNativePath(file); err != nil {
		return err
	}
	current, err := os.Lstat(path)
	if err != nil {
		return errors.Join(ErrChanged, err)
	}
	if !current.Mode().IsRegular() || !os.SameFile(current, original) {
		return fmt.Errorf("%w: recovery pathname identity", ErrChanged)
	}
	return validateWritableFile(file)
}

func recoveryCheckUnchanged(path string, file *vfs.File, original os.FileInfo) error {
	if err := recoveryCheckPath(path, file, original); err != nil {
		return err
	}
	current, err := file.Stat()
	if err != nil {
		return err
	}
	if current.Size() != original.Size() || !current.ModTime().Equal(original.ModTime()) {
		return fmt.Errorf("%w: recovery journal bytes changed", ErrChanged)
	}
	return nil
}

func recoveryValidateHeader(db *vfs.File, plan *journal.ReplayPlan) error {
	if plan.Header.OriginalPages == 0 {
		return nil
	}
	var headerBytes []byte
	for _, page := range plan.Pages {
		if page.Number == 1 {
			headerBytes = page.Data
			break
		}
	}
	if headerBytes == nil {
		headerBytes = make([]byte, 100)
		if _, err := db.ReadAt(headerBytes, 0); err != nil {
			return fmt.Errorf("%w: original header unavailable: %v", ErrCorrupt, err)
		}
	}
	header, err := ParseHeader(headerBytes, plan.DatabaseBytes)
	if err != nil {
		return err
	}
	if header.PageSize != plan.Header.PageSize || header.PageCount != plan.Header.OriginalPages {
		return fmt.Errorf("%w: original header disagrees with journal geometry", ErrCorrupt)
	}
	if header.ReservedBytes != 0 {
		return fmt.Errorf("%w: rollback recovery of reserved-byte pages", ErrUnsupported)
	}
	return nil
}

func recoveryPlanError(err error) error {
	if errors.Is(err, journal.ErrReplayLimit) {
		return errors.Join(ErrRecoveryRequired, ErrLimit, err)
	}
	if errors.Is(err, journal.ErrReplayUnsupported) {
		return errors.Join(ErrRecoveryRequired, ErrUnsupported, err)
	}
	return errors.Join(ErrRecoveryRequired, ErrCorrupt, err)
}
