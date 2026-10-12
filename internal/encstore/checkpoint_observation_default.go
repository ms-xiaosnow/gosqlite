//go:build !linux || !manual_checkpoint_restart

package encstore

import "context"

func checkpointPublicationObserve(context.Context, checkpointPublicationPoint, string, []byte, []byte) {
}
