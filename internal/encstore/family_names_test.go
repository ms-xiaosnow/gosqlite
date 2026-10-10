package encstore

import (
	"errors"
	"strings"
	"testing"
)

func TestEncryptedFamilyPrefixMemoryCanonicalTail(t *testing.T) {
	// ASCII-only injected prefix comparator tests classification laws. It does
	// not model or claim Windows' native Unicode case table.
	prefix := func(name, head string) (string, bool, error) {
		if len(name) < len(head) || !strings.EqualFold(name[:len(head)], head) {
			return "", false, nil
		}
		return name[len(head):], true, nil
	}
	id := strings.Repeat("a", 64)
	for _, name := range []string{"DATABASE.GSE", "DATABASE.GSE.GSE-LOCK", "DATABASE.GSE.GSE-" + id + ".base", "DataBase.gse.gse-" + id + ".retained"} {
		member, err := foldedFamilyEntry(name, "database.gse", prefix)
		if !member || err != nil {
			t.Fatal("case-alias family omitted", name, err)
		}
	}
	for _, name := range []string{"DATABASE.GSE.GSE-" + strings.Repeat("A", 64) + ".base", "DATABASE.GSE.GSE-" + id + ".BASE", "DATABASE.GSE.GSE-unknown"} {
		if _, err := foldedFamilyEntry(name, "database.gse", prefix); !errors.Is(err, ErrUnsupported) {
			t.Fatal("noncanonical tail accepted", name, err)
		}
	}
	if member, err := foldedFamilyEntry("DATABASE.GSEX.gse-"+id+".base", "database.gse", prefix); member || err != nil {
		t.Fatal("unrelated prefix claimed", err)
	}
	marker := errors.New("memory prefix comparison error")
	if _, err := foldedFamilyEntry("a", "a", func(string, string) (string, bool, error) { return "", false, marker }); !errors.Is(err, marker) {
		t.Fatal("comparison failure ignored", err)
	}
}

func TestEncryptedFamilyNameMemoryUTF16Envelope(t *testing.T) {
	for _, name := range []string{strings.Repeat("a", 177), strings.Repeat("😀", 88) + "a"} {
		if err := windowsFamilyNameBound(name); err != nil {
			t.Fatal("exact family envelope", err)
		}
	}
	for _, name := range []string{strings.Repeat("a", 178), strings.Repeat("😀", 89), strings.Repeat("😀", 88) + "aa"} {
		if err := windowsFamilyNameBound(name); !errors.Is(err, ErrUnsupported) {
			t.Fatal("future retained name would overflow", err)
		}
	}
}
