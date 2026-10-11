package main

import (
	"fmt"
	"strings"
	"testing"
)

// Inert parser inputs only. No source below is compiled or invoked as a database
// workflow, and no reflection or alias is used to bypass the ordinary guard.
func TestScopeRecoveryOptionTypeChains(t *testing.T) {
	for _, source := range []string{
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type First=Second; type Second=s.Options; var x=First{nil}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O s.Options; var x=O{nil}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=(s.Options); var x=O{nil}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; func f(){type O=s.Options; _=O{nil}}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O[T any]=s.Options; var x=O[int]{nil}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O[T,U any]=s.Options; var x=O[int,int]{nil}`,
		`package x; import . "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=Options; var x=O{nil}`,
		`package x; import . "github.com/ms-xiaosnow/gosqlite/sqlite"; import . "another/package"; var x=Options{nil}`,
		`package sqlite; type O=Options; var x=O{nil}`,
	} {
		got, err := inspectSource("inert_alias_test.go", []byte(source))
		if err != nil || len(got) == 0 {
			t.Fatalf("unkeyed option chain escaped: %v %v", got, err)
		}
	}
	for _, source := range []string{
		`package x; import s "another/package"; type O=s.Options; var x=O{nil}`,
		`package x; type Options struct{N int}; type O=Options; var x=O{1}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=s.Options; var x=O{ReadOnly:true}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=s.Options; var x=O{}`,
		`package x; import . "github.com/ms-xiaosnow/gosqlite/sqlite"; var x=Options{ReadOnly:true}`,
		`package x; import . "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=Options; var x=O{}`,
		// Syntactically valid but type-invalid cycles are inert inputs. The
		// source-reference guard must terminate, not become a type checker.
		`package x; type A=B; type B=A; var x=A{1}`,
	} {
		got, err := inspectSource("inert_alias_test.go", []byte(source))
		if err != nil || len(got) != 0 {
			t.Fatalf("safe source control rejected: %v %v", got, err)
		}
	}
	// Reverse-declared chains require propagation independent of source order.
	var source strings.Builder
	source.WriteString(`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; `)
	for i := 0; i < 128; i++ {
		fmt.Fprintf(&source, "type O%d=O%d; ", i, i+1)
	}
	source.WriteString(`type O128=s.Options; var x=O0{nil}`)
	got, err := inspectSource("inert_chain_test.go", []byte(source.String()))
	if err != nil || len(got) != 1 {
		t.Fatalf("reverse alias chain: %v %v", got, err)
	}
}
