# Selected behavior audit

Source-only recovery cleanup repair against tree 7d3184f87165e11e12560884ec6e4f92c9bfd039.

- `TestPlainRecoveryCleanupMemory*`: only plainMemoryOwner objects, local bounded registries, synthetic errors. No descriptor, path/native acquisition or recovery call.
- `TestPlainRecoveryCleanupSourceAdmissionAndRetirementBoundary`: parses local Go source ASTs, never invokes parsed functions.
- `TestPlainCleanupMemory*`, `TestPlainTxCleanupMemory*`: reviewed fake resource/callback/error graph laws. The database/sql-disposal cases connect to memory-only driver objects. Reader/Tx placeholder vfs.File values have no raw descriptor and are never operated natively. Admission is tested through local registries. No recovery call.
- `TestCleanupOwnersBoundedMemory*`: VFS errors around zero-value Files and terminal memory markers; one-pass, dependency and bound laws. No native descriptor operation.
- `TestPathAbsenceMemory*`: memory callbacks and inert numeric IDs. No native open, missing file, close, or failure simulation.
- `TestReplay.*InMemory`: byte-slice journal parser/geometry/extent tests only; no native fixture or recovery call.
- `TestTxCreatePageSizesAndReadSnapshot`: ordinary create/commit/intact open/read/read-only commit/close; helper createEmpty/openTxTest/fileBytes/noJournal uses ordinary files. The mutated caller-returned page copy is never written; this is aliasing coverage, not corruption.
- `TestPlaintextCleanupOrdinaryReaderRowsAndReopen`: intact normal create/insert/read/close/reopen with ordinary SQL planning error and canceled query; no physical faults.
- `TestPlaintextCleanupOrdinaryDatabaseSQLZeroIdle`: intact normal create/insert/read/zero-idle connection close. No physical faults.

Each selected execution follows the source-reference guard. No TestMain or test init function exists in the baseline. Compile-only go test -run '^$' selects no tests or fuzz seeds; cross compilation uses -exec=true to avoid executing target test binaries. No manual tags, broad executed suites, recovery calls, native recovery fixtures, physical negative workflows, injection hooks, interrupted publication/rekey or remote writes.
