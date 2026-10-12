package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Inert source strings only. No manual binary, SQL, fixture or key operation.
func encryptedRootFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestEncryptedRootFiniteStructure(t *testing.T) {
	for _, path := range []string{encryptedRootNative, encryptedRootModel, encryptedRootModelTest, "internal/testscope/main.go"} {
		findings, err := inspectEncryptedRootNegativeStructure(path, encryptedRootFixture(t, path))
		if err != nil || len(findings) != 0 {
			t.Fatal(path, findings, err)
		}
	}
}
func TestEncryptedRootFalsePassSourcesEvenWhenRepinned(t *testing.T) {
	original := string(encryptedRootFixture(t, encryptedRootNative))
	// Accept the two equivalent gofmt spellings of these finite anchors.
	original = strings.ReplaceAll(original, "ReadOnly: true, NoCreate: true", "ReadOnly:true, NoCreate:true")
	original = strings.ReplaceAll(original, "os.O_WRONLY | os.O_CREATE | os.O_EXCL", "os.O_WRONLY|os.O_CREATE|os.O_EXCL")
	for _, mutation := range [][2]string{
		{`//go:build linux && manual_encrypted_root_negative`, `//go:build linux || manual_encrypted_root_negative`},
		{`os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_NEGATIVE") != "independently_authorized"`, `false`},
		{`ReadOnly:true, NoCreate:true`, `ReadOnly:false, NoCreate:true`},
		{`ReadOnly:true, NoCreate:true`, `ReadOnly:true, NoCreate:false`},
		{`candidateOwner := r.capture("db", 0, db, nil, nil, openErr)`, `candidateOwner := r.capture("db", 0, db, nil, nil, nil)`},
		{`r.writeCopy(filepath.Join(candidate, member.Name), data)`, `r.writeCopy(filepath.Join(original, member.Name), data)`},
		{`r.receipt.Class, r.receipt.Code, r.receipt.CausePreserved = rootAuthClass(openErr)`, `r.receipt.Class, r.receipt.Code, r.receipt.CausePreserved = plan.Class, plan.Code, true`},
		{`r.receipt.OriginalAfter, _ = r.snapshot(original)`, `r.receipt.OriginalAfter = r.receipt.OriginalBefore`},
		{`os.O_WRONLY|os.O_CREATE|os.O_EXCL`, `os.O_WRONLY|os.O_CREATE|os.O_TRUNC`},
		{`result.ExactRows && result.EOF && result.RowsErrNil`, `true`},
		{`r.receipt.OpenCalls++`, `r.receipt.OpenCalls++; sqlite.OpenContext(ctx, "retry", sqlite.Options{})`},
		{`r.receipt.Status = "observed"`, `db.Rekey(ctx, candidateOptions); r.receipt.Status = "observed"`},
	} {
		if strings.Count(original, mutation[0]) != 1 {
			t.Fatal("mutation anchor changed", mutation[0])
		}
		raw := []byte(strings.Replace(original, mutation[0], mutation[1], 1))
		old := encryptedRootPins[encryptedRootNative]
		encryptedRootPins[encryptedRootNative] = fmt.Sprintf("%x", sha256.Sum256(raw))
		findings, err := inspectEncryptedRootNegativeSource(encryptedRootNative, raw)
		encryptedRootPins[encryptedRootNative] = old
		if err != nil || len(findings) == 0 {
			t.Fatal("repinned false PASS", mutation[0], findings, err)
		}
	}
}
func TestEncryptedRootWholeSourceAndCapabilityBoundary(t *testing.T) {
	for _, source := range []string{
		"//go:build linux && manual_encrypted_root_negative\n\npackage sqlite_test\nfunc TestCopied() {}",
		`package ordinary; import "github.com/ms-xiaosnow/gosqlite/internal/testkit/encryptedroot"; var _ = encryptedroot.Select`,
		`package ordinary; func TestManualEncryptedRootNegative() {}`,
		`package ordinary; var rootAuthRetained = 1`,
	} {
		findings, err := inspectEncryptedRootNegativeStructure("sqlite/copied_test.go", []byte(source))
		if err != nil || len(findings) == 0 {
			t.Fatal("copied capability accepted", findings, err)
		}
	}
	for _, path := range []string{encryptedRootNative, encryptedRootModel, encryptedRootModelTest} {
		raw := encryptedRootFixture(t, path)
		old := encryptedRootPins[path]
		encryptedRootPins[path] = fmt.Sprintf("%x", sha256.Sum256(raw))
		findings, err := inspectEncryptedRootNegativeSource(path, append(raw, []byte("\n// unreviewed bytes\n")...))
		encryptedRootPins[path] = old
		if err != nil || len(findings) == 0 {
			t.Fatal("unreviewed whole-source change accepted")
		}
	}
	findings, err := inspectEncryptedRootNegativeStructure(encryptedRootModel, []byte("package encryptedroot\nimport \"os\"\nvar _ = os.Open"))
	if err != nil || len(findings) == 0 {
		t.Fatal("native scalar model accepted")
	}
}
func TestEncryptedRootPinsRequireIndependentRelease(t *testing.T) {
	for path, pin := range encryptedRootPins {
		if fmt.Sprintf("%x", sha256.Sum256(encryptedRootFixture(t, path))) != pin {
			t.Fatal("source review HOLD", path)
		}
	}
}
