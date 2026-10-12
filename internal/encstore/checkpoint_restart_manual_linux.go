//go:build linux && manual_checkpoint_restart

package encstore

import (
	"context"
	"crypto/sha256"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/checkpointrestart"
)

func checkpointPublicationObserve(ctx context.Context, point checkpointPublicationPoint, path string, oldRoot, candidateRoot []byte) {
	if !checkpointrestart.ManualArmed(ctx) {
		return
	}
	// Layout 2: one selected segment before checkpoint, zero afterward.
	// These bytes were authenticated by Publish; public decoding adds no claim.
	if len(oldRoot) != 1320 || len(candidateRoot) != 1064 {
		checkpointrestart.ManualAbort()
	}
	old, oldErr := encryption.DecodeHeader(oldRoot[:encryption.HeaderSize], encryption.Limits{})
	candidate, candidateErr := encryption.DecodeHeader(candidateRoot[:encryption.HeaderSize], encryption.Limits{})
	if oldErr != nil || candidateErr != nil {
		checkpointrestart.ManualAbort()
	}
	checkpointrestart.ManualRootObserve(ctx, uint8(point)+4, path, checkpointrestart.ManualRoot{OldHash: sha256.Sum256(oldRoot), CandidateHash: sha256.Sum256(candidateRoot), Old: old, Candidate: candidate})
}
