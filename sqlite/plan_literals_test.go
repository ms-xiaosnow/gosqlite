package sqlite

import (
	"math/rand"
	"strings"
	"testing"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Pure-memory proof exercise: each extracted token is necessary under the
// evaluator's exact escaped wildcard, ASCII-fold and first-NUL semantics.
func TestLikeLiteralNecessaryGramProof(t *testing.T) {
	rng := rand.New(rand.NewSource(130054))
	alphabet := []rune("aABéÉé数据库_%\\界🙂\x00")
	for i := 0; i < 12000; i++ {
		valueRunes := make([]rune, rng.Intn(18))
		for j := range valueRunes {
			valueRunes[j] = alphabet[rng.Intn(len(alphabet))]
		}
		value := string(valueRunes)
		escape := []rune{'\\', '界', '%', '_'}[i%4]
		var pattern string
		if i%2 == 0 && len(valueRunes) > 0 {
			start := rng.Intn(len(valueRunes))
			length := 1 + rng.Intn(len(valueRunes)-start)
			var literal strings.Builder
			for _, r := range valueRunes[start : start+length] {
				if r == escape || r == '%' || r == '_' {
					literal.WriteRune(escape)
				}
				literal.WriteRune(r)
			}
			pattern = "%" + literal.String() + "%"
		} else {
			rr := make([]rune, rng.Intn(18))
			for j := range rr {
				rr[j] = alphabet[rng.Intn(len(alphabet))]
			}
			pattern = string(rr)
		}
		like := &sqlast.Like{Pattern: &sqlast.Literal{Value: pattern}, Escape: &sqlast.Literal{Value: string(escape)}}
		literals, _, ok := constantLikePattern(like, nil)
		if !ok || !sqlast.LikeMatch(value, pattern, &escape) {
			continue
		}
		if n := strings.IndexByte(value, 0); n >= 0 {
			value = value[:n]
		}
		value = fold(value)
		for _, tok := range literalTokens(literals.runs) {
			if !strings.Contains(value, tok.Text) {
				t.Fatalf("false-negative filter value=%q pattern=%q escape=%q token=%+v", value, pattern, escape, tok)
			}
		}
	}
}
