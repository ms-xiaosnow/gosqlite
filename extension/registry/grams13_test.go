package registry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
)

func configuredGrams(t *testing.T, config grams13.Config) registry.Index {
	t.Helper()
	r, err := registry.New(grams13.Method())
	if err != nil {
		t.Fatal(err)
	}
	m, ok := r.Lookup(grams13.MethodName, grams13.MethodVersion)
	if !ok {
		t.Fatal("grams13 not registered")
	}
	canonical, err := grams13.ConfigBytes(config)
	if err != nil {
		t.Fatal(err)
	}
	index, err := m.Configure(canonical)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func TestGramsMethodConfigRoundTripAndOwnership(t *testing.T) {
	match := grams13.DefaultMatchConfig()
	match.CaseFold = grams13.CaseFoldUnicode
	match.Whitespace = grams13.WhitespaceCollapse
	match.Punctuation = grams13.PunctuationRemove
	match.Limits.MaxInputBytes = 1000
	b, err := grams13.ConfigBytes(match)
	if err != nil {
		t.Fatal(err)
	}
	like, got, err := grams13.ParseIndexConfig(b)
	if err != nil || like != grams13.DefaultLikeConfig() || got != match {
		t.Fatalf("config roundtrip: %#v %#v %v", like, got, err)
	}
	lb, _ := like.Canonical()
	mb, _ := match.Canonical()
	want := `{"version":1,"like":` + string(lb) + `,"match":` + string(mb) + `}`
	if string(b) != want {
		t.Fatalf("unexpected bundle encoding: %s", b)
	}
	i, err := grams13.Method().Configure(b)
	if err != nil {
		t.Fatal(err)
	}
	clear(b)
	if string(i.Canonical()) != want {
		t.Fatal("Configure retained a mutable canonical alias")
	}
	returned := i.Canonical()
	clear(returned)
	if string(i.Canonical()) != want {
		t.Fatal("Canonical returned mutable internal storage")
	}
	got.CaseFold = grams13.CaseFoldNone
	_, parsed, err := grams13.ParseIndexConfig(i.Canonical())
	if err != nil || parsed != match {
		t.Fatal("returned configuration mutated the method")
	}
}

func TestGramsMethodStrictConfig(t *testing.T) {
	b, err := grams13.ConfigBytes(grams13.DefaultMatchConfig())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{
		"", "null", "{}", "[]", "true", "\xff", strings.Repeat(" ", 8193),
		" " + s, s + "\n", s + "{}", s[:len(s)-1],
		strings.Replace(s, `"version":1`, `"version":2`, 1),
		strings.Replace(s, `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(s, `"version":1,`, "", 1),
		strings.Replace(s, `"version":1`, `"version":1,"unknown":0`, 1),
		strings.Replace(s, `"version":1`, `"Version":1`, 1),
		strings.Replace(s, `"version":1`, `"version":1.0`, 1),
		strings.Replace(s, `"version":1`, `"version":null`, 1),
		strings.Replace(s, `"like":`, `"like":null,"like":`, 1),
		strings.Replace(s, `"max_input_bytes":1048576`, `"max_input_bytes":1000`, 1),
		strings.Replace(s, `"namespace":"MATCH_CONFIG_V1"`, `"namespace":"MATCH_CONFIG_V2"`, 1),
		strings.Replace(s, `"namespace":"LIKE_RAW_V1"`, `"namespace":"LIKE_RAW_V2"`, 1),
		strings.Replace(s, `"17.0.0"`, `"16.0.0"`, 1),
		strings.Replace(s, `"max_input_bytes":1048576`, `"max_input_bytes":1048576,"max_input_bytes":1048576`, 1),
	} {
		like, match, err := grams13.ParseIndexConfig([]byte(bad))
		if !errors.Is(err, grams13.ErrConfig) || like != (grams13.Config{}) || match != (grams13.Config{}) {
			t.Fatalf("accepted malformed bundle %q: %v", bad, err)
		}
		if index, err := grams13.Method().Configure([]byte(bad)); index != nil || !errors.Is(err, grams13.ErrConfig) {
			t.Fatalf("Configure accepted malformed bundle %q: %v", bad, err)
		}
	}
	for _, bad := range []grams13.Config{{}, grams13.DefaultLikeConfig()} {
		if b, err := grams13.ConfigBytes(bad); b != nil || !errors.Is(err, grams13.ErrConfig) {
			t.Fatalf("accepted invalid MATCH configuration: %s %v", b, err)
		}
	}
	var bundle map[string]json.RawMessage
	if err := json.Unmarshal(b, &bundle); err != nil {
		t.Fatal(err)
	}
	reordered := []byte(`{"match":` + string(bundle["match"]) + `,"like":` + string(bundle["like"]) + `,"version":1}`)
	if _, _, err := grams13.ParseIndexConfig(reordered); !errors.Is(err, grams13.ErrConfig) {
		t.Fatal("accepted reordered bundle")
	}
}

func TestGramsMethodNamespaceKeys(t *testing.T) {
	i := configuredGrams(t, grams13.DefaultMatchConfig())
	keys, err := i.Keys(context.Background(), "AáA")
	if err != nil {
		t.Fatal(err)
	}
	want := []registry.Key{
		{Namespace: "LIKE_RAW_V1", Length: 1, Text: "a"},
		{Namespace: "LIKE_RAW_V1", Length: 1, Text: "á"},
		{Namespace: "LIKE_RAW_V1", Length: 2, Text: "aá"},
		{Namespace: "LIKE_RAW_V1", Length: 2, Text: "áa"},
		{Namespace: "LIKE_RAW_V1", Length: 3, Text: "aáa"},
		{Namespace: "MATCH_CONFIG_V1", Length: 1, Text: "A"},
		{Namespace: "MATCH_CONFIG_V1", Length: 1, Text: "á"},
		{Namespace: "MATCH_CONFIG_V1", Length: 2, Text: "Aá"},
		{Namespace: "MATCH_CONFIG_V1", Length: 2, Text: "áA"},
		{Namespace: "MATCH_CONFIG_V1", Length: 3, Text: "AáA"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
	keys[0].Text = "corrupted by caller"
	keys, err = i.Keys(context.Background(), "AáA")
	if err != nil || !reflect.DeepEqual(keys, want) {
		t.Fatal("returned keys alias reusable method state")
	}
	keys, err = i.Keys(context.Background(), "e\u0301\x00X")
	if err != nil {
		t.Fatal(err)
	}
	want = []registry.Key{
		{Namespace: "LIKE_RAW_V1", Length: 1, Text: "e"},
		{Namespace: "LIKE_RAW_V1", Length: 1, Text: "\u0301"},
		{Namespace: "LIKE_RAW_V1", Length: 2, Text: "e\u0301"},
		{Namespace: "MATCH_CONFIG_V1", Length: 1, Text: "\x00"},
		{Namespace: "MATCH_CONFIG_V1", Length: 1, Text: "X"},
		{Namespace: "MATCH_CONFIG_V1", Length: 1, Text: "é"},
		{Namespace: "MATCH_CONFIG_V1", Length: 2, Text: "\x00X"},
		{Namespace: "MATCH_CONFIG_V1", Length: 2, Text: "é\x00"},
		{Namespace: "MATCH_CONFIG_V1", Length: 3, Text: "é\x00X"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("decomposed/NUL namespace semantics: %#v", keys)
	}
	keys, err = i.Keys(context.Background(), "")
	if err != nil || len(keys) != 0 {
		t.Fatalf("empty keys: %v %v", keys, err)
	}
}

func TestGramsMethodRejectsPartialKeys(t *testing.T) {
	i := configuredGrams(t, grams13.DefaultMatchConfig())
	if keys, err := i.Keys(context.Background(), "ok\x00\xff"); keys != nil || !errors.Is(err, grams13.ErrInvalidUTF8) {
		t.Fatalf("accepted invalid UTF-8 after NUL: %v %v", keys, err)
	}
	if keys, err := i.Keys(nil, "abc"); keys != nil || !errors.Is(err, grams13.ErrContext) {
		t.Fatalf("nil context: %v %v", keys, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if keys, err := i.Keys(ctx, "abc"); keys != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v %v", keys, err)
	}
	match := grams13.DefaultMatchConfig()
	match.Limits.MaxDistinctTokens = 1
	i = configuredGrams(t, match)
	if keys, err := i.Keys(context.Background(), "ab"); keys != nil || !errors.Is(err, grams13.ErrLimit) {
		t.Fatalf("returned raw LIKE keys when MATCH exceeded its limit: %v %v", keys, err)
	}
	i = configuredGrams(t, grams13.DefaultMatchConfig())
	if keys, err := i.Keys(context.Background(), "e"+strings.Repeat("\u0301", 31)); keys != nil || !errors.Is(err, grams13.ErrStreamSafe) {
		t.Fatalf("returned raw LIKE keys when MATCH rejected stream safety: %v %v", keys, err)
	}
}

func TestGramsMethodConcurrentImmutability(t *testing.T) {
	match := grams13.DefaultMatchConfig()
	match.CaseFold = grams13.CaseFoldUnicode
	i := configuredGrams(t, match)
	input := "Straße e\u0301 한글 カナ 数据库 👩‍🔬"
	want, err := i.Keys(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	wantConfig := i.Canonical()
	var wg sync.WaitGroup
	for n := 0; n < 12; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repeat := 0; repeat < 30; repeat++ {
				keys, err := i.Keys(context.Background(), input)
				if err != nil || !reflect.DeepEqual(keys, want) || !bytes.Equal(i.Canonical(), wantConfig) {
					t.Errorf("concurrent immutable method changed: %v", err)
					return
				}
				clear(keys)
				clear(i.Canonical())
			}
		}()
	}
	wg.Wait()
}
