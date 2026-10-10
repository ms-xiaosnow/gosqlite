# Lead independent review of the Reader create-absence boundary

2026-10-10 UTC. Decision: **V4 is not ready for integration**. Its memory laws
pass, but the additional native Windows source boundary below remains a blocker.
No native failure or namespace manipulation was executed.

The separate independent V2 review found and corrected public error-code
precedence. V3 then added an `errors.Is(NotExist) && !errors.Is(ErrPoisoned)`
create guard. Lead source review found that mixed NotExist/ErrChanged and
NotExist/consumed-cleanup diagnostics still authorized creating a database.
The independent descriptor-free test retained here reproduces four false
positives on immutable V3. The same unchanged test passes on immutable V4,
together with existing Reader/public/VFS memory laws and intact ordinary creation.
Both normal and race selections passed, exit0 at19:30:02Z. Commands, raw negative
control and results are retained; they are scoped checks, not a full suite.

V4 uses a private initial-absence marker and bounded single-cause traversal,
rejecting mixed diagnostics, post-acquisition missing main-file errors, pending
owners, duplicate markers and cyclic/overbound graphs. Both immutable V4 and the
independent copy matched all1016 owner-manifest files; the copy adds only the
listed test probe. Manifest SHA256:
c6b054ec84a165336154404ddbb8368c191f5155c3a115bf9dc20b23489d8190.
V3-to-V4 patch SHA256:
74390ea4df48ee1989d1a930780aa00fdcc6857e0a4d6fa0c01ec151335f6a68.

A later source inspection found that the initial Windows VFS resolver itself
opens a metadata handle before calling winSnapshot, winFinalPath and winLocalNTFS.
A sole NotExist error from those later operations can pass through observeMetadata
after successful cleanup. V4's marker on *any* resolveNativePath NotExist is thus
broader than the no-handle initial acquisition boundary. The owner confirmed the
finding. V5 must carry provenance from the initial VFS open failure itself and
must leave post-acquisition inspection errors unmarked even after retirement.
The default resolver requires its own explicit no-handle classification. Existing
callers and ordinary wrapped/joined error identities must remain compatible.

V4 and Tx-on-V4 evidence stay immutable and superseded only for this new boundary.
Native Windows runtime/failures, physical interruption/recovery, independent
security/legal review and complete milestone qualification remain NOT_RUN.
