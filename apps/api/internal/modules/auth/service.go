package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
)

const (
	maxOTPAttempts   = 5
	maxLoginAttempts = 10
	otpTTL           = 5 * time.Minute
	otpWindow        = 15 * time.Minute
	loginWindow      = 15 * time.Minute
)

var (
	otpVerifyScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	redis.call("DEL", KEYS[1], KEYS[2])
	return 1
end
return 0
`)
	incrWithWindowScript = redis.NewScript(`
local c = redis.call("INCR", KEYS[1])
if c == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return c
`)
	rotateRefreshScript = redis.NewScript(`
local cur = redis.call("GET", KEYS[1])
if not cur or cur == "" then
	return 0
end
redis.call("DEL", KEYS[1])
redis.call("PSETEX", KEYS[2], ARGV[1], ARGV[2])
return 1
`)
)

// OTPDeliverer dispatches 6-digit Parent OTP challenges via email/SMS/WhatsApp in production.
type OTPDeliverer interface {
	SendOTP(ctx context.Context, principal Principal, code string) error
}

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
	code        string
	expiresAt   time.Time
	attempts    int
	windowEnds  time.Time
	lockedUntil time.Time
}

type loginAttemptEntry struct {
	count      int
	windowEnds time.Time
}

type Service struct {
	cfg       config.Config
	store     Store
	rdb       *redis.Client
	logger    *slog.Logger
	deliverer OTPDeliverer

	mu             sync.Mutex
	memOTPs        map[string]*otpMemoryEntry
	memRefreshJTIs map[string]time.Time
	memLoginTries  map[string]*loginAttemptEntry
	nowClock       func() time.Time
}

func NewService(cfg config.Config, store Store, rdb *redis.Client, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		cfg:            cfg,
		store:          store,
		rdb:            rdb,
		logger:         logger,
		memOTPs:        make(map[string]*otpMemoryEntry),
		memRefreshJTIs: make(map[string]time.Time),
		memLoginTries:  make(map[string]*loginAttemptEntry),
		nowClock:       time.Now,
	}
}

func (s *Service) SetOTPDeliverer(d OTPDeliverer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliverer = d
}

func randomHexID(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *Service) checkAndRecordLoginAttempt(ctx context.Context, key string) error {
	if s.rdb != nil {
		rKey := "auth:login_attempts:" + key
		count, err := incrWithWindowScript.Run(ctx, s.rdb, []string{rKey}, loginWindow.Milliseconds()).Int64()
		if err != nil {
			return err
		}
		if count > maxLoginAttempts {
			return ErrLoginRateLimited
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowClock()
	e, ok := s.memLoginTries[key]
	if !ok || now.After(e.windowEnds) {
		e = &loginAttemptEntry{count: 0, windowEnds: now.Add(loginWindow)}
		s.memLoginTries[key] = e
	}
	e.count++
	if e.count > maxLoginAttempts {
		return ErrLoginRateLimited
	}
	return nil
}

func (s *Service) clearLoginAttempts(ctx context.Context, key string) {
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, "auth:login_attempts:"+key).Err()
		return
	}
	s.mu.Lock()
	delete(s.memLoginTries, key)
	s.mu.Unlock()
}

func (s *Service) Login(ctx context.Context, identifier, password string) (*Principal, TokenPair, error) {
	normID := strings.ToLower(strings.TrimSpace(identifier))
	if normID == "" || s.store == nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	if err := s.checkAndRecordLoginAttempt(ctx, normID); err != nil {
		if errors.Is(err, ErrLoginRateLimited) {
			return nil, TokenPair{}, ErrLoginRateLimited
		}
		return nil, TokenPair{}, err
	}

	rec, err := s.store.FindByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, TokenPair{}, ErrInvalidCredentials
		}
		return nil, TokenPair{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(password)); err != nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}

	s.clearLoginAttempts(ctx, normID)
	pair, err := s.issueTokens(ctx, rec.Principal)
	if err != nil {
		return nil, TokenPair{}, err
	}
	return &rec.Principal, pair, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*Principal, TokenPair, error) {
	claims, err := s.ParseToken(refreshToken, "refresh")
	if err != nil || claims.ID == "" {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	if s.store == nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	if err := s.hasRefreshJTI(ctx, claims.ID); err != nil {
		return nil, TokenPair{}, err
	}
	p, err := s.store.FindByIDAndRole(ctx, claims.Subject, claims.Role)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, TokenPair{}, ErrInvalidCredentials
		}
		return nil, TokenPair{}, err
	}
	pair, newJTI, refreshExp, err := s.mintTokenPair(*p)
	if err != nil {
		return nil, TokenPair{}, err
	}
	if err := s.rotateRefreshJTI(ctx, claims.ID, newJTI, p.ID, refreshExp); err != nil {
		return nil, TokenPair{}, err
	}
	return p, pair, nil
}

func (s *Service) RevokeRefreshToken(ctx context.Context, refreshToken string) {
	if refreshToken == "" {
		return
	}
	claims, err := s.ParseToken(refreshToken, "refresh")
	if err != nil || claims.ID == "" {
		return
	}
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, "auth:rt:"+claims.ID).Err()
		return
	}
	s.mu.Lock()
	delete(s.memRefreshJTIs, claims.ID)
	s.mu.Unlock()
}

func (s *Service) hasRefreshJTI(ctx context.Context, jti string) error {
	if s.rdb != nil {
		exists, err := s.rdb.Exists(ctx, "auth:rt:"+jti).Result()
		if err != nil {
			return err
		}
		if exists == 0 {
			return ErrInvalidCredentials
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.memRefreshJTIs[jti]
	if !ok {
		return ErrInvalidCredentials
	}
	if s.nowClock().After(exp) {
		delete(s.memRefreshJTIs, jti)
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) rotateRefreshJTI(ctx context.Context, oldJTI, newJTI, principalID string, refreshExp time.Time) error {
	if s.rdb != nil {
		swapped, err := rotateRefreshScript.Run(
			ctx,
			s.rdb,
			[]string{"auth:rt:" + oldJTI, "auth:rt:" + newJTI},
			s.cfg.JWTRefreshTTL.Milliseconds(),
			principalID,
		).Int64()
		if err != nil {
			return fmt.Errorf("rotate refresh token: %w", err)
		}
		if swapped != 1 {
			return ErrInvalidCredentials
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.memRefreshJTIs[oldJTI]
	if !ok || s.nowClock().After(exp) {
		delete(s.memRefreshJTIs, oldJTI)
		return ErrInvalidCredentials
	}
	delete(s.memRefreshJTIs, oldJTI)
	s.memRefreshJTIs[newJTI] = refreshExp
	return nil
}

func (s *Service) ResolvePrincipal(ctx context.Context, accessToken string) (*Principal, error) {
	claims, err := s.ParseToken(accessToken, "access")
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if s.store == nil {
		return &Principal{ID: claims.Subject, Role: claims.Role}, nil
	}
	p, err := s.store.FindByIDAndRole(ctx, claims.Subject, claims.Role)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) mintTokenPair(p Principal) (TokenPair, string, time.Time, error) {
	now := s.nowClock().UTC()
	jti, err := randomHexID(16)
	if err != nil {
		return TokenPair{}, "", time.Time{}, fmt.Errorf("generate refresh jti: %w", err)
	}
	refreshExp := now.Add(s.cfg.JWTRefreshTTL)

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
			ID:        jti,
			Subject:   p.ID,
			Issuer:    "planetlogi-api",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExp),
		},
	}

	accessTok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		return TokenPair{}, "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	refreshTok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.JWTRefreshSecret))
	if err != nil {
		return TokenPair{}, "", time.Time{}, fmt.Errorf("sign refresh token: %w", err)
	}
	return TokenPair{AccessToken: accessTok, RefreshToken: refreshTok}, jti, refreshExp, nil
}

func (s *Service) issueTokens(ctx context.Context, p Principal) (TokenPair, error) {
	pair, jti, refreshExp, err := s.mintTokenPair(p)
	if err != nil {
		return TokenPair{}, err
	}
	if s.rdb != nil {
		if err := s.rdb.Set(ctx, "auth:rt:"+jti, p.ID, s.cfg.JWTRefreshTTL).Err(); err != nil {
			return TokenPair{}, fmt.Errorf("persist refresh token: %w", err)
		}
	} else {
		s.mu.Lock()
		s.memRefreshJTIs[jti] = refreshExp
		s.mu.Unlock()
	}
	return pair, nil
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
// Re-issuing a code NEVER resets the 15-minute failed-attempt counter.
func (s *Service) RequestParentOTP(ctx context.Context, p Principal) (string, error) {
	if !HasAllowedRole(p.Role, RoleParent, RoleTeacher, RoleSchoolAdmin, RoleAdmin) {
		return "", ErrForbidden
	}

	s.mu.Lock()
	deliverer := s.deliverer
	s.mu.Unlock()

	isDevEchoAllowed := s.cfg.OTPDevEcho && !strings.EqualFold(s.cfg.AppEnv, "production")
	if deliverer == nil && !isDevEchoAllowed {
		return "", ErrOTPDeliveryUnavailable
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	code := fmt.Sprintf("%06d", n.Int64())

	if s.rdb != nil {
		key := "otp:code:" + p.ID
		if err := s.rdb.Set(ctx, key, code, otpTTL).Err(); err != nil {
			return "", fmt.Errorf("store otp in redis: %w", err)
		}
	} else {
		s.mu.Lock()
		now := s.nowClock()
		existing, ok := s.memOTPs[p.ID]
		if !ok {
			existing = &otpMemoryEntry{}
			s.memOTPs[p.ID] = existing
		}
		existing.code = code
		existing.expiresAt = now.Add(otpTTL)
		s.mu.Unlock()
	}

	if deliverer != nil {
		if err := deliverer.SendOTP(ctx, p, code); err != nil {
			return "", fmt.Errorf("deliver otp: %w", err)
		}
	}
	return code, nil
}

// VerifyParentOTP checks a 6-digit OTP and enforces max 5 attempts per 15-minute window.
func (s *Service) VerifyParentOTP(ctx context.Context, p Principal, submittedCode string) error {
	if !HasAllowedRole(p.Role, RoleParent, RoleTeacher, RoleSchoolAdmin, RoleAdmin) {
		return ErrForbidden
	}

	if s.rdb != nil {
		attemptKey := "otp:attempts:" + p.ID
		codeKey := "otp:code:" + p.ID
		attempts, err := incrWithWindowScript.Run(ctx, s.rdb, []string{attemptKey}, otpWindow.Milliseconds()).Int64()
		if err != nil {
			return fmt.Errorf("check otp attempts: %w", err)
		}
		if attempts > maxOTPAttempts {
			return ErrOTPRateLimited
		}
		matched, err := otpVerifyScript.Run(ctx, s.rdb, []string{codeKey, attemptKey}, submittedCode).Int64()
		if err != nil {
			return fmt.Errorf("verify otp in redis: %w", err)
		}
		if matched != 1 {
			return ErrOTPInvalid
		}
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.memOTPs[p.ID]
	now := s.nowClock()
	if !ok {
		return ErrOTPInvalid
	}
	// Reset expired lockout / attempt window before checking code expiry
	if !entry.lockedUntil.IsZero() {
		if now.Before(entry.lockedUntil) {
			return ErrOTPRateLimited
		}
		entry.lockedUntil = time.Time{}
		entry.attempts = 0
		entry.windowEnds = time.Time{}
	}
	if !entry.windowEnds.IsZero() && now.After(entry.windowEnds) {
		entry.attempts = 0
		entry.windowEnds = time.Time{}
	}
	if entry.code == "" || now.After(entry.expiresAt) {
		return ErrOTPInvalid
	}
	if entry.windowEnds.IsZero() {
		entry.windowEnds = now.Add(otpWindow)
	}
	entry.attempts++
	if entry.attempts > maxOTPAttempts {
		entry.lockedUntil = now.Add(otpWindow)
		return ErrOTPRateLimited
	}
	if subtle.ConstantTimeCompare([]byte(entry.code), []byte(submittedCode)) != 1 {
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
