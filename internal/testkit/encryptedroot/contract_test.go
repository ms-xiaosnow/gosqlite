package encryptedroot

import (
	"fmt"
	"strings"
	"testing"
)

// All fixtures below are scalar records. No key, file, SQL, native API, parser,
// subprocess or cryptographic implementation is used by these tests.
func memoryReceipt(name string) Receipt {
	p, _ := Select(name)
	ledger := Ledger{true, true, true, true, true, 1, 512, 2, 3, 3}
	r := Receipt{Profile: Profile, Case: name, Status: "observed", UID: 1000,
		BinarySHA256: strings.Repeat("a", 64), ExpectedBinarySHA256: strings.Repeat("a", 64),
		Root: Directory{1, 1, 1000, 0700}, Run: Directory{1, 2, 1000, 0700},
		Original: Directory{1, 3, 1000, 0700}, Candidate: Directory{1, 4, 1000, 0700},
		ChangedOffset: p.Offset, ChangedXOR: p.XOR, SaltBeforeNonzero: true, SaltAfterNonzero: true,
		Seed: ledger, OpenCalls: 1, Class: p.Class, Code: p.Code, CausePreserved: true,
		SeedAcquires: 1, SeedReleases: 1, CandidateAcquires: p.ProviderCalls, CandidateReleases: p.ProviderCalls,
		Owners: []Owner{{1, 0, "db", true, 1, "closed"}, {2, 1, "rows", true, 1, "eof"}, {3, 0, "db", false, 0, "absent"}}}
	if p.Offset >= 0 {
		r.ChangedBytes = 1
	}
	if name == "control" {
		r.Control, r.QueryCalls, r.DBReturned = ledger, 1, true
		r.Owners[2] = Owner{3, 0, "db", true, 1, "closed"}
		r.Owners = append(r.Owners, Owner{4, 3, "rows", true, 1, "eof"})
	}
	for i := 0; i < FileOwners; i++ {
		r.Owners = append(r.Owners, Owner{ID: len(r.Owners) + 1, Kind: "file", Present: true, Attempts: 1, Terminal: "closed"})
	}
	names := []string{RootName, RootName + ".gse-" + strings.Repeat("1", 64) + ".base", RootName + ".gse-" + strings.Repeat("2", 64) + ".base", RootName + ".gse-" + strings.Repeat("3", 64) + ".base", RootName + ".gse-" + strings.Repeat("4", 64) + ".retained", RootName + ".gse-" + strings.Repeat("5", 64) + ".retained", RootName + ".gse-lock"}
	for i, name := range names {
		size := int64(RootBytes)
		if i == 1 {
			size = 1256
		}
		if i == 2 || i == 3 {
			size = 1808
		}
		if i == 6 {
			size = 0
		}
		f := File{name, size, fmt.Sprintf("%064x", i+1), 1, uint64(i + 10), 1000, 0600, 1}
		r.OriginalBefore = append(r.OriginalBefore, f)
		f.Inode += 100
		if i == 0 && p.Offset >= 0 {
			f.SHA256 = strings.Repeat("b", 64)
		}
		r.CandidateBefore = append(r.CandidateBefore, f)
	}
	r.OriginalAfter = append([]File(nil), r.OriginalBefore...)
	r.CandidateAfter = append([]File(nil), r.CandidateBefore...)
	return r
}

func TestSixRootPlansAndScalarReceipts(t *testing.T) {
	for _, want := range []Plan{
		{"control", -1, 0, false, "ok", 0, 1}, {"wrong-key", -1, 0, true, "authentication", 26, 1},
		{"root-salt", 144, 1, false, "authentication", 26, 1}, {"metadata-ciphertext", 536, 1, false, "authentication", 26, 1},
		{"metadata-tag", 664, 1, false, "authentication", 26, 1}, {"unknown-major", 9, 2, false, "unsupported", 1, 0},
	} {
		if got, err := Select(want.Case); err != nil || got != want {
			t.Fatal("finite plan changed", got, err)
		}
		if err := memoryReceipt(want.Case).Validate(); err != nil {
			t.Fatal(want.Case, err)
		}
	}
	for _, name := range []string{"", "all", "password", "layout2", "repair", "wrong-key-retry"} {
		if _, err := Select(name); err == nil {
			t.Fatal("unbounded admission", name)
		}
	}
}

func TestRootReceiptRejectsFiniteFalsePasses(t *testing.T) {
	changes := map[string]func(*Receipt){
		"unobserved":        func(r *Receipt) { r.Status = "PASS" },
		"binary mismatch":   func(r *Receipt) { r.BinarySHA256 = strings.Repeat("b", 64) },
		"directory alias":   func(r *Receipt) { r.Candidate = r.Original },
		"original altered":  func(r *Receipt) { r.OriginalAfter[0].SHA256 = strings.Repeat("f", 64) },
		"candidate altered": func(r *Receipt) { r.CandidateAfter[0].SHA256 = strings.Repeat("f", 64) },
		"copy hard link": func(r *Receipt) {
			r.CandidateBefore[0].Inode = r.OriginalBefore[0].Inode
			r.CandidateAfter[0] = r.CandidateBefore[0]
		},
		"unrelated file":       func(r *Receipt) { r.OriginalBefore[1].Name = "other"; r.OriginalAfter[1] = r.OriginalBefore[1] },
		"missing member":       func(r *Receipt) { r.OriginalBefore = r.OriginalBefore[:6]; r.OriginalAfter = r.OriginalAfter[:6] },
		"wrong offset":         func(r *Receipt) { r.ChangedOffset++ },
		"multiple bytes":       func(r *Receipt) { r.ChangedBytes = 2 },
		"wrong xor":            func(r *Receipt) { r.ChangedXOR = 128 },
		"zero salt":            func(r *Receipt) { r.SaltAfterNonzero = false },
		"io error":             func(r *Receipt) { r.Class = "other"; r.Code = 10 },
		"lost original cause":  func(r *Receipt) { r.CausePreserved = false },
		"nil error":            func(r *Receipt) { r.Class = "ok"; r.Code = 0 },
		"usable db":            func(r *Receipt) { r.DBReturned = true },
		"negative queried":     func(r *Receipt) { r.QueryCalls = 1 },
		"negative plaintext":   func(r *Receipt) { r.Control = r.Seed },
		"unverified seed":      func(r *Receipt) { r.Seed.Verified = false },
		"wrong seed members":   func(r *Receipt) { r.Seed.ExactRows = false },
		"missing eof":          func(r *Receipt) { r.Seed.EOF = false },
		"missing check":        func(r *Receipt) { r.Seed.Checked = false },
		"open retry":           func(r *Receipt) { r.OpenCalls = 2 },
		"pending cleanup":      func(r *Receipt) { r.PendingCleanup = true },
		"duplicate release":    func(r *Receipt) { r.DuplicateRelease = true },
		"key lease leak":       func(r *Receipt) { r.CandidateReleases = 0 },
		"marker leak":          func(r *Receipt) { r.PlaintextMarker = true },
		"owner retry":          func(r *Receipt) { r.Owners[0].Attempts = 2 },
		"unknown owner":        func(r *Receipt) { r.Owners[0].Terminal = "unknown" },
		"absent owner omitted": func(r *Receipt) { r.Owners = r.Owners[:2] },
		"extra db":             func(r *Receipt) { r.Owners = append(r.Owners, Owner{len(r.Owners) + 1, 0, "db", false, 0, "absent"}) },
		"missing file owner":   func(r *Receipt) { r.Owners = r.Owners[:len(r.Owners)-1] },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			r := memoryReceipt("root-salt")
			change(&r)
			if r.Validate() == nil {
				t.Fatal("false PASS accepted")
			}
		})
	}
	for _, change := range []func(*Receipt){
		func(r *Receipt) { r.Control.ExactRows = false },
		func(r *Receipt) { r.Control.Rows = 2 },
		func(r *Receipt) { r.Control.RowsErrNil = false },
		func(r *Receipt) { r.QueryCalls = 0 },
		func(r *Receipt) { r.Owners[3].Parent = 1 },
	} {
		r := memoryReceipt("control")
		change(&r)
		if r.Validate() == nil {
			t.Fatal("incomplete control accepted")
		}
	}
	r := memoryReceipt("unknown-major")
	r.Class, r.Code = "authentication", 26
	if r.Validate() == nil {
		t.Fatal("unknown major accepted as authentication case")
	}
}

func TestRootOwnerRetiresOnlyOnce(t *testing.T) {
	o := Owner{ID: 1, Kind: "file", Present: true, Terminal: "live"}
	if o.Retire("unknown") != nil || o.Retire("closed") == nil {
		t.Fatal("unknown close was retried")
	}
	o = Owner{ID: 1, Kind: "rows", Present: true, Terminal: "live"}
	if o.Retire("eof") != nil || o.Retire("closed") == nil {
		t.Fatal("EOF retirement was repeated")
	}
	o = Owner{ID: 1, Kind: "db", Present: true, Terminal: "live"}
	if o.Retire("eof") == nil {
		t.Fatal("DB retirement fabricated")
	}
}

func TestRootErrorFactsRejectUnrelatedFailures(t *testing.T) {
	for _, facts := range []ErrorFacts{
		{Code: 10}, {Code: 26}, {Code: 1}, {Code: 1, Authentication: true},
		{Code: 26, Authentication: true, IO: true}, {Code: 26, Authentication: true, Other: true},
		{Code: 1, CryptoUnsupported: true}, {Code: 1, PublicUnsupported: true},
		{Code: 1, CryptoUnsupported: true, PublicUnsupported: true, IO: true},
		{Code: 1, Authentication: true, CryptoUnsupported: true, PublicUnsupported: true},
	} {
		if _, _, preserved := Classify(facts); preserved {
			t.Fatal("unrelated error accepted", facts)
		}
	}
	if class, code, preserved := Classify(ErrorFacts{Code: 26, Authentication: true}); class != "authentication" || code != 26 || !preserved {
		t.Fatal("authentication refused")
	}
	if class, code, preserved := Classify(ErrorFacts{Code: 1, CryptoUnsupported: true, PublicUnsupported: true}); class != "unsupported" || code != 1 || !preserved {
		t.Fatal("unsupported refused")
	}
}
