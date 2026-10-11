package sqlite

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestReadOnlyWALOptionPolicyMemory(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, keep := range []bool{false, true} {
			opts := Options{ReadOnly: true, WALShared: true, KeepWALOnClose: keep}
			if err := readOnlyWALOptionPolicy(goos, opts); (err == nil) != (goos == "linux") {
				t.Fatalf("%s keep=%v: %v", goos, keep, err)
			}
			opts.NoCreate = true
			if err := readOnlyWALOptionPolicy(goos, opts); (err == nil) != (goos == "linux") { t.Fatal(err) }
			opts.WALPowersafeOverwrite = true
			if err := readOnlyWALOptionPolicy(goos, opts); !errors.Is(err, ErrUnsupported) { t.Fatal(err) }
		}
	}
	if err := readOnlyWALOptionPolicy("linux", Options{ReadOnly: true, WALExclusive: true}); !errors.Is(err, ErrUnsupported) { t.Fatal(err) }
	if err := readOnlyWALOptionPolicy("darwin", Options{WALShared: true}); err != nil { t.Fatal(err) }
	if err := readOnlyWALOptionPolicy("darwin", Options{ReadOnly: true}); err != nil { t.Fatal(err) }
	d := &DB{opts: Options{ReadOnly: true, WALShared: true}}
	if d.pagerOptions().Create { t.Fatal("ReadOnly permits create") }
	err := d.Checkpoint(context.Background())
	var coded *Error
	if !errors.Is(err, ErrReadOnly) || !errors.As(err, &coded) || coded.Code != CodeReadOnly { t.Fatal(err) }
}

func TestReadOnlyWALPublicSourceGuards(t *testing.T) {
	b, err := os.ReadFile("sqlite.go")
	if err != nil { t.Fatal(err) }
	s := string(b)
	policy := strings.Index(s, "readOnlyWALOptionPolicy(runtime.GOOS, opts)")
	reserve := strings.Index(s, "pager.ReserveSharedClose()")
	canonical := strings.Index(s, "canonicalOpenPath(path, opts.Encryption != nil)")
	if policy < 0 || reserve <= policy || canonical <= reserve { t.Fatal("policy/admission/path ordering") }
	if !strings.Contains(s, "pager.OpenReadOnlySharedSessionAdmitted(ctx, path, d.pagerOptions(), closeAdmission)") { t.Fatal("missing readonly route") }
}
