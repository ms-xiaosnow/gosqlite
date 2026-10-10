# Secondary-index range scoped behavior audit

The new selected SQL tests create intact files only through ordinary CREATE,
INSERT, UPDATE and normal transaction commits/rollbacks. Shared WAL coverage uses
cooperating normal readers/writers, snapshot leases, checkpoint and close/reopen.
Negative cases are scalar/parser/name/bind errors, ordinary cancellation and
memory/work admission. No test introduces physical corruption, tamper, wrong
keys, missing files, namespace mutation, crash/kill/process exit, hot recovery,
native lock/unmap/close/Sync/short-write/disk-full/permission failures, interrupted
publication or rekey. Test-only official CLI processes use ordinary SQL and normal
EOF/completion. Source-reference guard must run before each selected suite,
race/oracle command and build/vet; it is not a replacement for this audit.

Historical default-tag tests retained verbatim are not all selected by this
component runner. Manual physical-risk build tags are never enabled. A full
integrated gate remains the lead's separate responsibility.
