package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	pg "github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func TestReviewRealArrayIndependentBinary32(t *testing.T) {
	random := rand.New(rand.NewSource(20261010))
	cases := [][]float32{{math.Float32frombits(0x80000000), 0, math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32, math.MaxFloat32}}
	for n := 0; n < 200; n++ {
		values := make([]float32, 1+n%13)
		for i := range values {
			for {
				bits := random.Uint32()
				if bits&0x7f800000 != 0x7f800000 {
					values[i] = math.Float32frombits(bits)
					break
				}
			}
		}
		cases = append(cases, values)
	}
	for n, want := range cases {
		tokens := make([]string, len(want))
		for i, v := range want {
			tokens[i] = strconv.FormatFloat(float64(v), 'g', -1, 32)
			if (n+i)%2 == 0 {
				tokens[i] = `"` + tokens[i] + `"`
			}
		}
		text := "{" + strings.Join(tokens, ", ") + "}"
		if n%2 == 0 {
			text = "[1:" + strconv.Itoa(len(want)) + "] = " + text
		}
		got, err := pg.ParseRealArray(text, len(want))
		if err != nil {
			t.Fatalf("case%d: %v", n, err)
		}
		values := got.Values()
		if len(values) != len(want) {
			t.Fatal("dimension changed")
		}
		for i, v := range values {
			if math.Float32bits(v) != math.Float32bits(want[i]) {
				t.Fatalf("case%d component%d: bit mismatch", n, i)
			}
		}
	}
	invalid := []string{"NULL", "{}", "{NULL}", `{"NULL"}`, "{{1}}", "[0:0]={1}", "[1:1][1:1]={{1}}", "[1:2]={1}", "{1,}", "{1} trailing", "{NaN}", "{Infinity}", "{1e400}", "{0x1p0}", "{1_0}", `{"1,2"}`, "{true}", "{\x00}", "{\xff}"}
	for _, input := range invalid {
		v, err := pg.ParseRealArray(input, 1)
		if err == nil {
			t.Errorf("accepted invalid array: %q", input)
		}
		if v.Dims() != 0 {
			t.Fatal("partial value escaped on error")
		}
	}
	secret := "private-embedding-marker-8237"
	_, err := pg.ParseRealArray("{"+secret+"}", 1)
	var typed *pg.ValueError
	if err == nil || !errors.As(err, &typed) || strings.Contains(err.Error(), secret) {
		t.Fatal("untyped error or source contents leaked", err)
	}
	for _, dims := range []int{-1, 0, 16001} {
		if _, err := pg.ParseRealArray("{1}", dims); err == nil {
			t.Fatal("unbounded dimensions accepted")
		}
	}
}

func reviewFixture(t *testing.T) ([]byte, pg.CatalogSnapshot) {
	t.Helper()
	b, err := os.ReadFile("../../testdata/pgembedding-0.3.6/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := pg.DecodeCatalog(bytes.NewReader(b), pg.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return b, c
}
func reviewHas(r pg.Report, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
func TestReviewCatalogStrictnessAndIdentity(t *testing.T) {
	b, _ := reviewFixture(t)
	bad := [][]byte{
		[]byte(strings.Replace(string(b), `"format_version":`, `"format_version":1,"format_version":`, 1)),
		[]byte(strings.Replace(string(b), `"name":`, `"name":"spoof","name":`, 1)),
		[]byte(strings.Replace(string(b), `"format_version":`, `"unknown_root":0,"format_version":`, 1)),
		append(append([]byte{}, b...), []byte(" {}")...),
		append(append([]byte{}, b...), 0xff),
	}
	for i, input := range bad {
		if _, err := pg.DecodeCatalog(bytes.NewReader(input), pg.Limits{}); err == nil {
			t.Fatalf("strict JSON case%d accepted", i)
		}
	}
	mutate := func(f func(*pg.CatalogSnapshot)) pg.Report {
		t.Helper()
		_, c := reviewFixture(t)
		f(&c)
		r, err := pg.Inspect(context.Background(), c, pg.InspectOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := mutate(func(c *pg.CatalogSnapshot) { c.Extensions[0].Version = "9.99" })
	if r.Status != "blocked" || !reviewHas(r, "source_version_unsupported") {
		t.Fatal("source version accepted", r)
	}
	r = mutate(func(c *pg.CatalogSnapshot) { c.OpClasses[0].ExtensionOID = 0 })
	if !reviewHas(r, "legacy_index_identity_mismatch") {
		t.Fatal("name-only opclass identity accepted", r)
	}
	for _, options := range [][]string{{"dims=3"}, {"dims=3", "m=3", "efconstruction=5"}} {
		r = mutate(func(c *pg.CatalogSnapshot) { c.Indexes[0].Options = options })
		if !reviewHas(r, "target_option_constraint") {
			t.Fatal("invalid legacy parameter pair was silently accepted", r)
		}
		if len(r.Indexes) == 0 || r.Indexes[0].ProposedTarget != nil {
			t.Fatal("invalid target proposal escaped", r)
		}
	}
	_, c := reviewFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pg.Inspect(ctx, c, pg.InspectOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
	r1, err := pg.Inspect(context.Background(), c, pg.InspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := pg.Inspect(context.Background(), c, pg.InspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var one, two bytes.Buffer
	if err = pg.RenderJSON(&one, r1); err != nil {
		t.Fatal(err)
	}
	if err = pg.RenderJSON(&two, r2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one.Bytes(), two.Bytes()) || !json.Valid(one.Bytes()) {
		t.Fatal("report is nondeterministic or invalid JSON")
	}
}

func TestReviewCatalogProgrammaticFiniteAndProvenance(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, c := reviewFixture(t)
		c.Profiles[0].MinNorm = &value
		if _, err := pg.Inspect(context.Background(), c, pg.InspectOptions{}); err == nil {
			t.Fatal("nonfinite programmatic profile accepted")
		}
	}
	_, c := reviewFixture(t)
	first, err := pg.Inspect(context.Background(), c, pg.InspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c.Relations[0].Name = "renamed_source"
	second, err := pg.Inspect(context.Background(), c, pg.InspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.CatalogSHA256 == second.CatalogSHA256 {
		t.Fatal("report reuses stale raw hash after exported snapshot mutation")
	}
}
