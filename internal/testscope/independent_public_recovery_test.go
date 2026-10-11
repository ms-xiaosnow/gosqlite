package main

import "testing"

// These are inert source strings, never compiled or run as database operations.
// The original independent alias/dot-import witnesses now require detection.
// This remains prevention-only, never authorization or full call-graph proof.
func TestIndependentReviewGuardDirectAndAliasLimit(t *testing.T) {
	cases := []struct {
		source   string
		detected bool
	}{
		{`package sqlite; var f=recoverRollbackOpen`, true},
		{`package x; import p "github.com/ms-xiaosnow/gosqlite/internal/pager"; var f=(p.RecoverRollback)`, true},
		{`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=s.Options; var x=O{nil,nil,false,false,&s.RollbackRecoveryOptions{},false,false,false,false,0,0,0,0,0}`, true},
		{`package x; import . "github.com/ms-xiaosnow/gosqlite/sqlite"; var x=Options{nil,nil,false,false,&RollbackRecoveryOptions{},false,false,false,false,0,0,0,0,0}`, true},
	}
	for _, c := range cases {
		got, err := inspectSource("inert_test.go", []byte(c.source))
		if err != nil || (len(got) != 0) != c.detected {
			t.Fatalf("guard result: %v %v", got, err)
		}
	}
}
