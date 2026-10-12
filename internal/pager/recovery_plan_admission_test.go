package pager

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"
)

// These tests use byte arrays, the descriptor-free helper, and inert source
// ASTs only. They never invoke native recovery or a recovery-enabled open.
func recoveryAdmissionBytes(count uint32, numbers ...uint32) []byte {
	const sector, pageSize = 512, 512
	data := make([]byte, sector+len(numbers)*(pageSize+8))
	copy(data, []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7})
	for offset, value := range map[int]uint32{8: count, 12: 0xfffffff0, 16: 3, 20: sector, 24: pageSize} {
		binary.BigEndian.PutUint32(data[offset:], value)
	}
	for i, number := range numbers {
		offset := sector + i*(pageSize+8)
		binary.BigEndian.PutUint32(data[offset:], number)
		page := data[offset+4 : offset+4+pageSize]
		for j := range page {
			page[j] = byte(uint32(j) + number)
		}
		sum := uint32(0xfffffff0)
		for j := pageSize - 200; j > 0; j -= 200 {
			sum += uint32(page[j])
		}
		binary.BigEndian.PutUint32(data[offset+4+pageSize:], sum)
	}
	return data
}

func TestRecoveryPlanAdmissionInMemory(t *testing.T) {
	decode := func(data []byte) *journal.ReplayPlan {
		t.Helper()
		before := bytes.Clone(data)
		plan, err := journal.ParseReplay(data, journal.ReplayLimits{})
		if err != nil || plan == nil || !bytes.Equal(data, before) {
			t.Fatalf("decode fixture: plan=%v err=%v", plan, err)
		}
		return plan
	}
	finite := decode(recoveryAdmissionBytes(2, 1, 3))
	finiteEmpty := decode(recoveryAdmissionBytes(0))
	sentinelBytes := recoveryAdmissionBytes(0xffffffff, 1, 3)
	sentinel := decode(sentinelBytes)
	headerOnly := decode(sentinelBytes[:512])
	shortened := decode(sentinelBytes[:512+520])
	unknown := *finite
	unknown.CountEvidence = journal.ReplayCountUnknown
	future := *finite
	future.CountEvidence = journal.ReplayCountEvidence(255)
	contradictory := *sentinel
	contradictory.CountEvidence = journal.ReplayCountFiniteHeaders
	finiteLooking := *finite
	finiteLooking.CountEvidence = journal.ReplayCountEOFUnproven
	for _, tc := range []struct {
		name  string
		plan  *journal.ReplayPlan
		allow bool
	}{
		{"finite", finite, true}, {"finite-zero-records", finiteEmpty, true},
		{"sentinel", sentinel, false}, {"header-only-sentinel", headerOnly, false},
		{"whole-record-shortened-sentinel", shortened, false}, {"nil", nil, false},
		{"zero-plan", &journal.ReplayPlan{}, false}, {"unknown-evidence", &unknown, false},
		{"future-evidence", &future, false}, {"finite-label-raw-sentinel", &contradictory, false},
		{"EOF-label-finite-header", &finiteLooking, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before *journal.ReplayPlan
			if tc.plan != nil {
				copyPlan := *tc.plan
				copyPlan.Pages = make([]journal.PageImage, len(tc.plan.Pages))
				for i, page := range tc.plan.Pages {
					copyPlan.Pages[i] = journal.PageImage{Number: page.Number, Data: bytes.Clone(page.Data)}
				}
				if tc.plan.Pages == nil {
					copyPlan.Pages = nil
				}
				before = &copyPlan
			}
			err := admitRecoveryPlan(tc.plan)
			if (err == nil) != tc.allow || !reflect.DeepEqual(before, tc.plan) {
				t.Fatalf("admission or mutation: allow=%v err=%v", tc.allow, err)
			}
			if !tc.allow {
				mapped := recoveryPlanError(err)
				if !errors.Is(err, journal.ErrReplayUnsupported) || !errors.Is(mapped, journal.ErrReplayUnsupported) ||
					!errors.Is(mapped, ErrRecoveryRequired) || !errors.Is(mapped, ErrUnsupported) ||
					errors.Is(mapped, ErrCorrupt) || errors.Is(mapped, ErrLimit) || errors.Is(mapped, ErrCommitUncertain) {
					t.Fatalf("refusal cause/category changed: %v", mapped)
				}
			}
		})
	}
	// A complete main extent does not prove EOF-count image completeness.
	if err := shortened.ValidateExtent(shortened.DatabaseBytes); err != nil {
		t.Fatal(err)
	}
	if err := admitRecoveryPlan(shortened); !errors.Is(err, journal.ErrReplayUnsupported) {
		t.Fatal("extent check granted EOF-count authority", err)
	}
	if !bytes.Equal(sentinelBytes, recoveryAdmissionBytes(0xffffffff, 1, 3)) {
		t.Fatal("admission changed input bytes")
	}
}

// These source laws deliberately bind a small, reviewed syntactic closure.
// The exact pre-header prefix proves the checked gate is on the direct parser
// result and precedes every later extent/header/Sync/write/truncate/unlink step.
// Prefix changes require review; this is not a whole-program effect proof.
func recoveryAdmissionSyntax(node ast.Node) string {
	var out bytes.Buffer
	if err := printer.Fprint(&out, token.NewFileSet(), node); err != nil {
		return "<unprintable>"
	}
	return out.String()
}

func recoveryAdmissionParse(name, source string) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
}

func recoveryAdmissionFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func recoveryAdmissionSourceFiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func checkRecoveryAdmissionSource(files map[string]string) error {
	recovery, err := recoveryAdmissionParse("recovery.go", files["internal/pager/recovery.go"])
	if err != nil {
		return err
	}
	expected, err := recoveryAdmissionParse("prefix.go", "package pager\n"+recoveryAdmissionExpectedPrefix)
	if err != nil {
		return err
	}
	actualFn := recoveryAdmissionFunction(recovery, "RecoverRollback")
	expectedFn := recoveryAdmissionFunction(expected, "RecoverRollback")
	if actualFn == nil || actualFn.Body == nil || expectedFn == nil || len(actualFn.Body.List) < len(expectedFn.Body.List) {
		return fmt.Errorf("missing direct recovery admission prefix")
	}
	if recoveryAdmissionSyntax(actualFn.Type) != recoveryAdmissionSyntax(expectedFn.Type) {
		return fmt.Errorf("recovery signature changed")
	}
	for i, statement := range expectedFn.Body.List {
		if recoveryAdmissionSyntax(actualFn.Body.List[i]) != recoveryAdmissionSyntax(statement) {
			return fmt.Errorf("recovery admission prefix statement %d differs", i)
		}
	}
	helper, err := recoveryAdmissionParse("helper.go", files["internal/pager/recovery_plan_admission.go"])
	if err != nil {
		return err
	}
	expectedHelper, err := recoveryAdmissionParse("expected_helper.go", recoveryAdmissionExpectedHelper)
	if err != nil {
		return err
	}
	if recoveryAdmissionSyntax(helper) != recoveryAdmissionSyntax(expectedHelper) {
		return fmt.Errorf("pure helper closure or fail-closed/raw-header law changed")
	}
	// All references, including aliases and type uses, are accounted for.
	// Only tests may construct additional plans or call the parser elsewhere.
	want := map[string]map[string]int{
		"internal/journal/replay.go":                {"ParseReplay": 1, "ReplayPlan": 4},
		"internal/pager/recovery.go":                {"ParseReplay": 1, "ReplayPlan": 1, "admitRecoveryPlan": 1},
		"internal/pager/recovery_plan_admission.go": {"ReplayPlan": 1, "admitRecoveryPlan": 1},
	}
	got := map[string]map[string]int{}
	for path, source := range files {
		file, err := recoveryAdmissionParse(path, source)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			id, ok := node.(*ast.Ident)
			if !ok || (id.Name != "ParseReplay" && id.Name != "ReplayPlan" && id.Name != "admitRecoveryPlan") {
				return true
			}
			if got[path] == nil {
				got[path] = map[string]int{}
			}
			got[path][id.Name]++
			return true
		})
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("unreviewed parser/plan/gate reference inventory: %v", got)
	}
	return nil
}

func TestRecoveryPlanAdmissionSourceBoundary(t *testing.T) {
	if err := checkRecoveryAdmissionSource(recoveryAdmissionSourceFiles(t)); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryPlanAdmissionSourceMutationCounterexamples(t *testing.T) {
	files := recoveryAdmissionSourceFiles(t)
	if err := checkRecoveryAdmissionSource(files); err != nil {
		t.Fatal(err)
	}
	const recoveryPath = "internal/pager/recovery.go"
	const helperPath = "internal/pager/recovery_plan_admission.go"
	const gate = "\tif err = admitRecoveryPlan(plan); err != nil {\n\t\treturn result, recoveryPlanError(err)\n\t}\n"
	moveGate := func(source, before string) string {
		source = strings.Replace(source, gate, "", 1)
		return strings.Replace(source, before, before+gate, 1)
	}
	for _, tc := range []struct {
		name string
		path string
		edit func(string) string
	}{
		{"omitted-gate", recoveryPath, func(s string) string { return strings.Replace(s, gate, "", 1) }},
		{"inverted-gate", recoveryPath, func(s string) string {
			return strings.Replace(s, "if err = admitRecoveryPlan(plan); err != nil", "if err = admitRecoveryPlan(plan); err == nil", 1)
		}},
		{"discarded-gate-error", recoveryPath, func(s string) string { return strings.Replace(s, gate, "\t_ = admitRecoveryPlan(plan)\n", 1) }},
		{"unclassified-gate-error", recoveryPath, func(s string) string {
			return strings.Replace(s, gate, "\tif err = admitRecoveryPlan(plan); err != nil { return result, err }\n", 1)
		}},
		{"unknown-evidence-accepted", helperPath, func(s string) string {
			return strings.Replace(s, "plan.CountEvidence != journal.ReplayCountFiniteHeaders", "plan.CountEvidence == journal.ReplayCountEOFUnproven", 1)
		}},
		{"raw-sentinel-check-removed", helperPath, func(s string) string { return strings.Replace(s, " || plan.Header.Records == 0xffffffff", "", 1) }},
		{"substituted-plan", recoveryPath, func(s string) string {
			return strings.Replace(s, "admitRecoveryPlan(plan)", "admitRecoveryPlan(&journal.ReplayPlan{})", 1)
		}},
		{"normalized-raw-header", recoveryPath, func(s string) string {
			return strings.Replace(s, gate, "\tplan.Header.Records = uint32(len(plan.Pages))\n"+gate, 1)
		}},
		{"gate-after-first-sync", recoveryPath, func(s string) string {
			return moveGate(s, "\tif err = jf.Sync(); err != nil {\n\t\treturn result, nativeFileError(err)\n\t}\n")
		}},
		{"gate-after-first-write", recoveryPath, func(s string) string {
			return moveGate(s, "\t\tif err = writeFullAt(db, page.Data, int64(page.Number-1)*int64(plan.Header.PageSize)); err != nil {\n\t\t\treturn result, err\n\t\t}\n")
		}},
		{"earlier-truncate", recoveryPath, func(s string) string { return strings.Replace(s, gate, "\t_ = db.Truncate(0)\n"+gate, 1) }},
		{"earlier-unlink", recoveryPath, func(s string) string { return strings.Replace(s, gate, "\t_ = os.Remove(jpath)\n"+gate, 1) }},
		{"duplicate-gate", recoveryPath, func(s string) string { return strings.Replace(s, gate, gate+gate, 1) }},
		{"helper-global-dependency", helperPath, func(s string) string { return s + "\nvar admissionHook = admitRecoveryPlan\n" }},
		{"unguarded-parser-consumer", "internal/pager/unguarded_plan.go", func(string) string {
			return "package pager\nimport \"github.com/ms-xiaosnow/gosqlite/internal/journal\"\nfunc unguardedPlan(b []byte) { _, _ = journal.ParseReplay(b, journal.ReplayLimits{}) }\n"
		}},
		{"aliased-parser-consumer", "internal/pager/aliased_plan.go", func(string) string {
			return "package pager\nimport \"github.com/ms-xiaosnow/gosqlite/internal/journal\"\nvar decodePlan = journal.ParseReplay\n"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutant := make(map[string]string, len(files)+1)
			for path, source := range files {
				mutant[path] = source
			}
			mutant[tc.path] = tc.edit(files[tc.path])
			if mutant[tc.path] == files[tc.path] {
				t.Fatal("source mutation did not apply")
			}
			// Counterexamples must remain syntactically valid inert Go source;
			// a parser error alone must not masquerade as a detected bypass.
			if _, err := recoveryAdmissionParse(tc.path, mutant[tc.path]); err != nil {
				t.Fatal("invalid source mutation", err)
			}
			if err := checkRecoveryAdmissionSource(mutant); err == nil {
				t.Fatal("source admission counterexample escaped")
			}
		})
	}
}

const recoveryAdmissionExpectedHelper = `package pager

import "github.com/ms-xiaosnow/gosqlite/internal/journal"

// admitRecoveryPlan is a descriptor-free refusal gate, not a publication token.
// Finite evidence and a nonsentinel raw header are both required. A successful
// parser result, complete main extent, or checksum cannot authorize EOF-count
// recovery. Existing extent, restored-header, identity and locking checks still
// apply to admitted finite plans. This helper neither mutates nor retains plans.
func admitRecoveryPlan(plan *journal.ReplayPlan) error {
	if plan == nil || plan.CountEvidence != journal.ReplayCountFiniteHeaders || plan.Header.Records == 0xffffffff {
		return journal.ErrReplayUnsupported
	}
	return nil
}
`

const recoveryAdmissionExpectedPrefix = `func RecoverRollback(ctx context.Context, path string, opts RecoveryOptions) (result RecoveryResult, retErr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// Charge the fixed main/temporary ownership slots before path resolution,
	// including metadata probes that can return an error-carried native owner.
	cleanup, err := plainRetirements.reserve()
	if err != nil {
		return result, err
	}
	touched, removed := false, false
	defer func() {
		retErr = finishPlainRecovery(cleanup, retErr, touched, removed)
		if retErr != nil {
			result = RecoveryResult{}
		}
	}()
	path, err = filepath.Abs(path)
	if err != nil {
		return result, err
	}
	path, err = resolveNativePath(path)
	if err != nil {
		return result, err
	}
	db, err := openRecoveryFile(path, os.O_RDWR)
	if db != nil {
		cleanup.main = plainFile{db}
	}
	if err != nil {
		return result, err
	}
	info, err := statNativeFile(db)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("%w: recovery requires a regular database", ErrUnsupported)
	}
	if err = validateWritableFile(db); err != nil {
		return result, err
	}
	if err = lockShared(db); err != nil {
		return result, err
	}
	if err = recoveryCheckPath(path, db, info); err != nil {
		return result, err
	}
	if err = recoveryCheckWAL(path); err != nil {
		return result, err
	}
	jpath := path + "-journal"
	if _, absent, probeErr := nativeLstatForCreate(jpath); absent {
		return result, nil
	} else if probeErr != nil {
		return result, probeErr
	}
	if err = recoveryCheckReserved(db); err != nil {
		return result, err
	}
	// The first-byte check is SQLite's hot-candidate test, not proof that the
	// journal is valid. Cold files are left untouched and fail closed.
	candidate, _, err := recoveryOpenJournal(jpath, cleanup)
	if err != nil {
		return result, err
	}
	var first [1]byte
	_, readErr := readNativeAt(candidate, first[:], 0)
	// Only the direct EOF from this one-byte read is an empty candidate. A
	// wrapped/joined EOF may carry another diagnostic or ownership obligation.
	if readErr == io.EOF {
		readErr = nil
	}
	if err = cleanup.closeTemporary(readErr); err != nil {
		return result, err
	}
	if first[0] == 0 {
		return result, fmt.Errorf("%w: cold or empty journal preserved", ErrRecoveryRequired)
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	// lockExclusive acquires PENDING and the shared-byte range; it does not
	// acquire RESERVED. Closing db releases every acquired OFD lock on error.
	if err = lockExclusive(db); err != nil {
		return result, err
	}
	if err = recoveryCheckReserved(db); err != nil {
		return result, err
	}
	if err = recoveryCheckPath(path, db, info); err != nil {
		return result, err
	}
	if err = recoveryCheckWAL(path); err != nil {
		return result, err
	}
	info, err = statNativeFile(db)
	if err != nil {
		return result, err
	}
	// SQLite can discard journals beside an empty database. We cannot prove
	// which database incarnation such a journal belongs to, so preserve it.
	if info.Size() == 0 {
		return result, fmt.Errorf("%w: journal beside empty database is ambiguous", ErrRecoveryRequired)
	}
	jf, absent, err := recoveryOpenJournal(jpath, cleanup)
	if absent {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	ji, err := statNativeFile(jf)
	if err != nil {
		return result, err
	}
	limits := opts.Limits.WithDefaults()
	if ji.Size() < 0 || uint64(ji.Size()) > limits.MaxJournalBytes || uint64(ji.Size()) > uint64(^uint(0)>>1) {
		return result, fmt.Errorf("%w: journal bytes", ErrLimit)
	}
	data := make([]byte, int(ji.Size()))
	if _, err = io.ReadFull(jf, data); err != nil {
		return result, nativeFileError(err)
	}
	plan, err := journal.ParseReplay(data, limits)
	if err != nil {
		return result, recoveryPlanError(err)
	}
	if err = admitRecoveryPlan(plan); err != nil {
		return result, recoveryPlanError(err)
	}
	if err = plan.ValidateExtent(info.Size()); err != nil {
		return result, recoveryPlanError(err)
	}
	if err = recoveryValidateHeader(db, plan); err != nil {
		return result, err
	}
}
`
