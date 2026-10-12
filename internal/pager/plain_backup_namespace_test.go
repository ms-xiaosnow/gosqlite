package pager

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// All collision/alias cases are inert strings and identity values. They never
// create files, inspect the filesystem, or invoke the backup operation.
func TestPlainBackupMemoryFamilyCrossProducts(t *testing.T) {
	suffixes := []string{"", "-journal", "-wal", "-shm"}
	for _, sourceSuffix := range suffixes {
		for _, destinationSuffix := range suffixes {
			source, destination := "source"+sourceSuffix, "SOURCE"+destinationSuffix
			// Independent set intersection, including all four members of each
			// family. Every case exercises all main/sidecar cross-products.
			family := map[string]bool{}
			for _, suffix := range suffixes {
				family[strings.ToUpper(source+suffix)] = true
			}
			overlaps := false
			for _, suffix := range suffixes {
				overlaps = overlaps || family[strings.ToUpper(destination+suffix)]
			}
			err := backupFamilyDisjoint("linux", source, destination, true)
			rejected := overlaps || !vfs.BackupBasename(source) || !vfs.BackupBasename(destination)
			if errors.Is(err, ErrUnsupported) != rejected || err != nil && !rejected {
				t.Fatal(source, destination, overlaps, err)
			}
		}
	}
	for _, name := range []string{"source", "source-journal", "source-wal", "source-shm", "SOURCE-WAL"} {
		if err := backupFamilyDisjoint("linux", "source", name, true); !errors.Is(err, ErrUnsupported) {
			t.Fatal(name, err)
		}
		if err := backupFamilyDisjoint("linux", name, "source", true); !errors.Is(err, ErrUnsupported) {
			t.Fatal(name, err)
		}
	}
	for _, pair := range [][2]string{{"source.db", "backup.db"}, {"source.db", "source.db.bak"}} {
		if err := backupFamilyDisjoint("linux", pair[0], pair[1], true); err != nil {
			t.Fatal(pair, err)
		}
	}
}

type backupInertParent struct {
	volume, inode uint64
	spelling      string
}

func TestPlainBackupMemoryParentIdentityAndPlatformBoundary(t *testing.T) {
	source := backupInertParent{7, 11, "/canonical/source-parent"}
	for _, destination := range []backupInertParent{{7, 11, "/bind-mount/alias"}, {7, 11, "/symlink-resolved/other-spelling"}, {7, 12, "/different-directory"}, {8, 11, "/different-volume"}} {
		sameParent := source.volume == destination.volume && source.inode == destination.inode
		err := backupFamilyDisjoint("linux", "source.db", "source.db-wal", sameParent)
		if !errors.Is(err, ErrUnsupported) {
			t.Fatal(destination, err)
		}
	}
	for _, name := range []string{"café.db", "cafe\u0301.db", "数据库", "\u212a.db", "SOURCE~1.DB", "file:stream", "file name", "", ".", ".."} {
		if err := backupFamilyDisjoint("linux", "source.db", name, true); !errors.Is(err, ErrUnsupported) {
			t.Fatal(name, err)
		}
		if err := backupFamilyDisjoint("linux", name, "backup.db", true); !errors.Is(err, ErrUnsupported) {
			t.Fatal(name, err)
		}
	}
	// Different physical parents still require supported main-name grammar.
	if err := backupFamilyDisjoint("linux", "数据库", "数据库-wal", false); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	for _, platform := range []string{"darwin", "windows", "freebsd", ""} {
		for _, sameParent := range []bool{false, true} {
			for _, names := range [][2]string{{"source.db", "copy.db"}, {"SOURCE~1.DB", "source.db-wal"}, {"café.db", "cafe\u0301.db-shm"}} {
				if err := backupFamilyDisjoint(platform, names[0], names[1], sameParent); !errors.Is(err, ErrUnsupported) {
					t.Fatal(platform, names, sameParent, err)
				}
			}
		}
	}
}

func TestPlainBackupMemoryNamespaceErrorGraphs(t *testing.T) {
	cause := errors.New("inert source namespace probe")
	for _, native := range []error{fmt.Errorf("wrapped: %w", errors.Join(vfs.ErrIdentity, cause)), errors.Join(vfs.ErrPoisoned, vfs.ErrIdentity, cause)} {
		err := finishPlaintextBackup(nativeFileError(native), nil, false)
		for _, target := range []error{cause, vfs.ErrIdentity} {
			if !errors.Is(err, target) {
				t.Fatal("lost namespace cause", target, err)
			}
		}
		if errors.Is(native, vfs.ErrPoisoned) && !errors.Is(err, ErrPoisoned) {
			t.Fatal(err)
		}
		if errors.Is(err, ErrCommitUncertain) {
			t.Fatal("pre-create rejection became publication uncertainty", err)
		}
	}
}

func TestPlainBackupSourceFamilyGuardOrdering(t *testing.T) {
	if err := backupPagerSourceGuard(backupPagerSourceInputs(t)); err != nil { t.Fatal(err) }
}

func TestPlainBackupSourceNamespaceKeepsOwner(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "plain_backup_source.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "ObserveBackupNamespace" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("missing source observation")
	}
	observes, normalizes, namespace, closes := 0, 0, 0, 0
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "nativeFileError" {
			normalizes++
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "observe":
				observes++
			case "ObserveDirectSourceFamily":
				namespace++
			case "Close", "begin", "retry":
				closes++
			}
		}
		return true
	})
	if observes != 1 || normalizes != 1 || namespace != 1 || closes != 0 {
		t.Fatal(observes, normalizes, namespace, closes)
	}
}

func TestPlainBackupSourceSHMProfileGuard(t *testing.T) {
	// Static only: never create or read a database with an SHM collision.
	f, err := parser.ParseFile(token.NewFileSet(), "plain_backup.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "ValidateRollbackBackupSource" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("missing source validation")
	}
	probes := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != "nativeLstatForCreate" {
			return true
		}
		if len(call.Args) == 1 {
			if path, ok := call.Args[0].(*ast.BinaryExpr); ok && path.Op == token.ADD {
				if suffix, ok := path.Y.(*ast.BasicLit); ok && suffix.Value == `"-shm"` {
					probes++
				}
			}
		}
		return true
	})
	if probes != 1 {
		t.Fatal("SHM source-profile preflight missing", probes)
	}
}
