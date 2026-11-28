package reqguard

import (
	"errors"
	"strings"
	"testing"
)

func TestToken(t *testing.T) {
	cases := map[string]struct{ cookie, header, want string }{
		"cookie wins":           {"c", "Bearer h", "c"},
		"bearer header":         {"", "Bearer abc", "abc"},
		"bearer any case":       {"", "bEaReR abc", "abc"},
		"other scheme rejected": {"", "Basic abc", ""},
		"short header":          {"", "Bearer", ""},
		"nothing":               {"", "", ""},
	}
	for name, tc := range cases {
		if got := Token(tc.cookie, tc.header); got != tc.want {
			t.Fatalf("%s: got %q want %q", name, got, tc.want)
		}
	}
}

func TestValidateAnswers(t *testing.T) {
	if err := ValidateAnswers(map[string]string{"answer": "12", "numerator": "3"}); err != nil {
		t.Fatalf("valid answers rejected: %v", err)
	}
	tooMany := map[string]string{}
	for i := 0; i <= MaxAnswerParts; i++ {
		tooMany[strings.Repeat("k", i+1)] = "1"
	}
	for name, in := range map[string]map[string]string{
		"too many parts": tooMany,
		"long value":     {"answer": strings.Repeat("9", MaxAnswerValueLen+1)},
		"long key":       {strings.Repeat("k", MaxAnswerKeyLen+1): "1"},
		"empty key":      {"": "1"},
	} {
		if err := ValidateAnswers(in); !errors.Is(err, ErrInvalidAnswers) {
			t.Fatalf("%s: expected ErrInvalidAnswers, got %v", name, err)
		}
	}
	if err := ValidateDrafts(map[int]map[string]string{1: {"answer": strings.Repeat("9", 1000)}}); !errors.Is(err, ErrInvalidAnswers) {
		t.Fatalf("drafts: expected ErrInvalidAnswers, got %v", err)
	}
}
