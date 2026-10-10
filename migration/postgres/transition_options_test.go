package postgres

import (
	"bytes"
	"strings"
	"testing"
)

func TestTransitionOptionsStrictBoundedDecode(t *testing.T) {
	o, err := DecodeTransitionOptions(strings.NewReader(`{"relation_oid":30000,"attnum":3,"dimensions":3,"approach":"retain_real_array_cast","mode":"single_transaction","vector_schema":"public","target_version":"0.8.7","indexes":[],"shadow_column":"","state_schema":"","state_table":"","batch_size":0,"concurrent_indexes":false,"max_bytes":0}`))
	if err != nil || o.RelationOID != 30000 || o.Approach != RetainArrayCast {
		t.Fatal(o, err)
	}
	for _, raw := range []string{`{"apply":true}`, `{"mode":"single_transaction","mode":"multi_transaction"}`, `{"indexes":[{"m":16,"m":17}]}`, `{"relation_oid":-1}`, `{"relation_oid":1.5}`, `{} {}`, `null`, strings.Repeat(" ", MaxTransitionOptionsBytes+1)} {
		if _, err := DecodeTransitionOptions(strings.NewReader(raw)); err == nil {
			t.Fatalf("accepted invalid options %q", raw[:min(len(raw), 100)])
		}
	}
	if _, err := DecodeTransitionOptions(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	if _, err := DecodeTransitionOptions(bytes.NewReader(nil)); err == nil {
		t.Fatal("empty input accepted")
	}
}
