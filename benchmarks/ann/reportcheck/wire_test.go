package reportcheck

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestANNIVFListsNoIdentityCollision(t *testing.T) {
	r := modelMethodReport(t, "ivfflat")
	r.Config.Probes = 1
	r.Config.MaxProbes = 4
	var prior string
	for _, lists := range []int{8, 16} {
		r.Config.Lists = lists
		r.QueryPolicy = queryPolicy(r.Config)
		var e error
		r.Algorithm, e = canonicalAlgorithm(r.Config)
		if e != nil {
			t.Fatal(e)
		}
		hash, e := workloadDigest(r)
		if e != nil {
			t.Fatal(e)
		}
		if hash == prior {
			t.Fatal("IVF list8/list16 material identity collision")
		}
		prior = hash
	}
}
func TestANNAllCanonicalAlgorithmFields(t *testing.T) {
	for _, method := range []string{"hnsw", "ivfflat"} {
		base := modelMethodReport(t, method)
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(base.Algorithm, &fields); e != nil {
			t.Fatal(e)
		}
		for key, raw := range fields {
			t.Run(method+"/"+key, func(t *testing.T) {
				r := modelMethodReport(t, method)
				var changed []byte
				if len(raw) > 0 && raw[0] == '"' {
					var value string
					if e := json.Unmarshal(raw, &value); e != nil {
						t.Fatal(e)
					}
					changed, _ = json.Marshal(value + "_changed")
				} else {
					number, e := strconv.ParseUint(string(raw), 10, 64)
					if e != nil {
						t.Fatal(e)
					}
					changed = []byte(strconv.FormatUint(number+1, 10))
				}
				needle := []byte(strconv.Quote(key) + ":" + string(raw))
				replacement := []byte(strconv.Quote(key) + ":" + string(changed))
				r.Algorithm = bytes.Replace(r.Algorithm, needle, replacement, 1)
				if bytes.Equal(r.Algorithm, base.Algorithm) {
					t.Fatal("no material mutation")
				}
				if hash, e := workloadDigest(r); e == nil {
					if hash == base.WorkloadSHA256 {
						t.Fatal("material algorithm collision")
					}
					r.WorkloadSHA256 = hash
				}
				if e := r.ValidatePass(); e == nil {
					t.Fatal("changed normalized algorithm/policy admitted")
				}
			})
		}
	}
}
func TestANNQueryBudgetAndProtocolIdentities(t *testing.T) {
	changes := map[string]func(*Report){"EF": func(r *Report) { r.QueryPolicy.EFSearch++ }, "max EF": func(r *Report) { r.QueryPolicy.MaxEFSearch++ }, "probes": func(r *Report) { r.QueryPolicy.Probes++ }, "max probes": func(r *Report) { r.QueryPolicy.MaxProbes++ }, "fallback": func(r *Report) { r.QueryPolicy.FallbackPolicy += "x" }, "methodversion": func(r *Report) { r.QueryPolicy.MethodVersion++ }, "generation": func(r *Report) { r.QueryPolicy.Generation++ }, "query order": func(r *Report) { r.TimingSemantics.Order += "x" }, "duration budget": func(r *Report) { r.Config.MaxDuration += time.Second }, "memory budget": func(r *Report) { r.Config.MaxMemoryBytes++ }, "data budget": func(r *Report) { r.Config.MaxDataBytes++ }, "work budget": func(r *Report) { r.Config.MaxWork++ }}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			r := modelReport(t)
			before := r.WorkloadSHA256
			change(&r)
			after, e := workloadDigest(r)
			if e != nil {
				t.Fatal(e)
			}
			if before == after {
				t.Fatal("material workload identity did not change")
			}
			if e = r.ValidatePass(); e == nil {
				t.Fatal("old checksum admitted after material change")
			}
		})
	}
	r := modelReport(t)
	r.Config.Seed++
	refreshModelHashes(t, &r)
	if e := r.ValidatePass(); e == nil {
		t.Fatal("rehashed false seeded dataset admitted")
	}
}
func TestANNStrictWireShapeTypes(t *testing.T) {
	r := modelReport(t)
	raw, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(raw, &fields); e != nil {
		t.Fatal(e)
	}
	for key := range fields {
		t.Run("missing "+key, func(t *testing.T) {
			copy := make(map[string]json.RawMessage, len(fields))
			for k, v := range fields {
				copy[k] = v
			}
			delete(copy, key)
			b, e := json.Marshal(copy)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = DecodeReport(b); e == nil {
				t.Fatal("missing required key admitted")
			}
		})
	}
	variants := map[string][]byte{
		"duplicate schema":   bytes.Replace(raw, []byte(`"schema_version":3`), []byte(`"schema_version":3,"schema_version":3`), 1),
		"case alias":         bytes.Replace(raw, []byte(`"schema_version":3`), []byte(`"schema_version":3,"SCHEMA_VERSION":3`), 1),
		"unknown":            append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"unknown":0}`)...),
		"trailing":           append(append([]byte{}, raw...), []byte(` {}`)...),
		"null safety":        bytes.Replace(raw, []byte(`"unverified_retirement":false`), []byte(`"unverified_retirement":null`), 1),
		"null counter":       bytes.Replace(raw, []byte(`"ANNNodeReads":0`), []byte(`"ANNNodeReads":null`), 1),
		"counter fractional": bytes.Replace(raw, []byte(`"ANNNodeReads":0`), []byte(`"ANNNodeReads":0.5`), 1),
		"numeric string":     bytes.Replace(raw, []byte(`"rows":256`), []byte(`"rows":"256"`), 1),
		"negative unsigned":  bytes.Replace(raw, []byte(`"max_memory_bytes":268435456`), []byte(`"max_memory_bytes":-1`), 1),
		"wrong seed":         bytes.Replace(raw, []byte(`"seed":2611923443488327891`), []byte(`"seed":2611923443488327890`), 1),
		"seed float":         bytes.Replace(raw, []byte(`"seed":2611923443488327891`), []byte(`"seed":2611923443488327891.0`), 1),
		"null limits":        bytes.Replace(raw, []byte(`"native_ann_query":{`), []byte(`"native_ann_query":null,"bad":{`), 1),
		"extra K":            bytes.Replace(raw, []byte(`"k":[1,10,100]`), []byte(`"k":[1,10,100,1000]`), 1),
		"unknown nested":     bytes.Replace(raw, []byte(`"MaxNodeReads":100000`), []byte(`"MaxNodeReads":100000,"Unknown":1`), 1),
		"duplicate nested":   bytes.Replace(raw, []byte(`"MaxNodeReads":100000`), []byte(`"MaxNodeReads":100000,"MaxNodeReads":100000`), 1),
		"null build setting": bytes.Replace(raw, []byte(`"main_sum":""`), []byte(`"main_sum":null`), 1),
	}
	for name, b := range variants {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(raw, b) {
				t.Fatal("test did not mutate input")
			}
			if _, e := DecodeReport(b); e == nil {
				t.Fatal("malformed wire accepted")
			}
		})
	}
	if _, e := DecodeReport([]byte(strings.Repeat("[", 65) + strings.Repeat("]", 65))); e == nil {
		t.Fatal("deep JSON accepted")
	}
}
func TestANNMaterialPlansAndCounters(t *testing.T) {
	for _, method := range []string{"hnsw", "ivfflat"} {
		for _, which := range []string{"exact", "ann"} {
			r := modelMethodReport(t, method)
			plan := r.Samples[0].Observations[0].Exact.Plan
			if which == "ann" {
				plan = r.Samples[0].Observations[0].ANN.Plan
			}
			typ := reflect.TypeOf(plan.QueryPlan)
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				t.Run(method+"/"+which+"/plan/"+field.Name, func(t *testing.T) {
					r := modelMethodReport(t, method)
					p := &r.Samples[0].Observations[0].Exact.Plan
					if which == "ann" {
						p = &r.Samples[0].Observations[0].ANN.Plan
					}
					v := reflect.ValueOf(&p.QueryPlan).Elem().FieldByName(field.Name)
					switch v.Kind() {
					case reflect.String:
						v.SetString(v.String() + "_changed")
					case reflect.Int, reflect.Int64:
						v.SetInt(v.Int() + 1)
					case reflect.Bool:
						v.SetBool(!v.Bool())
					case reflect.Slice:
						x := reflect.New(v.Type().Elem()).Elem()
						if x.Kind() == reflect.String {
							x.SetString("changed")
						} else {
							x.SetInt(1)
						}
						v.Set(reflect.Append(v, x))
					default:
						t.Fatalf("unhandled material plan field %s", field.Name)
					}
					if e := r.ValidatePass(); e == nil {
						t.Fatal("changed material plan admitted")
					}
				})
			}
			stats := r.Samples[0].Observations[0].Exact.Stats.QueryStats
			st := reflect.TypeOf(stats)
			for i := 0; i < st.NumField(); i++ {
				field := st.Field(i)
				t.Run(method+"/"+which+"/counter/"+field.Name, func(t *testing.T) {
					r := modelMethodReport(t, method)
					s := &r.Samples[0].Observations[0].Exact.Stats
					if which == "ann" {
						s = &r.Samples[0].Observations[0].ANN.Stats
					}
					v := reflect.ValueOf(&s.QueryStats).Elem().FieldByName(field.Name)
					v.SetInt(math.MaxInt64)
					if e := r.ValidatePass(); e == nil {
						t.Fatal("unbounded/inconsistent counter admitted")
					}
				})
			}
		}
	}
}
func TestANNRehashedConfigurationCannotBypassMaterialValidation(t *testing.T) {
	for _, method := range []string{"hnsw", "ivfflat"} {
		for _, change := range []func(*Report){func(r *Report) { r.QueryPolicy.MaxEFSearch++ }, func(r *Report) { r.Limits.ANN.MaxNodeReads++ }, func(r *Report) { r.TimingSemantics.Percentiles = "invented" }, func(r *Report) { r.BuildSettings["-compiler"] = "unknown" }, func(r *Report) { r.BuildSettings["-unknown"] = "flag" }, func(r *Report) { r.Hardware.CPUModel = "" }} {
			r := modelMethodReport(t, method)
			change(&r)
			refreshModelHashes(t, &r)
			if e := r.ValidatePass(); e == nil {
				t.Fatal("rehashed invalid material config accepted", method)
			}
		}
	}
}

func TestANNPortablePathAndInvalidUTF8Controls(t *testing.T) {
	for _, v := range [][3]string{{"linux", "/retained/evidence", "/retained/evidence/report.json"}, {"windows", `C:\retained\evidence`, `C:\retained\evidence\report.json`}, {"windows", `\\server\share\evidence`, `\\server\share\evidence\report.json`}} {
		if !retainedPathsValid(v[0], v[1], v[2]) {
			t.Fatal("portable path rejected", v)
		}
	}
	for _, v := range [][3]string{{"linux", "relative", "relative/report.json"}, {"linux", "/a/../b", "/b/report.json"}, {"windows", `C:relative`, `C:relative\report.json`}, {"windows", `\\server\share\..\evidence`, `\\server\evidence\report.json`}} {
		if retainedPathsValid(v[0], v[1], v[2]) {
			t.Fatal("invalid path accepted", v)
		}
	}
	r := modelReport(t)
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	b = bytes.Replace(b, []byte("AMD EPYC"), []byte{'A', 0xff, 'D'}, 1)
	if _, e = DecodeReport(b); e == nil {
		t.Fatal("invalid UTF8 admitted")
	}
}

func TestANNUTCParsingRejectsNumericOffsets(t *testing.T) {
	check := func(t *testing.T, r Report, want string) {
		t.Helper()
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		_, wire := DecodeReport(b)
		for name, err := range map[string]error{"ValidatePass": r.ValidatePass(), "DecodeReport": wire} {
			if want == "" {
				if err != nil {
					t.Fatal(name, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: want %q, got %v", name, want, err)
			}
		}
	}
	check(t, modelReport(t), "")
	for _, field := range []string{"started", "finished"} {
		t.Run(field, func(t *testing.T) {
			r := modelReport(t)
			if field == "started" {
				r.StartedUTC = "2026-10-10T00:00:00+00:00"
			} else {
				r.FinishedUTC = "2026-10-10T00:01:00+00:00"
			}
			check(t, r, "UTC/elapsed interval mismatch")
		})
	}
}
