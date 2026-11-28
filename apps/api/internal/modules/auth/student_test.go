package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

func TestResolveStudent(t *testing.T) {
	store := &memoryStore{records: map[string]*CredentialRecord{}}
	svc := NewService(config.Config{
		JWTAccessSecret: "test-access", JWTRefreshSecret: "test-refresh",
		JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour,
	}, store, nil, nil)
	for _, tc := range []struct {
		name             string
		role             Role
		grade            int
		timezone, wantTZ string
		token            string
		wantErr          error
		status           int
	}{
		{name: "first grade with default timezone", role: RoleStudent, grade: 1, wantTZ: "Asia/Jakarta"},
		{name: "sixth grade with saved timezone", role: RoleStudent, grade: 6, timezone: "Asia/Makassar", wantTZ: "Asia/Makassar"},
		{name: "missing grade", role: RoleStudent, wantErr: ErrStudentGradeRequired, status: 403},
		{name: "grade above range", role: RoleStudent, grade: 7, wantErr: ErrStudentGradeRequired, status: 403},
		{name: "parent with grade", role: RoleParent, grade: 4, wantErr: ErrForbidden, status: 403},
		{name: "missing token", token: "missing", wantErr: ErrInvalidCredentials, status: 401},
		{name: "invalid token", token: "invalid", wantErr: ErrInvalidCredentials, status: 401},
		{name: "refresh token", token: "refresh", role: RoleStudent, grade: 1, wantErr: ErrInvalidCredentials, status: 401},
		{name: "deleted account", token: "deleted", role: RoleStudent, grade: 1, wantErr: ErrInvalidCredentials, status: 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Principal{ID: "pilot", DisplayName: "Test Pilot", Role: tc.role, Grade: tc.grade, Timezone: tc.timezone}
			store.records[p.ID] = &CredentialRecord{Principal: p}
			tokens, _, _, err := svc.mintTokenPair(p)
			if err != nil {
				t.Fatal(err)
			}
			token := tokens.AccessToken
			switch tc.token {
			case "missing":
				token = ""
			case "invalid":
				token = "invalid"
			case "refresh":
				token = tokens.RefreshToken
			case "deleted":
				delete(store.records, p.ID)
			}
			got, err := svc.ResolveStudent(t.Context(), token)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ResolveStudent error = %v; want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				var httpErr huma.StatusError
				if mapped := StudentAuthHTTPError(err); !errors.As(mapped, &httpErr) || httpErr.GetStatus() != tc.status {
					t.Fatalf("mapped error = %v; want HTTP %d", mapped, tc.status)
				}
				if got != (StudentIdentity{}) {
					t.Fatalf("denied request returned identity %+v", got)
				}
			} else if want := (StudentIdentity{ID: p.ID, DisplayName: p.DisplayName, Grade: p.Grade, Timezone: tc.wantTZ}); got != want {
				t.Fatalf("ResolveStudent = %+v; want %+v", got, want)
			}
		})
	}
}
