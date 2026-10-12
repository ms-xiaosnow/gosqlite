package encstore

// Private observation vocabulary. The ordinary implementation is inert.
type rekeyPublicationPoint uint8

const (
	rekeyBeforeRootRename rekeyPublicationPoint = iota + 1
	rekeyAfterRootRename
	rekeyAfterRootDirSync
)
