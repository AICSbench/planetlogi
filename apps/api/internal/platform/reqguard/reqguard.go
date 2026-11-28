// Package reqguard holds small request-level guards shared by the Phase 2 learning modules.
package reqguard

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidAnswers means a submitted answer map exceeds the accepted bounds.
var ErrInvalidAnswers = errors.New("invalid answers")

// Bounds for a single question's answer map. Answers are persisted per student, so
// unbounded payloads would let one request bloat stored state.
const (
	MaxAnswerParts    = 10
	MaxAnswerKeyLen   = 32
	MaxAnswerValueLen = 64
)

// Token returns the access token from the pl_access cookie, or from an
// "Authorization: Bearer <token>" header (scheme matched case-insensitively).
// Any other Authorization scheme yields no token.
func Token(cookie, authorization string) string {
	if cookie != "" {
		return cookie
	}
	const prefix = "bearer "
	if len(authorization) > len(prefix) && strings.EqualFold(authorization[:len(prefix)], prefix) {
		return strings.TrimSpace(authorization[len(prefix):])
	}
	return ""
}

// ValidateAnswers checks a single question's answers against the bounds above.
func ValidateAnswers(in map[string]string) error {
	if len(in) > MaxAnswerParts {
		return fmt.Errorf("%w: at most %d answer parts", ErrInvalidAnswers, MaxAnswerParts)
	}
	for k, v := range in {
		if k == "" || len(k) > MaxAnswerKeyLen {
			return fmt.Errorf("%w: answer part key must be 1..%d bytes", ErrInvalidAnswers, MaxAnswerKeyLen)
		}
		if len(v) > MaxAnswerValueLen {
			return fmt.Errorf("%w: answer value must be at most %d bytes", ErrInvalidAnswers, MaxAnswerValueLen)
		}
	}
	return nil
}

// ValidateDrafts checks every question's answers in a paper-level drafts map.
func ValidateDrafts(drafts map[int]map[string]string) error {
	for _, ans := range drafts {
		if err := ValidateAnswers(ans); err != nil {
			return err
		}
	}
	return nil
}

// IsUUID reports whether s is a canonical 36-character UUID (8-4-4-4-12 hex). Phase 2 services
// use it to tell real (PostgreSQL-backed) student IDs from demo/test identities.
func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < 36; i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
