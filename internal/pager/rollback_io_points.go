package pager

// rollbackIOPoint names source sites, not a process-wide I/O ordinal.
// Directory points cover the complete existing open/Sync/Close helper.
type rollbackIOPoint string

const (
	rbJCreate        rollbackIOPoint = "RB.JCREATE"
	rbJInvalid       rollbackIOPoint = "RB.JINVALID"
	rbDBOriginalRead rollbackIOPoint = "RB.DB_ORIGINAL_READ"
	rbJRecord        rollbackIOPoint = "RB.JRECORD"
	rbJBodySync      rollbackIOPoint = "RB.JBODY_SYNC"
	rbJValidHeader   rollbackIOPoint = "RB.JVALID_HEADER"
	rbJHeaderSync    rollbackIOPoint = "RB.JHEADER_SYNC"
	rbPublishDirSync rollbackIOPoint = "RB.PUBLISH_DIRSYNC"
	rbDBPage         rollbackIOPoint = "RB.DB_PAGE"
	rbDBSync         rollbackIOPoint = "RB.DB_SYNC"
	rbJUnlink        rollbackIOPoint = "RB.JUNLINK"
	rbFinalDirSync   rollbackIOPoint = "RB.FINAL_DIRSYNC"
	rbCleanupUnlink  rollbackIOPoint = "RB.CLEANUP_UNLINK"
	rbCleanupDirSync rollbackIOPoint = "RB.CLEANUP_DIRSYNC"
)
