// Package encryptedroot is a descriptor-free finite receipt model. It neither
// opens files nor authenticates ciphertext. Receipt acceptance is not execution
// authorization, physical qualification, or a durability result.
package encryptedroot

import (
	"fmt"
	"reflect"
	"strings"
)

const Profile = "layout1-root-negative-v1"
const RootName = "fixture.gse"
const RootBytes = 680
const FamilyFiles = 7
const MaxFileBytes = 8192
const MaxOwners = 128
const FileOwners = 40

// Plan fixes encoded byte locations, not a generic mutation registry. RootSalt
// remains nonzero; the one-byte unknown major changes big-endian 1 to 3.
type Plan struct {
	Case          string
	Offset        int
	XOR           byte
	WrongKey      bool
	Class         string
	Code          int
	ProviderCalls int
}

func Select(name string) (Plan, error) {
	switch name {
	case "control":
		return Plan{name, -1, 0, false, "ok", 0, 1}, nil
	case "wrong-key":
		return Plan{name, -1, 0, true, "authentication", 26, 1}, nil
	case "root-salt":
		return Plan{name, 144, 1, false, "authentication", 26, 1}, nil
	case "metadata-ciphertext":
		return Plan{name, 536, 1, false, "authentication", 26, 1}, nil
	case "metadata-tag":
		return Plan{name, 664, 1, false, "authentication", 26, 1}, nil
	case "unknown-major":
		return Plan{name, 9, 2, false, "unsupported", 1, 0}, nil
	}
	return Plan{}, fmt.Errorf("unadmitted case")
}

// ErrorFacts are observations of the original public Open error, not an error
// replacement. Unknown/generic errors and native I/O never satisfy a case.
type ErrorFacts struct {
	Code                                                            int
	Authentication, CryptoUnsupported, PublicUnsupported, IO, Other bool
}

func Classify(f ErrorFacts) (string, int, bool) {
	if f.IO || f.Other {
		return "other", f.Code, false
	}
	if f.Authentication && !f.CryptoUnsupported && !f.PublicUnsupported && f.Code == 26 {
		return "authentication", f.Code, true
	}
	if !f.Authentication && f.CryptoUnsupported && f.PublicUnsupported && f.Code == 1 {
		return "unsupported", f.Code, true
	}
	return "other", f.Code, false
}

func Hash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return s != strings.Repeat("0", 64)
}

type File struct {
	Name          string
	Size          int64
	SHA256        string
	Device, Inode uint64
	UID, Mode     uint32
	Links         uint64
}

type Directory struct {
	Device, Inode uint64
	UID, Mode     uint32
}

type Owner struct {
	ID, Parent int
	Kind       string
	Present    bool
	Attempts   int
	Terminal   string // absent, live, closed, eof, unknown, blocked
}

// Retire records exactly one attempt. Unknown retirement is never retried.
func (o *Owner) Retire(terminal string) error {
	if !o.Present || o.Attempts != 0 || o.Terminal != "live" ||
		(terminal != "closed" && terminal != "eof" && terminal != "unknown") ||
		(terminal == "eof" && o.Kind != "rows") {
		return fmt.Errorf("invalid retirement")
	}
	o.Attempts = 1
	o.Terminal = terminal
	return nil
}

type Ledger struct {
	Verified, Checked, ExactRows, EOF, RowsErrNil bool
	Layout                                        uint16
	PageSize, Pages                               uint32
	Sequence                                      uint64
	Rows                                          int
}

func (l Ledger) valid() bool {
	return l.Verified && l.Checked && l.ExactRows && l.EOF && l.RowsErrNil &&
		l.Layout == 1 && l.PageSize == 512 && l.Pages == 2 && l.Sequence == 3 && l.Rows == 3
}

type Receipt struct {
	Profile, Case, Status string
	// The binary hash is observed and compared with the separately approved
	// build artifact. No field pretends to prove source-to-binary identity.
	BinarySHA256, ExpectedBinarySHA256                               string
	Root, Run, Original, Candidate                                   Directory
	UID                                                              uint32
	OriginalBefore, OriginalAfter, CandidateBefore, CandidateAfter   []File
	ChangedBytes, ChangedOffset                                      int
	ChangedXOR                                                       byte
	SaltBeforeNonzero, SaltAfterNonzero                              bool
	Seed                                                             Ledger
	Control                                                          Ledger
	OpenCalls, QueryCalls                                            int
	DBReturned                                                       bool
	Class                                                            string
	Code                                                             int
	CausePreserved                                                   bool
	SeedAcquires, SeedReleases, CandidateAcquires, CandidateReleases int
	DuplicateRelease, PendingCleanup, PlaintextMarker                bool
	Owners                                                           []Owner
}

func validFiles(files []File, uid uint32) bool {
	if len(files) != FamilyFiles {
		return false
	}
	roots, locks, bases, retained, onePage, twoPages := 0, 0, 0, 0, 0, 0
	for i, f := range files {
		if f.Inode == 0 || f.UID != uid || f.Mode != 0600 || f.Links != 1 || !Hash(f.SHA256) || f.Size < 0 || f.Size > MaxFileBytes {
			return false
		}
		if i > 0 && files[i-1].Name >= f.Name {
			return false
		}
		for j := 0; j < i; j++ {
			if files[j].Device == f.Device && files[j].Inode == f.Inode {
				return false
			}
		}
		switch f.Name {
		case RootName:
			roots++
			if f.Size != RootBytes {
				return false
			}
		case RootName + ".gse-lock":
			locks++
			if f.Size != 0 {
				return false
			}
		default:
			if !strings.HasPrefix(f.Name, RootName+".gse-") {
				return false
			}
			tail := strings.TrimPrefix(f.Name, RootName+".gse-")
			switch {
			case strings.HasSuffix(tail, ".base"):
				bases++
				if !Hash(strings.TrimSuffix(tail, ".base")) || (f.Size != 1256 && f.Size != 1808) {
					return false
				}
				if f.Size == 1256 {
					onePage++
				} else {
					twoPages++
				}
			case strings.HasSuffix(tail, ".retained"):
				retained++
				if !Hash(strings.TrimSuffix(tail, ".retained")) || f.Size != RootBytes {
					return false
				}
			default:
				return false
			}
		}
	}
	return roots == 1 && locks == 1 && bases == 3 && retained == 2 && onePage == 1 && twoPages == 2
}

func (r Receipt) Validate() error {
	p, err := Select(r.Case)
	if err != nil {
		return err
	}
	if r.Profile != Profile || r.Status != "observed" || !Hash(r.BinarySHA256) || r.BinarySHA256 != r.ExpectedBinarySHA256 || r.UID == 0 {
		return fmt.Errorf("missing provenance")
	}
	dirs := []Directory{r.Root, r.Run, r.Original, r.Candidate}
	for i, d := range dirs {
		if d.Inode == 0 || d.UID != r.UID || d.Mode != 0700 {
			return fmt.Errorf("invalid directory")
		}
		for j := 0; j < i; j++ {
			if dirs[j].Device == d.Device && dirs[j].Inode == d.Inode {
				return fmt.Errorf("directory alias")
			}
		}
	}
	if !validFiles(r.OriginalBefore, r.UID) || !validFiles(r.CandidateBefore, r.UID) ||
		!reflect.DeepEqual(r.OriginalBefore, r.OriginalAfter) || !reflect.DeepEqual(r.CandidateBefore, r.CandidateAfter) {
		return fmt.Errorf("family changed or incomplete")
	}
	for i, original := range r.OriginalBefore {
		candidate := r.CandidateBefore[i]
		if original.Name != candidate.Name || original.Size != candidate.Size {
			return fmt.Errorf("copy extent mismatch")
		}
		for _, member := range r.OriginalBefore {
			if member.Device == candidate.Device && member.Inode == candidate.Inode {
				return fmt.Errorf("copy aliases original")
			}
		}
		changed := p.Offset >= 0 && original.Name == RootName
		if (original.SHA256 != candidate.SHA256) != changed {
			return fmt.Errorf("copy mutation mismatch")
		}
	}
	wantChanges := 0
	if p.Offset >= 0 {
		wantChanges = 1
	}
	if r.ChangedBytes != wantChanges || r.ChangedOffset != p.Offset || r.ChangedXOR != p.XOR || !r.SaltBeforeNonzero || !r.SaltAfterNonzero {
		return fmt.Errorf("wrong mutation")
	}
	if !r.Seed.valid() || r.OpenCalls != 1 || r.Class != p.Class || r.Code != p.Code || !r.CausePreserved ||
		r.SeedAcquires != 1 || r.SeedReleases != 1 || r.CandidateAcquires != p.ProviderCalls || r.CandidateReleases != p.ProviderCalls ||
		r.DuplicateRelease || r.PendingCleanup || r.PlaintextMarker {
		return fmt.Errorf("outcome or lifetime mismatch")
	}
	if p.Case == "control" {
		if !r.DBReturned || r.QueryCalls != 1 || !r.Control.valid() {
			return fmt.Errorf("incomplete control")
		}
	} else if r.DBReturned || r.QueryCalls != 0 || r.Control != (Ledger{}) {
		return fmt.Errorf("negative returned usable state")
	}
	if len(r.Owners) == 0 || len(r.Owners) > MaxOwners {
		return fmt.Errorf("owner inventory missing")
	}
	seedDB, candidateDB, rows, files := 0, 0, 0, 0
	seedID, candidateID := 0, 0
	rowParents := map[int]int{}
	for i, o := range r.Owners {
		if o.ID != i+1 || o.Parent < 0 || o.Parent >= o.ID {
			return fmt.Errorf("invalid owner provenance")
		}
		if o.Kind != "file" && o.Kind != "db" && o.Kind != "rows" {
			return fmt.Errorf("unknown owner kind")
		}
		if o.Parent != 0 && (o.Kind != "rows" || r.Owners[o.Parent-1].Kind != "db") {
			return fmt.Errorf("invalid parent")
		}
		if o.Present {
			if o.Attempts != 1 || (o.Terminal != "closed" && !(o.Kind == "rows" && o.Terminal == "eof")) {
				return fmt.Errorf("unretired owner")
			}
		} else if o.Attempts != 0 || o.Terminal != "absent" {
			return fmt.Errorf("absent owner retirement")
		}
		if o.Kind == "db" {
			if seedDB == 0 {
				if !o.Present {
					return fmt.Errorf("seed db missing")
				}
				seedDB++
				seedID = o.ID
			} else {
				candidateDB++
				candidateID = o.ID
				if o.Present != r.DBReturned {
					return fmt.Errorf("candidate owner disagreement")
				}
			}
		}
		if o.Kind == "file" {
			files++
			if !o.Present || o.Parent != 0 || o.Terminal != "closed" {
				return fmt.Errorf("file owner missing")
			}
		}
		if o.Kind == "db" && o.Parent != 0 {
			return fmt.Errorf("db parent invalid")
		}
		if o.Kind == "rows" {
			rows++
			rowParents[o.Parent]++
			if !o.Present || o.Terminal != "eof" || o.Parent == 0 {
				return fmt.Errorf("incomplete row owner")
			}
		}
	}
	if seedDB != 1 || candidateDB != 1 || rows != 1+r.QueryCalls || files != FileOwners || rowParents[seedID] != 1 || rowParents[candidateID] != r.QueryCalls {
		return fmt.Errorf("owner inventory mismatch")
	}
	return nil
}
