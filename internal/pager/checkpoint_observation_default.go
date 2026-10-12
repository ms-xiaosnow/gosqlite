//go:build !linux || !manual_checkpoint_restart

package pager

import "context"

func checkpointObserve(context.Context, checkpointPoint, string, uint32, uint32, uint32, uint32, [2]uint32, [2]uint32, uint32, uint32) {
}
