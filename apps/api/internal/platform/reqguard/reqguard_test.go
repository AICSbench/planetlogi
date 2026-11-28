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

func TestIsUUID(t *testing.T) {
	for s, want := range map[string]bool{
		"6a82b3f8-6104-433d-9a04-930c70a73242": true,
		"6A82B3F8-6104-433D-9A04-930C70A73242": true,
		"demo-pilot1":                          false,
		"6a82b3f8-6104-433d-9a04-930c70a7324":  false,
		"6a82b3f8x6104-433d-9a04-930c70a73242": false,
		"6a82b3f8-6104-433d-9a04-930c70a7324g": false,
	} {
		if got := IsUUID(s); got != want {
			t.Fatalf("IsUUID(%q) = %v, want %v", s, got, want)
		}
	}
}
