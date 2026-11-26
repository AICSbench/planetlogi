package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

const (
	AccessCookieName  = "pl_access"
	RefreshCookieName = "pl_refresh"
)

type ctxKey struct{}

type Handler struct {
	svc *Service
	cfg config.Config
}

func NewHandler(svc *Service, cfg config.Config) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

type loginInput struct {
	Body struct {
		Identifier string `json:"identifier" minLength:"1" doc:"Student Login ID or Parent Email"`
		Password   string `json:"password" minLength:"1" doc:"Account password"`
	}
}

type authOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      struct {
		User Principal `json:"user"`
	}
}

type meInput struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
}

type refreshInput struct {
	CookieRefresh string `cookie:"pl_refresh"`
}

type logoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      struct {
		OK bool `json:"ok"`
	}
}

type otpRequestOutput struct {
	Body struct {
		Sent    bool   `json:"sent"`
		DevCode string `json:"devCode,omitempty" doc:"Populated only when OTP_DEV_ECHO=true in non-production environments"`
	}
}

type otpVerifyInput struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Body          struct {
		Code string `json:"code" minLength:"6" maxLength:"6"`
	}
}

type otpVerifyOutput struct {
	Body struct {
		Verified bool `json:"verified"`
	}
}

type liveFeedOutput struct {
	Body struct {
		Items []LiveFeedItem `json:"items"`
	}
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Authenticate a Student, Parent, Teacher, or Admin account",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, in *loginInput) (*authOutput, error) {
		principal, pair, err := h.svc.Login(ctx, in.Body.Identifier, in.Body.Password)
		if err != nil {
			return nil, huma.Error401Unauthorized("invalid login credentials")
		}
		out := &authOutput{SetCookie: h.buildAuthCookies(pair)}
		out.Body.User = *principal
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-refresh",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/refresh",
		Summary:     "Rotate access and refresh cookies",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, in *refreshInput) (*authOutput, error) {
		if in.CookieRefresh == "" {
			return nil, huma.Error401Unauthorized("missing refresh cookie")
		}
		principal, pair, err := h.svc.Refresh(ctx, in.CookieRefresh)
		if err != nil {
			return nil, huma.Error401Unauthorized("invalid refresh token")
		}
		out := &authOutput{SetCookie: h.buildAuthCookies(pair)}
		out.Body.User = *principal
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-logout",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/logout",
		Summary:     "Clear session cookies",
		Tags:        []string{"Auth"},
	}, func(_ context.Context, _ *struct{}) (*logoutOutput, error) {
		out := &logoutOutput{SetCookie: h.clearAuthCookies()}
		out.Body.OK = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-me",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me",
		Summary:     "Return current authenticated profile",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, in *meInput) (*authOutput, error) {
		p, err := h.authenticateToken(ctx, in.CookieAccess, in.Authorization)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		out := &authOutput{}
		out.Body.User = *p
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-otp-request",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/otp/request",
		Summary:     "Request a 6-digit Parent OTP security code",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, in *meInput) (*otpRequestOutput, error) {
		p, err := h.authenticateToken(ctx, in.CookieAccess, in.Authorization)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		code, err := h.svc.RequestParentOTP(ctx, *p)
		if err != nil {
			if errors.Is(err, ErrForbidden) {
				return nil, huma.Error403Forbidden("parent or staff role required")
			}
			return nil, huma.Error500InternalServerError("failed to generate otp")
		}
		out := &otpRequestOutput{}
		out.Body.Sent = true
		if h.cfg.OTPDevEcho && h.cfg.AppEnv != "production" {
			out.Body.DevCode = code
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-otp-verify",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/otp/verify",
		Summary:     "Verify a 6-digit Parent OTP code (rate limited)",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, in *otpVerifyInput) (*otpVerifyOutput, error) {
		p, err := h.authenticateToken(ctx, in.CookieAccess, in.Authorization)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		if err := h.svc.VerifyParentOTP(ctx, *p, in.Body.Code); err != nil {
			switch {
			case errors.Is(err, ErrForbidden):
				return nil, huma.Error403Forbidden("parent or staff role required")
			case errors.Is(err, ErrOTPRateLimited):
				return nil, huma.Error429TooManyRequests("too many otp attempts; try again in 15 minutes")
			default:
				return nil, huma.Error400BadRequest("invalid or expired otp code")
			}
		}
		out := &otpVerifyOutput{}
		out.Body.Verified = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-live-feed",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/live-feed",
		Summary:     "Anonymized recent Star Points submissions for the login page ticker",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, _ *struct{}) (*liveFeedOutput, error) {
		items, err := h.svc.LiveFeed(ctx, 10)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load live feed")
		}
		out := &liveFeedOutput{}
		out.Body.Items = items
		return out, nil
	})
}

func (h *Handler) authenticateToken(ctx context.Context, cookieTok, authHeader string) (*Principal, error) {
	tok := cookieTok
	if tok == "" && strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		tok = strings.TrimSpace(authHeader[7:])
	}
	if tok == "" {
		return nil, ErrInvalidCredentials
	}
	return h.svc.ResolvePrincipal(ctx, tok)
}

func (h *Handler) buildAuthCookies(pair TokenPair) []http.Cookie {
	return []http.Cookie{
		{
			Name:     AccessCookieName,
			Value:    pair.AccessToken,
			Path:     "/",
			Domain:   h.cfg.CookieDomain,
			MaxAge:   int(h.cfg.JWTAccessTTL.Seconds()),
			HttpOnly: true,
			Secure:   h.cfg.CookieSecure,
			SameSite: http.SameSiteLaxMode,
		},
		{
			Name:     RefreshCookieName,
			Value:    pair.RefreshToken,
			Path:     "/api/v1/auth",
			Domain:   h.cfg.CookieDomain,
			MaxAge:   int(h.cfg.JWTRefreshTTL.Seconds()),
			HttpOnly: true,
			Secure:   h.cfg.CookieSecure,
			SameSite: http.SameSiteLaxMode,
		},
	}
}

func (h *Handler) clearAuthCookies() []http.Cookie {
	past := time.Unix(0, 0)
	return []http.Cookie{
		{
			Name:     AccessCookieName,
			Value:    "",
			Path:     "/",
			Domain:   h.cfg.CookieDomain,
			Expires:  past,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.cfg.CookieSecure,
			SameSite: http.SameSiteLaxMode,
		},
		{
			Name:     RefreshCookieName,
			Value:    "",
			Path:     "/api/v1/auth",
			Domain:   h.cfg.CookieDomain,
			Expires:  past,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.cfg.CookieSecure,
			SameSite: http.SameSiteLaxMode,
		},
	}
}

// PrincipalFromContext retrieves the authenticated principal attached by middleware.
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(*Principal)
	return p, ok
}
