package main

import "testing"

// Source-string probes only. No namespace operation is executed.
func TestNativeNamespaceMethodReferenceGuard(t *testing.T) {
	for _, src := range []string{`package encstore;func f(){ns.rename(a,b,false)}`, `package encstore;var f=ns.rename`, `package x;var f=adapter.Rename`} {
		got, err := inspectSource("ordinary_test.go", []byte(src))
		if err != nil || len(got) == 0 {
			t.Fatal(got, err)
		}
	}
	src := []byte("//go:build manual_namespace\n\npackage encstore;func f(){ns.rename(a,b,false)}")
	if got, err := inspectSource("namespace_manual_windows_amd64_test.go", src); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
