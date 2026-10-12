package encstore

// Checkpoint and rekey have independent purposes and independent manual tags.
type checkpointPublicationPoint uint8

const (
	checkpointBeforeRootRename checkpointPublicationPoint = iota + 1
	checkpointAfterRootRename
	checkpointAfterRootDirSync
)
