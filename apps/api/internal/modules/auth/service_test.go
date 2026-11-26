package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/server"
)

type memoryStore struct {
	records map[string]*CredentialRecord
}

func (m *memoryStore) FindByIdentifier(_ context.Context, id string) (*CredentialRecord, error) {
	r, ok := m.records[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (m *memoryStore) FindByIDAndRole(_ context.Context, id string, role Role) (*Principal, error) {
	for _, r := range m.records {
		if r.Principal.ID == id && r.Principal.Role == role {
			p := r.Principal
			return &p, nil
		}
	}
	return nil, ErrNotFound
}

func (m *memoryStore) RecentStarPointsFeed(_ context.Context, _ int) ([]LiveFeedItem, error) {
	return defaultLiveFeed(), nil
}

func TestAuthLoginRBACAndOTP(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("planet123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{
		records: map[string]*CredentialRecord{
			"pilot1": {
				PasswordHash: string(hash),
				Principal: Principal{
					ID: "stu-1", Role: RoleStudent, DisplayName: "Raka", Grade: 1, PreferredLocale: "id", Timezone: "Asia/Jakarta",
				},
			},
			"parent@planetlogi.local": {
				PasswordHash: string(hash),
				Principal: Principal{
					ID: "par-1", Role: RoleParent, DisplayName: "Budi", PreferredLocale: "id", Timezone: "Asia/Jakarta", OTPEnabled: true,
				},
			},
		},
	}

	cfg := config.Load()
	svc := NewService(cfg, store, nil, nil)
	app := server.New(server.Dependencies{
		Config:     cfg,
		Registrars: []server.ModuleRegistrar{NewHandler(svc, cfg).RegisterRoutes},
	})

	// 1. Student login succeeds and sets HttpOnly cookies
	body, _ := json.Marshal(map[string]string{"identifier": "pilot1", "password": "planet123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on student login, got %d: %s", rec.Code, rec.Body.String())
	}
	studentCookies := rec.Result().Cookies()
	if len(studentCookies) < 2 {
		t.Fatalf("expected access + refresh cookies, got %d", len(studentCookies))
	}

	// 2. Student MUST be blocked (403) from requesting Parent OTP
	otpReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", nil)
	for _, c := range studentCookies {
		otpReq.AddCookie(c)
	}
	otpRec := httptest.NewRecorder()
	app.Router.ServeHTTP(otpRec, otpReq)
	if otpRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for student calling parent OTP, got %d", otpRec.Code)
	}

	// 3. Parent login + OTP request + wrong attempts lockout + fresh valid OTP verification
	pBody, _ := json.Marshal(map[string]string{"identifier": "parent@planetlogi.local", "password": "planet123"})
	pReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(pBody))
	pReq.Header.Set("Content-Type", "application/json")
	pRec := httptest.NewRecorder()
	app.Router.ServeHTTP(pRec, pReq)
	if pRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on parent login, got %d", pRec.Code)
	}
	parentCookies := pRec.Result().Cookies()

	pOtpReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", nil)
	for _, c := range parentCookies {
		pOtpReq.AddCookie(c)
	}
	pOtpRec := httptest.NewRecorder()
	app.Router.ServeHTTP(pOtpRec, pOtpReq)
	if pOtpRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on parent OTP request, got %d: %s", pOtpRec.Code, pOtpRec.Body.String())
	}
	var otpPayload struct {
		Sent    bool   `json:"sent"`
		DevCode string `json:"devCode"`
	}
	_ = json.Unmarshal(pOtpRec.Body.Bytes(), &otpPayload)
	if len(otpPayload.DevCode) != 6 {
		t.Fatalf("expected 6-digit devCode, got %q", otpPayload.DevCode)
	}

	// Verify valid code succeeds
	vBody, _ := json.Marshal(map[string]string{"code": otpPayload.DevCode})
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(vBody))
	vReq.Header.Set("Content-Type", "application/json")
	for _, c := range parentCookies {
		vReq.AddCookie(c)
	}
	vRec := httptest.NewRecorder()
	app.Router.ServeHTTP(vRec, vReq)
	if vRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid OTP verify, got %d: %s", vRec.Code, vRec.Body.String())
	}

	// 4. Rate limit after 5 failed attempts
	_, _ = svc.RequestParentOTP(context.Background(), store.records["parent@planetlogi.local"].Principal)
	for i := 0; i < 5; i++ {
		if err := svc.VerifyParentOTP(context.Background(), store.records["parent@planetlogi.local"].Principal, "000000"); err == nil {
			t.Fatalf("expected error on wrong otp")
		}
	}
	if err := svc.VerifyParentOTP(context.Background(), store.records["parent@planetlogi.local"].Principal, "000000"); err != ErrOTPRateLimited {
		t.Fatalf("expected ErrOTPRateLimited on 6th attempt, got %v", err)
	}
}
