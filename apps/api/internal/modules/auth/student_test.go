package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/danielgtaylor/huma/v2"
)

func TestResolveStudent(t *testing.T) {
	ctx := context.Background()
	const id = "pilot-test"
	storeFailure := errors.New("principal store unavailable")
	for _, tc := range []struct {
		name     string
		token    string
		role     Role
		grade    int
		tz       string
		storeErr error
		wantErr  error
		wantTZ   string
		status   int
	}{
		{name: "missing access token", wantErr: ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "invalid access token", token: "invalid", wantErr: ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "refresh is not access", token: "refresh", role: RoleStudent, grade: 2, wantErr: ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "parent rejected", token: "access", role: RoleParent, wantErr: ErrForbidden, status: http.StatusForbidden},
		{name: "missing grade", token: "access", role: RoleStudent, wantErr: ErrStudentGradeRequired, status: http.StatusForbidden},
		{name: "grade above maximum", token: "access", role: RoleStudent, grade: 7, wantErr: ErrStudentGradeRequired, status: http.StatusForbidden},
		{name: "deleted student", token: "access", role: RoleStudent, storeErr: ErrNotFound, wantErr: ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "store failure", token: "access", role: RoleStudent, storeErr: storeFailure, wantErr: storeFailure},
		{name: "default timezone", token: "access", role: RoleStudent, grade: 1, wantTZ: DefaultTimezone},
		{name: "configured timezone", token: "access", role: RoleStudent, grade: 6, tz: "Asia/Makassar", wantTZ: "Asia/Makassar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{
				JWTAccessSecret: "unit-test-access", JWTRefreshSecret: "unit-test-refresh",
				JWTAccessTTL: time.Minute, JWTRefreshTTL: time.Hour,
			}
			principal := Principal{ID: id, Role: tc.role, Grade: tc.grade, DisplayName: "Pilot", Timezone: tc.tz}
			store := &memoryStore{
				records:     map[string]*CredentialRecord{id: {Principal: principal}},
				findByIDErr: tc.storeErr,
			}
			svc := NewService(cfg, store, nil, nil)
			token := tc.token
			if token == "access" || token == "refresh" {
				pair, _, _, err := svc.mintTokenPair(Principal{ID: id, Role: tc.role})
				if err != nil {
					t.Fatal(err)
				}
				if token == "access" {
					token = pair.AccessToken
				} else {
					token = pair.RefreshToken
				}
			}
			got, err := svc.ResolveStudent(ctx, token)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ResolveStudent error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil {
				want := StudentIdentity{ID: id, DisplayName: "Pilot", Grade: tc.grade, Timezone: tc.wantTZ}
				if got != want {
					t.Fatalf("identity = %+v, want %+v", got, want)
				}
				return
			}
			if got != (StudentIdentity{}) {
				t.Fatalf("failed request returned student identity: %+v", got)
			}
			mapped := StudentAuthHTTPError(err)
			if tc.status == 0 {
				if mapped != err {
					t.Fatalf("unrecognized error changed: %v", mapped)
				}
				return
			}
			var statusErr huma.StatusError
			if !errors.As(mapped, &statusErr) || statusErr.GetStatus() != tc.status {
				t.Fatalf("HTTP error = %v, want status %d", mapped, tc.status)
			}
		})
	}
}
