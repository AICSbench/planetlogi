package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

const (
	maxOTPAttempts = 5
	otpTTL         = 5 * time.Minute
	otpWindow      = 15 * time.Minute
)

type TokenClaims struct {
	Role      Role   `json:"role"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type otpMemoryEntry struct {
	code      string
	expiresAt time.Time
	attempts  int
	lockedTil time.Time
}

type Service struct {
	cfg    config.Config
	store  Store
	rdb    *redis.Client
	logger *slog.Logger

	mu       sync.Mutex
	memOTPs  map[string]*otpMemoryEntry
	nowClock func() time.Time
}

func NewService(cfg config.Config, store Store, rdb *redis.Client, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		cfg:      cfg,
		store:    store,
		rdb:      rdb,
		logger:   logger,
		memOTPs:  make(map[string]*otpMemoryEntry),
		nowClock: time.Now,
	}
}

func (s *Service) Login(ctx context.Context, identifier, password string) (*Principal, TokenPair, error) {
	if s.store == nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	rec, err := s.store.FindByIdentifier(ctx, identifier)
	if err != nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(password)); err != nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}

	pair, err := s.issueTokens(rec.Principal)
	if err != nil {
		return nil, TokenPair{}, err
	}
	return &rec.Principal, pair, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*Principal, TokenPair, error) {
	claims, err := s.ParseToken(refreshToken, "refresh")
	if err != nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	if s.store == nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	p, err := s.store.FindByIDAndRole(ctx, claims.Subject, claims.Role)
	if err != nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	pair, err := s.issueTokens(*p)
	if err != nil {
		return nil, TokenPair{}, err
	}
	return p, pair, nil
}

func (s *Service) ResolvePrincipal(ctx context.Context, accessToken string) (*Principal, error) {
	claims, err := s.ParseToken(accessToken, "access")
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if s.store == nil {
		return &Principal{ID: claims.Subject, Role: claims.Role}, nil
	}
	return s.store.FindByIDAndRole(ctx, claims.Subject, claims.Role)
}

func (s *Service) issueTokens(p Principal) (TokenPair, error) {
	now := s.nowClock().UTC()
	accessClaims := TokenClaims{
		Role:      p.Role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.ID,
			Issuer:    "planetlogi-api",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTAccessTTL)),
		},
	}
	refreshClaims := TokenClaims{
		Role:      p.Role,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.ID,
			Issuer:    "planetlogi-api",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTRefreshTTL)),
		},
	}

	accessTok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign access token: %w", err)
	}
	refreshTok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.JWTRefreshSecret))
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign refresh token: %w", err)
	}

	return TokenPair{AccessToken: accessTok, RefreshToken: refreshTok}, nil
}

func (s *Service) ParseToken(raw, expectedType string) (*TokenClaims, error) {
	secret := s.cfg.JWTAccessSecret
	if expectedType == "refresh" {
		secret = s.cfg.JWTRefreshSecret
	}
	parsed, err := jwt.ParseWithClaims(raw, &TokenClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidCredentials
	}
	claims, ok := parsed.Claims.(*TokenClaims)
	if !ok || claims.TokenType != expectedType || claims.Subject == "" {
		return nil, ErrInvalidCredentials
	}
	return claims, nil
}

// RequestParentOTP issues a 6-digit OTP restricted strictly to non-student adult roles.
func (s *Service) RequestParentOTP(ctx context.Context, p Principal) (string, error) {
	if !HasAllowedRole(p.Role, RoleParent, RoleTeacher, RoleSchoolAdmin, RoleAdmin) {
		return "", ErrForbidden
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	code := fmt.Sprintf("%06d", n.Int64())

	if s.rdb != nil {
		key := "otp:code:" + p.ID
		if err := s.rdb.Set(ctx, key, code, otpTTL).Err(); err == nil {
			_ = s.rdb.Del(ctx, "otp:attempts:"+p.ID).Err()
			return code, nil
		}
	}

	s.mu.Lock()
	s.memOTPs[p.ID] = &otpMemoryEntry{
		code:      code,
		expiresAt: s.nowClock().Add(otpTTL),
		attempts:  0,
	}
	s.mu.Unlock()
	return code, nil
}

// VerifyParentOTP checks a 6-digit OTP and enforces max 5 attempts per 15-minute window.
func (s *Service) VerifyParentOTP(ctx context.Context, p Principal, submittedCode string) error {
	if !HasAllowedRole(p.Role, RoleParent, RoleTeacher, RoleSchoolAdmin, RoleAdmin) {
		return ErrForbidden
	}

	if s.rdb != nil {
		attemptKey := "otp:attempts:" + p.ID
		attempts, err := s.rdb.Incr(ctx, attemptKey).Result()
		if err == nil {
			if attempts == 1 {
				_ = s.rdb.Expire(ctx, attemptKey, otpWindow).Err()
			}
			if attempts > maxOTPAttempts {
				return ErrOTPRateLimited
			}
			stored, err := s.rdb.Get(ctx, "otp:code:"+p.ID).Result()
			if err != nil || subtle.ConstantTimeCompare([]byte(stored), []byte(submittedCode)) != 1 {
				return ErrOTPInvalid
			}
			_ = s.rdb.Del(ctx, "otp:code:"+p.ID, attemptKey).Err()
			return nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.memOTPs[p.ID]
	now := s.nowClock()
	if !ok {
		return ErrOTPInvalid
	}
	if !entry.lockedTil.IsZero() && now.Before(entry.lockedTil) {
		return ErrOTPRateLimited
	}
	entry.attempts++
	if entry.attempts > maxOTPAttempts {
		entry.lockedTil = now.Add(otpWindow)
		return ErrOTPRateLimited
	}
	if now.After(entry.expiresAt) || subtle.ConstantTimeCompare([]byte(entry.code), []byte(submittedCode)) != 1 {
		return ErrOTPInvalid
	}
	delete(s.memOTPs, p.ID)
	return nil
}

func (s *Service) LiveFeed(ctx context.Context, limit int) ([]LiveFeedItem, error) {
	if s.store == nil {
		return defaultLiveFeed(), nil
	}
	return s.store.RecentStarPointsFeed(ctx, limit)
}

func HasAllowedRole(actual Role, allowed ...Role) bool {
	for _, r := range allowed {
		if actual == r {
			return true
		}
	}
	return false
}
