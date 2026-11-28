// Package reqguard holds the request-level guards shared by the Phase 2 learning
// modules: credential binding + principal resolution, and bounds on learner answers.
package reqguard

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
)

// DemoStudentID is the read-only guest identity used when a request carries no credentials.
const DemoStudentID = "demo-pilot1"

var (
	// ErrUnauthenticated means the request has no usable credentials for an operation that needs them.
	ErrUnauthenticated = errors.New("authentication required")
	// ErrInvalidAnswers means a submitted answer map exceeds the accepted bounds.
	ErrInvalidAnswers = errors.New("invalid answers")
)

// Credentials is embedded into Huma input structs. It MUST stay an exported type:
// Huma skips embedded structs whose type is unexported, which silently drops the
// cookie, Authorization header and locale bindings.
type Credentials struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

// NormalizedLocale returns "id" or "en".
func (c Credentials) NormalizedLocale() string {
	if c.Locale == "id" {
		return "id"
	}
	return "en"
}

func (c Credentials) token() string {
	if c.CookieAccess != "" {
		return c.CookieAccess
	}
	const prefix = "bearer "
	if len(c.Authorization) > len(prefix) && strings.EqualFold(c.Authorization[:len(prefix)], prefix) {
		return strings.TrimSpace(c.Authorization[len(prefix):])
	}
	return ""
}

// PrincipalResolver is satisfied by *auth.Service.
type PrincipalResolver interface {
	ResolvePrincipal(ctx context.Context, accessToken string) (*auth.Principal, error)
}

// Resolve returns the caller's principal.
//
//   - A token that is present but invalid or expired is always rejected (never downgraded to the guest).
//   - With no token, requireAuth=true is rejected; requireAuth=false yields the read-only guest
//     student so anonymous GETs keep working like the Phase 1 modules.
func Resolve(ctx context.Context, r PrincipalResolver, c Credentials, requireAuth bool) (auth.Principal, error) {
	if tok := c.token(); tok != "" {
		if r == nil {
			return auth.Principal{}, ErrUnauthenticated
		}
		p, err := r.ResolvePrincipal(ctx, tok)
		if err != nil || p == nil {
			return auth.Principal{}, ErrUnauthenticated
		}
		return *p, nil
	}
	if requireAuth {
		return auth.Principal{}, ErrUnauthenticated
	}
	return auth.Principal{ID: DemoStudentID, Role: auth.RoleStudent}, nil
}

// Bounds for a single question's answer map.
const (
	MaxAnswerParts    = 10
	MaxAnswerKeyLen   = 32
	MaxAnswerValueLen = 64
)

// CopyAnswers validates a single question's answers against the bounds above and
// returns a defensive copy so callers never retain the request's map.
func CopyAnswers(in map[string]string) (map[string]string, error) {
	if len(in) > MaxAnswerParts {
		return nil, fmt.Errorf("%w: at most %d answer parts", ErrInvalidAnswers, MaxAnswerParts)
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "" || len(k) > MaxAnswerKeyLen {
			return nil, fmt.Errorf("%w: answer part key must be 1..%d bytes", ErrInvalidAnswers, MaxAnswerKeyLen)
		}
		if len(v) > MaxAnswerValueLen {
			return nil, fmt.Errorf("%w: answer value must be at most %d bytes", ErrInvalidAnswers, MaxAnswerValueLen)
		}
		out[k] = v
	}
	return out, nil
}
