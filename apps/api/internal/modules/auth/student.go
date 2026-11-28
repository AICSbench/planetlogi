package auth

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
)

// StudentIdentity is the trusted, server-side student identity used by the Phase 3
// social modules (Star Duels, Super Nova, Planet Stories, Astro Buddy, Events).
// Grade and timezone always come from the authenticated principal, never from
// client query strings, so grade isolation stays authoritative (AGENTS.md §3).
type StudentIdentity struct {
	ID          string
	DisplayName string
	Grade       int
	Timezone    string
}

// ErrStudentGradeRequired is returned when an authenticated student principal has
// no usable grade (1..6); Phase 3 modules scope all state by grade.
var ErrStudentGradeRequired = errors.New("student grade is required")

// DefaultTimezone is the product default (PlanetLogi launches in Indonesia).
const DefaultTimezone = "Asia/Jakarta"

// demoIdentity is used only when no auth service is wired (unit tests). Production
// servers always receive an auth service, so live traffic never hits this fallback.
func demoIdentity() StudentIdentity {
	return StudentIdentity{ID: "demo-pilot1", DisplayName: "Raka Pratama", Grade: 1, Timezone: DefaultTimezone}
}

// ResolveStudent resolves the caller's access token to a STUDENT principal.
// A missing or invalid token yields ErrInvalidCredentials (HTTP 401); an
// authenticated non-student (parent, teacher, admin) yields ErrForbidden
// (HTTP 403) because Phase 3 state is student-owned and grade-scoped.
func (s *Service) ResolveStudent(ctx context.Context, token string) (StudentIdentity, error) {
	if s == nil {
		return demoIdentity(), nil
	}
	if token == "" {
		return StudentIdentity{}, ErrInvalidCredentials
	}
	p, err := s.ResolvePrincipal(ctx, token)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return StudentIdentity{}, ErrInvalidCredentials
		}
		return StudentIdentity{}, err
	}
	if p.Role != RoleStudent {
		return StudentIdentity{}, ErrForbidden
	}
	if p.Grade < 1 || p.Grade > 6 {
		return StudentIdentity{}, ErrStudentGradeRequired
	}
	tz := p.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}
	return StudentIdentity{ID: p.ID, DisplayName: p.DisplayName, Grade: p.Grade, Timezone: tz}, nil
}

// StudentAuthHTTPError maps strict student-auth errors to HTTP responses. Pass
// through anything else so callers can map their own service errors.
func StudentAuthHTTPError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return huma.Error401Unauthorized("authentication required")
	case errors.Is(err, ErrForbidden):
		return huma.Error403Forbidden("student account required")
	case errors.Is(err, ErrStudentGradeRequired):
		return huma.Error403Forbidden(err.Error())
	default:
		return err
	}
}
