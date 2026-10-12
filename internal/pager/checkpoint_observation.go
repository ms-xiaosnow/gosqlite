package pager

// Private scalar vocabulary. The default build observes nothing.
type checkpointPoint uint8

const (
	checkpointExclusiveCopy checkpointPoint = iota + 1
	checkpointExclusiveRetired
	checkpointSharedCopy
	checkpointSharedBackfill
)
