package encstore

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Every name and directory entry in these tests is an in-memory value. No
// namespace fixture is created and no filesystem equivalence is simulated.
func TestDarwinUnicodeSiblingMemoryLeadingMismatch(t *testing.T) {
	for _, name := range []string{
		"unrelated-Ω.txt", "j\u0301.gse", "J\u200d.gse", "jK.gse", "jK.gse.gse-unknown",
		"k.GSf\u0301", "k.GSf\u200c", "k.GSf\u0345", "k.GSf😀", "k.GSf\ufeff",
	} {
		member, err := darwinFamilyEntry(name, "k.gse")
		if member || err != nil {
			t.Fatalf("proved unrelated %q: member=%v err=%v", name, member, err)
		}
	}
}

func TestDarwinUnicodeSiblingMemoryAmbiguityStopsBeforeSkip(t *testing.T) {
	for _, name := range []string{
		"K.gse", "K.gse.gse-unknown", "ſ.gse", "Ω-unrelated.txt", "é-unrelated.txt",
		"\u200dk.gse", "k\u200d.gse", "k\u0301x.gse", "k\u0345x.gse",
		"k.gse\u200d", "K.GSE\u0301", "k.gse-unrelated-Ω.txt",
		"k.gse.gse-\u200dunknown", "k.gse.gſe-unknown",
		"k.gse.gse-" + strings.Repeat("a", 64) + ".retained\u200d",
		"k.gse.gse-" + strings.Repeat("a", 63) + "K.base",
		"\x01jΩ", "\x7fjΩ", "k\x01xΩ", "k\x7fxΩ",
		"unrelated-\xff", "unrelated-\xc0\xaf", "unrelated-\xed\xa0\x80",
	} {
		member, err := darwinFamilyEntry(name, "k.gse")
		if member || !errors.Is(err, ErrUnsupported) {
			t.Fatalf("ambiguous/invalid %q: member=%v err=%v", name, member, err)
		}
	}
}

func TestDarwinUnicodeSiblingMemoryPrintableASCIIBoundaries(t *testing.T) {
	fold := func(c byte) byte {
		if c >= 'A' && c <= 'Z' {
			return c + 'a' - 'A'
		}
		return c
	}
	// Every printable pair: only a real mismatch inside the base can skip.
	// A slash cannot be returned by the wire parser and is not a basename.
	for a := byte(32); a <= 126; a++ {
		if a == '/' {
			continue
		}
		for b := byte(32); b <= 126; b++ {
			if b == '/' {
				continue
			}
			base := "P" + string(b) + ".gse"
			for _, suffix := range []string{"Ω", "\u0301", "\u200d", "\u0345", "K", "😀"} {
				name := "p" + string(a) + suffix
				member, err := darwinFamilyEntry(name, base)
				if member || (fold(a) != fold(b) && err != nil) || (fold(a) == fold(b) && !errors.Is(err, ErrUnsupported)) {
					t.Fatalf("pair name=%q base=%q: member=%v err=%v", name, base, member, err)
				}
			}
		}
	}
	for _, tc := range []struct{ name, base string }{
		{"", "k.gse"}, {"K", "k.gse"}, {"k.gse", "k.gse"},
		{"k.gse-xΩ", "k.gse"}, {"aΩx", "ab"}, {"ab", "aΩ"},
		{"a\x01x", "ab"}, {"a\x7fx", "ab"},
		{"bΩ", "aΩ"}, {"bΩ", "a\x01"}, {"k.gse", "K.gse"},
	} {
		if darwinASCIIBaseMismatch(tc.name, tc.base) {
			t.Fatalf("no qualifying mismatch in %q / %q", tc.name, tc.base)
		}
	}
	base := strings.Repeat("a", 176) + "b"
	if member, err := darwinFamilyEntry(strings.Repeat("A", 176)+"cΩ", base); member || err != nil {
		t.Fatal("last byte of maximum root", member, err)
	}
	if member, err := darwinFamilyEntry(strings.Repeat("A", 176)+"\u200dc", base); member || !errors.Is(err, ErrUnsupported) {
		t.Fatal("Unicode immediately before final mismatch", member, err)
	}
}

func TestDarwinUnicodeSiblingMemoryFamilyGrammarUnchanged(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, name := range []string{"K.GSE", "K.GSE.GSE-LOCK", "K.GSE.GSE-" + id + ".base", "K.GSE.gse-" + id + ".wal", "k.gse.gse-" + id + ".retained", "k.gse.gse-" + id + ".pending"} {
		if member, err := darwinFamilyEntry(name, "k.gse"); !member || err != nil {
			t.Fatal("family omitted", name, member, err)
		}
	}
	for _, name := range []string{"k.gse.gse-unknown", "k.gse.gse-" + strings.Repeat("A", 64) + ".base", "k.gse.gse-" + id + ".BASE", "k.gse.gse-" + id + ".base.extra"} {
		if member, err := darwinFamilyEntry(name, "k.gse"); member || !errors.Is(err, ErrUnsupported) {
			t.Fatal("unknown family accepted", name, member, err)
		}
	}
}

func TestDarwinUnicodeSiblingMemoryDirectoryKindsDoNotClassifyFamily(t *testing.T) {
	for _, kind := range []byte{0, 1, 2, 4, 6, 8, 10, 12, 255} {
		for _, tc := range []struct {
			name        string
			member      bool
			unsupported bool
		}{
			{"unrelated-Ω.txt", false, false},
			{"K.gse", false, true},
			{"K.gse", true, false},
		} {
			raw := directoryMemoryRecord(tc.name, 1)
			raw[20] = kind
			entry, used, skip, err := nextDarwinDirectoryName(raw)
			if err != nil || skip || used != len(raw) {
				t.Fatal("memory wire record", kind, err)
			}
			member, err := darwinFamilyEntry(entry.Name(), "k.gse")
			if member != tc.member || errors.Is(err, ErrUnsupported) != tc.unsupported {
				t.Fatal("kind changed classification", kind, tc.name, member, err)
			}
		}
	}
}

func TestDarwinUnicodeSiblingSourceInventoryCountsBeforeClassification(t *testing.T) {
	// Source-only structural guard: even newly unrelated entries consume the
	// directory-entry quota; family metadata still goes through ns.checked.
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "inventory.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var count, limit, classify, checked token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		if inc, ok := n.(*ast.IncDecStmt); ok {
			if id, ok := inc.X.(*ast.Ident); ok && id.Name == "scanned" {
				count = inc.Pos()
			}
		}
		if comparison, ok := n.(*ast.BinaryExpr); ok && comparison.Op == token.GTR {
			left, lOK := comparison.X.(*ast.Ident)
			right, rOK := comparison.Y.(*ast.Ident)
			if lOK && rOK && left.Name == "scanned" && right.Name == "maxEntries" {
				limit = comparison.Pos()
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "platformFamilyEntry" {
				if len(call.Args) != 3 {
					t.Fatal("classifier missing managed enumeration directory")
				}
				dir, ok := call.Args[2].(*ast.Ident)
				if !ok || dir.Name != "f" {
					t.Fatal("classifier is not bound to enumerated directory")
				}
				classify = call.Pos()
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "checked" {
				checked = call.Pos()
			}
		}
		return true
	})
	if count == 0 || !(count < limit && limit < classify && classify < checked) {
		t.Fatal("entry count/limit/classification/managed check order", count, limit, classify, checked)
	}
	raw, err := os.ReadFile("platform_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"member, err := darwinFamilyEntry(name, base)",
		"raw, e := dir.rawLocked()", "!dir.descriptor.directory",
		"unix.Fstatfs(int(raw.Fd()), &fs)",
		"!darwinUnicodeInventoryFilesystem(fs.Fstypename, fs.Fssubtype, fs.Flags&unix.MNT_LOCAL != 0)",
	} {
		if !strings.Contains(string(raw), required) {
			t.Fatal("missing exact descriptor filesystem routing", required)
		}
	}
}

func TestDarwinUnicodeSiblingMemoryFilesystemDomain(t *testing.T) {
	native := func(s string) (b [16]byte) { copy(b[:], s); return }
	for _, tc := range []struct {
		name    string
		subtype uint32
		allowed bool
	}{
		{"apfs", 0, true}, {"apfs", 0xffffffff, true},
		{"hfs", 0, true}, {"hfs", 1, true}, {"hfs", 2, true}, {"hfs", 3, true},
		{"hfs", 4, false}, {"hfs", 127, false}, {"hfs", 128, false}, {"hfs", 0xffffffff, false},
		{"", 0, false}, {"APFS", 0, false}, {"HFS", 0, false},
		{"apfsx", 0, false}, {"hfs+", 0, false}, {"hfsx", 0, false},
		{"exfat", 0, false}, {"msdos", 0, false}, {"ntfs", 0, false}, {"smbfs", 0, false},
		{"osxfuse", 0, false}, {"1234567890123456", 0, false},
	} {
		for _, local := range []bool{false, true} {
			if got := darwinUnicodeInventoryFilesystem(native(tc.name), tc.subtype, local); got != (local && tc.allowed) {
				t.Fatalf("filesystem %q subtype=%d local=%v: %v", tc.name, tc.subtype, local, got)
			}
		}
	}
}
