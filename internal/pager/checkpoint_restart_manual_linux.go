//go:build linux && manual_checkpoint_restart

package pager

import (
	"context"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/checkpointrestart"
)

func checkpointObserve(ctx context.Context, point checkpointPoint, path string, page, selected, size, frames uint32, salt, checksum [2]uint32, safe, backfill uint32) {
	checkpointrestart.ManualPageObserve(ctx, uint8(point), path, checkpointrestart.ManualPage{Page: page, Selected: selected, DBSize: size, Frames: frames, Salt: salt, Checksum: checksum, Safe: safe, Backfill: backfill})
}
