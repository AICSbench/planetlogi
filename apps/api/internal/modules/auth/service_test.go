package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/server"
)

type memoryStore struct {
	records     map[string]*CredentialRecord
	findByIDErr error
}

func (m *memoryStore) FindByIdentifier(_ context.Context, id string) (*CredentialRecord, error) {
	r, ok := m.records[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (m *memoryStore) FindByIDAndRole(_ context.Context, id string, role Role) (*Principal, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
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

func TestAuthLoginRBACRefreshRotationAndOTP(t *testing.T) {
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
	cfg.AppEnv = "development"
	cfg.OTPDevEcho = true

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
	var refreshCookie *http.Cookie
	for _, c := range studentCookies {
		if !c.HttpOnly {
			t.Fatalf("expected cookie %s to be HttpOnly", c.Name)
		}
		if c.Name == RefreshCookieName {
			refreshCookie = c
		}
	}
	if refreshCookie == nil {
		t.Fatalf("missing refresh cookie")
	}

	// 2. Refresh token rotates once and rejects replay of consumed token
	refReq1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refReq1.AddCookie(refreshCookie)
	refRec1 := httptest.NewRecorder()
	app.Router.ServeHTTP(refRec1, refReq1)
	if refRec1.Code != http.StatusOK {
		t.Fatalf("expected first refresh to return 200, got %d", refRec1.Code)
	}
	refReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refReq2.AddCookie(refreshCookie)
	refRec2 := httptest.NewRecorder()
	app.Router.ServeHTTP(refRec2, refReq2)
	if refRec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected replay of consumed refresh token to return 401, got %d", refRec2.Code)
	}

	// 3. Cross-Origin CSRF cookie POST is rejected with 403
	csrfReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", nil)
	csrfReq.Header.Set("Origin", "https://evil.example.com")
	for _, c := range studentCookies {
		csrfReq.AddCookie(c)
	}
	csrfRec := httptest.NewRecorder()
	app.Router.ServeHTTP(csrfRec, csrfReq)
	if csrfRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for untrusted Origin cookie POST, got %d", csrfRec.Code)
	}

	// 4. Student MUST be blocked (403) from requesting Parent OTP
	otpReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", nil)
	for _, c := range studentCookies {
		otpReq.AddCookie(c)
	}
	otpRec := httptest.NewRecorder()
	app.Router.ServeHTTP(otpRec, otpReq)
	if otpRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for student calling parent OTP, got %d", otpRec.Code)
	}

	// 5. Parent login + OTP request + valid verification
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

	// 6. Rate limit after 5 failed attempts — AND re-requesting OTP does NOT reset lockout!
	parentPrincipal := store.records["parent@planetlogi.local"].Principal
	_, _ = svc.RequestParentOTP(context.Background(), parentPrincipal)
	for i := 0; i < 5; i++ {
		if err := svc.VerifyParentOTP(context.Background(), parentPrincipal, "000000"); err == nil {
			t.Fatalf("expected error on wrong otp")
		}
	}
	// Re-issue a fresh OTP — attempt budget MUST remain exhausted
	freshCode, _ := svc.RequestParentOTP(context.Background(), parentPrincipal)
	if err := svc.VerifyParentOTP(context.Background(), parentPrincipal, freshCode); err != ErrOTPRateLimited {
		t.Fatalf("expected ErrOTPRateLimited even after OTP reissue, got %v", err)
	}

	// 7. Transient store error during Refresh returns 503 WITHOUT burning the refresh JTI
	store.findByIDErr = errors.New("temporary db outage")
	transientRefReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	for _, c := range parentCookies {
		transientRefReq.AddCookie(c)
	}
	transientRefRec := httptest.NewRecorder()
	app.Router.ServeHTTP(transientRefRec, transientRefReq)
	if transientRefRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on transient DB failure during refresh, got %d", transientRefRec.Code)
	}

	// Once DB recovers, retrying with the SAME refresh cookie succeeds (200)
	store.findByIDErr = nil
	recoveredRefReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	for _, c := range parentCookies {
		recoveredRefReq.AddCookie(c)
	}
	recoveredRefRec := httptest.NewRecorder()
	app.Router.ServeHTTP(recoveredRefRec, recoveredRefReq)
	if recoveredRefRec.Code != http.StatusOK {
		t.Fatalf("expected 200 when retrying refresh after transient DB outage, got %d", recoveredRefRec.Code)
	}
}

func TestAuthCookiesSupportSameSiteNoneForCrossOriginPages(t *testing.T) {
	cfg := config.Load()
	cfg.CookieSecure = true
	cfg.CookieSameSite = "none"
	h := NewHandler(NewService(cfg, nil, nil, nil), cfg)
	cookies := h.buildAuthCookies(TokenPair{AccessToken: "a", RefreshToken: "r"})
	for _, c := range cookies {
		if c.SameSite != http.SameSiteNoneMode || !c.Secure {
			t.Fatalf("expected SameSite=None and Secure=true on cookie %s, got sameSite=%v secure=%v", c.Name, c.SameSite, c.Secure)
		}
	}
}
