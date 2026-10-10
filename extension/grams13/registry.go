package grams13

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/registry"
)

const (
	MethodName    = "grams13"
	MethodVersion = 1
)

// indexConfig is a separately versioned canonical bundle. RawMessage preserves
// the tokenizer's canonical encoding, including every data version and limit.
type indexConfig struct {
	Version int             `json:"version"`
	Like    json.RawMessage `json:"like"`
	Match   json.RawMessage `json:"match"`
}

// ConfigBytes returns the canonical v1 method configuration: the unchanged
// DefaultLikeConfig and the selected MATCH_CONFIG_V1 configuration. LIKE and
// MATCH limits apply independently. Changing any setting requires rebuilding
// the index. A LIKE config is not accepted in place of a MATCH config.
func ConfigBytes(match Config) ([]byte, error) {
	if match.Namespace != MatchConfigV1 {
		return nil, fmt.Errorf("%w: expected MATCH_CONFIG_V1", ErrConfig)
	}
	likeBytes, err := DefaultLikeConfig().Canonical()
	if err != nil {
		return nil, err
	}
	matchBytes, err := match.Canonical()
	if err != nil {
		return nil, err
	}
	return json.Marshal(indexConfig{MethodVersion, likeBytes, matchBytes})
}

// ParseIndexConfig accepts only the exact canonical bundle produced by
// ConfigBytes. Unknown versions, duplicate/missing/unknown fields, alternate
// JSON encodings, stale Unicode/dependency versions and modified raw LIKE
// settings fail closed. Returned configs are independent value copies.
func ParseIndexConfig(canonical []byte) (like Config, match Config, err error) {
	bad := func(reason string) (Config, Config, error) {
		return Config{}, Config{}, fmt.Errorf("%w: index configuration: %s", ErrConfig, reason)
	}
	if len(canonical) > 8192 {
		return bad("encoding too large")
	}
	var bundle indexConfig
	d := json.NewDecoder(bytes.NewReader(canonical))
	d.DisallowUnknownFields()
	if err := d.Decode(&bundle); err != nil {
		return bad("invalid JSON: " + err.Error())
	}
	if bundle.Version != MethodVersion {
		return bad("unsupported version")
	}
	like, err = ParseConfig(bundle.Like)
	if err != nil {
		return Config{}, Config{}, err
	}
	match, err = ParseConfig(bundle.Match)
	if err != nil {
		return Config{}, Config{}, err
	}
	if like != DefaultLikeConfig() {
		return bad("expected DefaultLikeConfig")
	}
	expected, err := ConfigBytes(match)
	if err != nil {
		return Config{}, Config{}, err
	}
	if !bytes.Equal(canonical, expected) {
		return bad("noncanonical encoding")
	}
	return like, match, nil
}

type indexMethod struct{}

// Method returns an immutable implementation for an application-owned registry.
// Registration selects real key-generation behavior, not merely a method name.
func Method() registry.IndexMethod { return indexMethod{} }

func (indexMethod) Name() string { return MethodName }
func (indexMethod) Version() int { return MethodVersion }
func (indexMethod) Configure(canonical []byte) (registry.Index, error) {
	like, match, err := ParseIndexConfig(canonical)
	if err != nil {
		return nil, err
	}
	l, err := NewTokenizer(like)
	if err != nil {
		return nil, err
	}
	m, err := NewTokenizer(match)
	if err != nil {
		return nil, err
	}
	return &configuredIndex{canonical: bytes.Clone(canonical), like: l, match: m}, nil
}

type configuredIndex struct {
	canonical []byte
	like      *Tokenizer
	match     *Tokenizer
}

func (i *configuredIndex) Canonical() []byte { return bytes.Clone(i.canonical) }

// Keys keeps LIKE_RAW_V1 and MATCH_CONFIG_V1 independent even when token texts
// coincide. Both namespaces are complete or the call returns no keys. Empty
// normalized input produces no keys in that namespace; empty-needle queries
// must therefore bypass candidate filtering and perform an exact recheck.
func (i *configuredIndex) Keys(ctx context.Context, input string) ([]registry.Key, error) {
	like, err := i.like.Tokenize(ctx, input)
	if err != nil {
		return nil, err
	}
	match, err := i.match.Tokenize(ctx, input)
	if err != nil {
		return nil, err
	}
	keys := make([]registry.Key, 0, len(like)+len(match))
	// Namespace order is LIKE_RAW_V1 before MATCH_CONFIG_V1. Each tokenizer
	// already returns distinct, length/text-sorted tokens; concatenating the
	// two sets therefore yields the registry's complete key order.
	for _, set := range []struct {
		namespace Namespace
		tokens    []Token
	}{{LikeRawV1, like}, {MatchConfigV1, match}} {
		for n, token := range set.tokens {
			if n&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			keys = append(keys, registry.Key{Namespace: string(set.namespace), Length: int(token.Length), Text: token.Text})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}
