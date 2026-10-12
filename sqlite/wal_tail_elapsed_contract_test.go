package sqlite_test

import "testing"

// SOURCE_ONLY / NOT_RUN. Scalar evidence only: no clock, files, native owners,
// callbacks, public constructors, cleanup operations or performance assertions.
const manualWALElapsedRevision = "wal_tail_publication_elapsed_v1"
const manualWALElapsedBoundary = "public_call_entry_to_return_including_scan_lock_publication_and_constructor_retirement"
const manualWALElapsedClock = "time.Now/time.Since_monotonic_same_process"
const manualWALElapsedCache = "fixture_freshly_written_and_verified_warm_or_unspecified_OS_cache_no_eviction"
const manualWALElapsedFlags = "CGO_ENABLED=0 GOFLAGS=-buildvcs=false tags=manual_wal_publication"

// Source, binary, hardware and CPU are bounded external declarations. The
// enclosing retained evidence must independently bind them to actual bytes.
type manualWALElapsedIdentity struct {
	HarnessRevision             string `json:"harness_revision"`
	SourceManifestSHA256        string `json:"source_manifest_sha256_declared"`
	BinarySHA256                string `json:"binary_sha256_declared"`
	QualificationEvidenceSHA256 string `json:"qualification_evidence_sha256_declared"`
	ProfileSHA256               string `json:"profile_sha256_observed"`
	Toolchain                   string `json:"toolchain_observed"`
	BuildFlags                  string `json:"build_flags_declared"`
	Hardware                    string `json:"hardware_declared"`
	CPU                         string `json:"cpu_declared"`
	OS                          string `json:"os_observed"`
	Architecture                string `json:"architecture_observed"`
	Filesystem                  string `json:"filesystem_observed"`
	SectorBytes                 int    `json:"sector_bytes_declared"`
	Cache                       string `json:"cache_condition"`
}

type manualWALElapsedFixture struct {
	ID                string `json:"fixture_id"`
	DatasetSHA256     string `json:"dataset_manifest_sha256"`
	MainSHA256        string `json:"main_sha256"`
	WALSHA256         string `json:"wal_sha256"`
	PublishedSHA256   string `json:"published_expected_sha256"`
	LedgerSHA256      string `json:"ledger_sha256"`
	Seed              string `json:"seed"`
	InputProvenance   string `json:"input_provenance"`
	PreCrashCommitAck string `json:"pre_crash_commit_ack"`
	IndexScope        string `json:"index_scope"`
	ChecksumOrder     string `json:"wal_checksum_order"`
	PageBytes         int    `json:"page_bytes"`
	MainPages         int    `json:"main_pages"`
	MainBytes         int    `json:"main_bytes"`
	WALBytes          int    `json:"wal_bytes"`
	PublishedBytes    int    `json:"published_expected_bytes"`
	CommittedFrames   int    `json:"committed_frames"`
	UncommittedFrames int    `json:"uncommitted_frames"`
	PublishedFrames   int    `json:"published_frames"`
}

type manualWALElapsedAttempt struct {
	ID                  string                   `json:"attempt_id"`
	Case                string                   `json:"case"`
	Entrance            string                   `json:"entrance"`
	Kind                string                   `json:"kind"`
	Identity            manualWALElapsedIdentity `json:"identity"`
	Fixture             manualWALElapsedFixture  `json:"fixture"`
	Attempted           bool                     `json:"attempted"`
	Returned            bool                     `json:"returned"`
	HandleReturned      bool                     `json:"handle_returned"`
	Outcome             string                   `json:"outcome"`
	Diagnostic          string                   `json:"diagnostic"`
	DiagnosticTruncated bool                     `json:"diagnostic_truncated"`
	ChecksComplete      bool                     `json:"checks_complete"`
	StartedUTC          string                   `json:"started_utc_correlation_only"`
	FinishedUTC         string                   `json:"finished_utc_correlation_only"`
	TimerStopped        bool                     `json:"timer_stopped"`
	Clock               string                   `json:"duration_clock"`
	Unit                string                   `json:"duration_unit"`
	Boundary            string                   `json:"timing_boundary"`
	PublicElapsedNS     int64                    `json:"public_open_with_tail_publication_elapsed_ns"`
	RefusalElapsedNS    int64                    `json:"refusal_attempt_elapsed_ns"`
	Qualified           bool                     `json:"qualified"`
	Samples             int                      `json:"qualified_sample_count"`
}

type manualWALElapsedCase struct {
	Name            string `json:"name"`
	Outcome         string `json:"outcome"`
	ChecksComplete  bool   `json:"checks_complete"`
	InputsUnchanged bool   `json:"inputs_unchanged"`
	OwnersRetired   bool   `json:"rows_database_observation_owners_retired"`
	StartedUTC      string `json:"started_utc_correlation_only"`
	FinishedUTC     string `json:"finished_utc_correlation_only"`
	ElapsedNS       int64  `json:"case_end_to_end_elapsed_ns"`
	TimerStopped    bool   `json:"timer_stopped"`
}

type manualWALElapsedRetirement struct {
	Owned     int `json:"owned"`
	Attempted int `json:"close_attempted_once"`
	Retired   int `json:"retired_without_diagnostic"`
}

type manualWALElapsedReport struct {
	Version                  int                           `json:"version"`
	Status                   string                        `json:"status"`
	Identity                 manualWALElapsedIdentity      `json:"identity"`
	Fixture                  manualWALElapsedFixture       `json:"fixture"`
	Attempts                 []manualWALElapsedAttempt     `json:"attempts"`
	Cases                    []manualWALElapsedCase        `json:"cases"`
	Retirement               [4]manualWALElapsedRetirement `json:"retirement_rows_database_observation_directories"`
	FailureLatched           bool                          `json:"failure_latched"`
	CloseDiagnostics         int                           `json:"close_diagnostics"`
	PendingOwners            int                           `json:"pending_owners_seen"`
	RootRetirementComplete   bool                          `json:"root_retirement_complete"`
	AllCasesComplete         bool                          `json:"all_four_cases_complete"`
	FinalReportComplete      bool                          `json:"final_report_complete"`
	RawEncodingComplete      bool                          `json:"raw_attempt_encoding_complete"`
	ReportEncodingChecked    bool                          `json:"report_encoding_checked"`
	StartedUTC               string                        `json:"started_utc_correlation_only"`
	FinishedUTC              string                        `json:"finished_utc_correlation_only"`
	InvocationElapsedNS      int64                         `json:"invocation_end_to_end_elapsed_ns"`
	Clock                    string                        `json:"duration_clock"`
	TimerStopped             bool                          `json:"timer_stopped"`
	ExternalTerminalRequired bool                          `json:"external_natural_exit_full_log_and_exact_PASS_required"`
}

func manualWALElapsedCases() []manualWALElapsedCase {
	return []manualWALElapsedCase{
		{Name: "nil-default-preserves-input", Outcome: "NOT_RUN"},
		{Name: "open-publishes-writes-reopens", Outcome: "NOT_RUN"},
		{Name: "open-context-publishes-writes-reopens", Outcome: "NOT_RUN"},
		{Name: "connector-publishes-writes-reopens", Outcome: "NOT_RUN"},
	}
}

func manualWALElapsedAttempts() []manualWALElapsedAttempt {
	cases := manualWALElapsedCases()
	var attempts []manualWALElapsedAttempt
	for _, entrance := range []string{"Open", "OpenContext", "Connector"} {
		attempts = append(attempts, manualWALElapsedAttempt{ID: "refusal/" + entrance, Case: cases[0].Name, Entrance: entrance, Kind: "refusal", Outcome: "NOT_RUN"})
	}
	for i, entrance := range []string{"Open", "OpenContext", "Connector"} {
		attempts = append(attempts, manualWALElapsedAttempt{ID: "publication/" + entrance, Case: cases[i+1].Name, Entrance: entrance, Kind: "publication", Outcome: "NOT_RUN"})
	}
	return attempts
}

func manualWALElapsedSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func manualWALElapsedLabel(value string) bool {
	if len(value) == 0 || len(value) > 256 {
		return false
	}
	for _, c := range value {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}

func manualWALElapsedIdentityValid(v manualWALElapsedIdentity) bool {
	return v.HarnessRevision == manualWALElapsedRevision && manualWALElapsedSHA(v.SourceManifestSHA256) &&
		manualWALElapsedSHA(v.BinarySHA256) && manualWALElapsedSHA(v.QualificationEvidenceSHA256) && manualWALElapsedSHA(v.ProfileSHA256) &&
		v.Toolchain == "go1.27.2" && v.BuildFlags == manualWALElapsedFlags && manualWALElapsedLabel(v.Hardware) && manualWALElapsedLabel(v.CPU) &&
		v.OS == "linux" && manualWALElapsedLabel(v.Architecture) && (v.Filesystem == "ext2" || v.Filesystem == "ext3" || v.Filesystem == "ext4") &&
		v.SectorBytes == 4096 && v.Cache == manualWALElapsedCache
}

func manualWALElapsedFixtureValid(v manualWALElapsedFixture) bool {
	return v.ID == "two_page_complete_uncommitted_tail_v1" && manualWALElapsedSHA(v.DatasetSHA256) && manualWALElapsedSHA(v.MainSHA256) &&
		manualWALElapsedSHA(v.WALSHA256) && manualWALElapsedSHA(v.PublishedSHA256) && manualWALElapsedSHA(v.LedgerSHA256) &&
		v.Seed == "not_applicable_deterministic_fixture_v1" && v.InputProvenance == "synthetic_encoded_committed_prefix" && v.PreCrashCommitAck == "not_present" &&
		v.IndexScope == "table_rowid_IPK_only_no_secondary_or_extension_indexes" && v.ChecksumOrder == "little_endian" &&
		v.PageBytes == 4096 && v.MainPages == 2 && v.MainBytes == 8192 && v.WALBytes == 12392 && v.PublishedBytes == 12392 &&
		v.CommittedFrames == 2 && v.UncommittedFrames == 1 && v.PublishedFrames == 3
}

// No partial success: first erase supplied qualification, then admit all three
// distinct publication samples only after the complete invocation has retired.
// This is an in-process candidate, never a substitute for complete outer logs,
// natural exit=0, exact top-level/four-subcase PASS and retained evidence binding.
func qualifyManualWALElapsed(in manualWALElapsedReport) manualWALElapsedReport {
	out := in
	out.Attempts = append([]manualWALElapsedAttempt(nil), in.Attempts...)
	out.Status = "UNKNOWN"
	for i := range out.Attempts {
		out.Attempts[i].Qualified = false
		out.Attempts[i].Samples = 0
	}
	if in.Version != 1 || in.FailureLatched || in.CloseDiagnostics != 0 || in.PendingOwners != 0 || !in.RootRetirementComplete || !in.AllCasesComplete ||
		!in.FinalReportComplete || !in.RawEncodingComplete || !in.ReportEncodingChecked || !in.ExternalTerminalRequired ||
		!in.TimerStopped || in.InvocationElapsedNS <= 0 || in.Clock != manualWALElapsedClock ||
		!manualWALElapsedLabel(in.StartedUTC) || !manualWALElapsedLabel(in.FinishedUTC) ||
		!manualWALElapsedIdentityValid(in.Identity) || !manualWALElapsedFixtureValid(in.Fixture) {
		return out
	}
	for _, owner := range in.Retirement {
		if owner.Owned <= 0 || owner.Attempted != owner.Owned || owner.Retired != owner.Owned {
			return out
		}
	}
	cases := manualWALElapsedCases()
	if len(in.Cases) != len(cases) {
		return out
	}
	remainingInvocationNS := in.InvocationElapsedNS
	for i, c := range in.Cases {
		if c.Name != cases[i].Name || c.Outcome != "VERIFIED" || !c.ChecksComplete || !c.InputsUnchanged || !c.OwnersRetired ||
			!c.TimerStopped || c.ElapsedNS <= 0 || c.ElapsedNS > in.InvocationElapsedNS ||
			!manualWALElapsedLabel(c.StartedUTC) || !manualWALElapsedLabel(c.FinishedUTC) {
			return out
		}
		// Cases are sequential. Compare before subtraction; never sum int64 durations.
		if c.ElapsedNS > remainingInvocationNS {
			return out
		}
		remainingInvocationNS -= c.ElapsedNS
	}
	expected := manualWALElapsedAttempts()
	if len(in.Attempts) != len(expected) {
		return out
	}
	remainingRefusalNS := in.Cases[0].ElapsedNS
	for i, a := range in.Attempts {
		e := expected[i]
		if a.ID != e.ID || a.Case != e.Case || a.Entrance != e.Entrance || a.Kind != e.Kind || a.Identity != in.Identity || a.Fixture != in.Fixture ||
			!a.Attempted || !a.Returned || !a.ChecksComplete || !a.TimerStopped || a.Clock != manualWALElapsedClock || a.Unit != "ns" ||
			a.Boundary != manualWALElapsedBoundary || !manualWALElapsedLabel(a.StartedUTC) || !manualWALElapsedLabel(a.FinishedUTC) ||
			len(a.Diagnostic) > 1024 || a.DiagnosticTruncated {
			return out
		}
		if a.Kind == "refusal" {
			if a.Outcome != "EXPECTED_REFUSAL" || a.HandleReturned || a.Diagnostic == "" || a.PublicElapsedNS != 0 ||
				a.RefusalElapsedNS <= 0 || a.RefusalElapsedNS > in.Cases[0].ElapsedNS {
				return out
			}
			// The three controls are sequential inside the first case.
			if a.RefusalElapsedNS > remainingRefusalNS {
				return out
			}
			remainingRefusalNS -= a.RefusalElapsedNS
		} else {
			if a.Outcome != "RETURNED_OK" || !a.HandleReturned || a.Diagnostic != "" || a.RefusalElapsedNS != 0 ||
				a.PublicElapsedNS <= 0 || a.PublicElapsedNS > in.Cases[i-2].ElapsedNS {
				return out
			}
		}
	}
	for i := 3; i < 6; i++ {
		out.Attempts[i].Qualified = true
		out.Attempts[i].Samples = 1
	}
	out.Status = "QUALIFIED_CANDIDATE_EXTERNAL_TERMINAL_REQUIRED"
	return out
}

func manualWALElapsedMemoryComplete() manualWALElapsedReport {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	identity := manualWALElapsedIdentity{HarnessRevision: manualWALElapsedRevision, SourceManifestSHA256: digest, BinarySHA256: digest,
		QualificationEvidenceSHA256: digest, ProfileSHA256: digest, Toolchain: "go1.27.2", BuildFlags: manualWALElapsedFlags,
		Hardware: "declared test hardware", CPU: "declared test CPU", OS: "linux", Architecture: "amd64", Filesystem: "ext4", SectorBytes: 4096, Cache: manualWALElapsedCache}
	fixture := manualWALElapsedFixture{ID: "two_page_complete_uncommitted_tail_v1", DatasetSHA256: digest, MainSHA256: digest, WALSHA256: digest,
		PublishedSHA256: digest, LedgerSHA256: digest, Seed: "not_applicable_deterministic_fixture_v1", InputProvenance: "synthetic_encoded_committed_prefix",
		PreCrashCommitAck: "not_present", IndexScope: "table_rowid_IPK_only_no_secondary_or_extension_indexes", ChecksumOrder: "little_endian",
		PageBytes: 4096, MainPages: 2, MainBytes: 8192, WALBytes: 12392, PublishedBytes: 12392, CommittedFrames: 2, UncommittedFrames: 1, PublishedFrames: 3}
	r := manualWALElapsedReport{Version: 1, Identity: identity, Fixture: fixture, Attempts: manualWALElapsedAttempts(), Cases: manualWALElapsedCases(),
		RootRetirementComplete: true, AllCasesComplete: true, FinalReportComplete: true, RawEncodingComplete: true, ReportEncodingChecked: true,
		StartedUTC: "2026-10-12T00:00:00Z", FinishedUTC: "2026-10-12T00:00:01Z", InvocationElapsedNS: 1000,
		Clock: manualWALElapsedClock, TimerStopped: true, ExternalTerminalRequired: true}
	for i := range r.Retirement {
		r.Retirement[i] = manualWALElapsedRetirement{Owned: 1, Attempted: 1, Retired: 1}
	}
	for i := range r.Cases {
		c := &r.Cases[i]
		c.Outcome, c.ChecksComplete, c.InputsUnchanged, c.OwnersRetired = "VERIFIED", true, true, true
		c.StartedUTC, c.FinishedUTC, c.ElapsedNS, c.TimerStopped = r.StartedUTC, r.FinishedUTC, 100, true
	}
	for i := range r.Attempts {
		a := &r.Attempts[i]
		a.Identity, a.Fixture, a.Attempted, a.Returned, a.ChecksComplete = identity, fixture, true, true, true
		a.StartedUTC, a.FinishedUTC, a.TimerStopped, a.Clock, a.Unit, a.Boundary = r.StartedUTC, r.FinishedUTC, true, manualWALElapsedClock, "ns", manualWALElapsedBoundary
		if i < 3 {
			a.Outcome, a.Diagnostic, a.RefusalElapsedNS = "EXPECTED_REFUSAL", "expected unsupported tail", 10
		} else {
			a.Outcome, a.HandleReturned, a.PublicElapsedNS = "RETURNED_OK", true, 20
		}
	}
	return r
}

func manualWALElapsedMemoryRejected(t *testing.T, r manualWALElapsedReport) {
	t.Helper()
	got := qualifyManualWALElapsed(r)
	if got.Status != "UNKNOWN" {
		t.Fatalf("incomplete evidence admitted: %s", got.Status)
	}
	for _, a := range got.Attempts {
		if a.Qualified || a.Samples != 0 {
			t.Fatal("rejection retained a successful sample")
		}
	}
}

func TestWALPublicationElapsedMemoryQualifiedSample(t *testing.T) {
	r := manualWALElapsedMemoryComplete()
	got := qualifyManualWALElapsed(r)
	if got.Status != "QUALIFIED_CANDIDATE_EXTERNAL_TERMINAL_REQUIRED" {
		t.Fatal(got.Status)
	}
	for i, a := range got.Attempts {
		if a.Qualified != (i >= 3) || (i >= 3 && a.Samples != 1) || (i < 3 && a.Samples != 0) {
			t.Fatalf("slot %d admitted incorrectly", i)
		}
		if r.Attempts[i].Qualified {
			t.Fatal("reducer mutated original raw attempt")
		}
	}
}

func TestWALPublicationElapsedMemoryRejectsUnexecutedOrPartial(t *testing.T) {
	for _, mutate := range []func(*manualWALElapsedReport){
		func(r *manualWALElapsedReport) { r.Attempts[3].Attempted = false },
		func(r *manualWALElapsedReport) { r.Attempts[3].Returned = false },
		func(r *manualWALElapsedReport) { r.Attempts = nil },
		func(r *manualWALElapsedReport) { r.Cases = r.Cases[:3] },
		func(r *manualWALElapsedReport) { r.AllCasesComplete = false },
		func(r *manualWALElapsedReport) { r.FinalReportComplete = false },
		func(r *manualWALElapsedReport) { r.RawEncodingComplete = false },
		func(r *manualWALElapsedReport) { r.ReportEncodingChecked = false },
		func(r *manualWALElapsedReport) { r.Cases[2].ChecksComplete = false },
		func(r *manualWALElapsedReport) { r.Cases[1].InputsUnchanged = false },
		func(r *manualWALElapsedReport) { r.Attempts[4].ChecksComplete = false },
		func(r *manualWALElapsedReport) { r.ExternalTerminalRequired = false },
	} {
		r := manualWALElapsedMemoryComplete()
		mutate(&r)
		manualWALElapsedMemoryRejected(t, r)
	}
	manualWALElapsedMemoryRejected(t, manualWALElapsedReport{})
}

func TestWALPublicationElapsedMemoryRejectsFailureAndUnknown(t *testing.T) {
	for _, outcome := range []string{"NOT_RUN", "EXPECTED_REFUSAL", "UNSUPPORTED", "BUSY", "CANCELLED", "ERROR", "UNKNOWN"} {
		r := manualWALElapsedMemoryComplete()
		r.Attempts[3].Outcome = outcome
		manualWALElapsedMemoryRejected(t, r)
	}
	for _, mutate := range []func(*manualWALElapsedReport){
		func(r *manualWALElapsedReport) { r.Attempts[3].Diagnostic = "error plus handle" },
		func(r *manualWALElapsedReport) { r.Attempts[3].HandleReturned = false },
		func(r *manualWALElapsedReport) { r.Attempts[0].HandleReturned = true },
		func(r *manualWALElapsedReport) { r.Attempts[0].Outcome = "RETURNED_OK" },
		func(r *manualWALElapsedReport) {
			r.FailureLatched = true
			r.Attempts[3].Qualified = true
			r.Attempts[3].Samples = 1
		},
		func(r *manualWALElapsedReport) { r.Attempts[0].DiagnosticTruncated = true },
	} {
		r := manualWALElapsedMemoryComplete()
		mutate(&r)
		manualWALElapsedMemoryRejected(t, r)
	}
}

func TestWALPublicationElapsedMemoryRequiresRetirement(t *testing.T) {
	for i := 0; i < 4; i++ {
		r := manualWALElapsedMemoryComplete()
		r.Retirement[i].Retired = 0
		manualWALElapsedMemoryRejected(t, r)
		r = manualWALElapsedMemoryComplete()
		r.Retirement[i].Attempted = 2
		manualWALElapsedMemoryRejected(t, r)
	}
	for _, mutate := range []func(*manualWALElapsedReport){
		func(r *manualWALElapsedReport) { r.RootRetirementComplete = false },
		func(r *manualWALElapsedReport) { r.Cases[3].OwnersRetired = false },
		func(r *manualWALElapsedReport) { r.PendingOwners = 1 },
		// A later nil close and complete counts never erase the first diagnostic.
		func(r *manualWALElapsedReport) { r.CloseDiagnostics = 1; r.RootRetirementComplete = true },
	} {
		r := manualWALElapsedMemoryComplete()
		mutate(&r)
		manualWALElapsedMemoryRejected(t, r)
	}
}

func TestWALPublicationElapsedMemoryKeepsEntrancesSeparate(t *testing.T) {
	for _, mutate := range []func(*manualWALElapsedReport){
		func(r *manualWALElapsedReport) { r.Attempts[4].Entrance = "Open" },
		func(r *manualWALElapsedReport) { r.Attempts[4].ID = r.Attempts[3].ID },
		func(r *manualWALElapsedReport) { r.Attempts[3].Kind = "ordinary_reopen" },
		func(r *manualWALElapsedReport) { r.Attempts = r.Attempts[:5] },
		func(r *manualWALElapsedReport) { r.Attempts = append(r.Attempts, r.Attempts[3]) },
		func(r *manualWALElapsedReport) {
			r.Attempts[3].Identity.SourceManifestSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		func(r *manualWALElapsedReport) {
			r.Attempts[4].Identity.ProfileSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		func(r *manualWALElapsedReport) {
			r.Attempts[5].Fixture.MainSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		func(r *manualWALElapsedReport) { r.Identity.SourceManifestSHA256 = "unbound" },
		func(r *manualWALElapsedReport) { r.Fixture.PreCrashCommitAck = "claimed_real_crash" },
	} {
		r := manualWALElapsedMemoryComplete()
		mutate(&r)
		manualWALElapsedMemoryRejected(t, r)
	}
	got := qualifyManualWALElapsed(manualWALElapsedMemoryComplete())
	for _, a := range got.Attempts {
		if a.Kind == "publication" && a.Samples != 1 || a.Kind == "refusal" && a.Samples != 0 {
			t.Fatal("different entrances or refusal pooled")
		}
	}
}

func TestWALPublicationElapsedMemoryRejectsInvalidTiming(t *testing.T) {
	for _, mutate := range []func(*manualWALElapsedReport){
		func(r *manualWALElapsedReport) { r.InvocationElapsedNS = 100 },
		func(r *manualWALElapsedReport) {
			for i := 0; i < 3; i++ {
				r.Attempts[i].RefusalElapsedNS = 60
			}
		},
		func(r *manualWALElapsedReport) { r.Attempts[3].PublicElapsedNS = -1 },
		func(r *manualWALElapsedReport) { r.Attempts[3].PublicElapsedNS = 0 },
		func(r *manualWALElapsedReport) { r.Attempts[3].PublicElapsedNS = 101 },
		func(r *manualWALElapsedReport) { r.Attempts[0].RefusalElapsedNS = 0 },
		func(r *manualWALElapsedReport) { r.Attempts[3].TimerStopped = false },
		func(r *manualWALElapsedReport) { r.Attempts[3].Unit = "ms" },
		func(r *manualWALElapsedReport) { r.Attempts[3].Boundary = "recovery_only" },
		func(r *manualWALElapsedReport) { r.Attempts[3].Clock = "UTC_subtraction" },
		func(r *manualWALElapsedReport) { r.Cases[1].ElapsedNS = 0 },
		func(r *manualWALElapsedReport) { r.Cases[1].TimerStopped = false },
		func(r *manualWALElapsedReport) { r.InvocationElapsedNS = 0 },
		func(r *manualWALElapsedReport) { r.Clock = "UTC_subtraction" },
		func(r *manualWALElapsedReport) { r.TimerStopped = false },
	} {
		r := manualWALElapsedMemoryComplete()
		mutate(&r)
		manualWALElapsedMemoryRejected(t, r)
	}
}
