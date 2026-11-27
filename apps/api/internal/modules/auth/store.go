package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound               = errors.New("account not found")
	ErrInvalidCredentials     = errors.New("invalid login credentials")
	ErrForbidden              = errors.New("forbidden role")
	ErrOTPInvalid             = errors.New("invalid or expired otp")
	ErrOTPRateLimited         = errors.New("too many otp attempts")
	ErrLoginRateLimited       = errors.New("too many login attempts")
	ErrOTPDeliveryUnavailable = errors.New("otp delivery provider not configured")
)

type Store interface {
	FindByIdentifier(ctx context.Context, identifier string) (*CredentialRecord, error)
	FindByIDAndRole(ctx context.Context, id string, role Role) (*Principal, error)
	RecentStarPointsFeed(ctx context.Context, limit int) ([]LiveFeedItem, error)
}

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) FindByIdentifier(ctx context.Context, identifier string) (*CredentialRecord, error) {
	if s.pool == nil {
		return nil, ErrNotFound
	}
	idClean := strings.TrimSpace(identifier)
	if idClean == "" {
		return nil, ErrNotFound
	}

	if strings.Contains(idClean, "@") {
		rec, err := s.findUserAccountByEmail(ctx, idClean)
		if err == nil {
			return rec, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	rec, err := s.findStudentByLoginID(ctx, idClean)
	if err == nil {
		return rec, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	rec, err = s.findUserAccountByPhone(ctx, idClean)
	if err == nil {
		return rec, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	return s.findUserAccountByEmail(ctx, idClean)
}

func (s *PGStore) findStudentByLoginID(ctx context.Context, loginID string) (*CredentialRecord, error) {
	var rec CredentialRecord
	err := s.pool.QueryRow(ctx, `
		SELECT sp.id::text, sp.password_hash, sp.first_name, sp.grade, sp.preferred_locale, sp.timezone,
		       sp.avatar_item_id, COALESCE(w.total_sp, 0), COALESCE(w.astro_coins, 0), COALESCE(w.pilot_level, 1),
		       COALESCE(st.current_streak, 0)
		FROM student_profiles sp
		LEFT JOIN student_wallets w ON w.student_id = sp.id
		LEFT JOIN subjects sub ON sub.code = 'math'
		LEFT JOIN student_streaks st ON st.student_id = sp.id AND st.subject_id = sub.id
		WHERE lower(sp.login_id) = lower($1)
		LIMIT 1
	`, loginID).Scan(
		&rec.Principal.ID,
		&rec.PasswordHash,
		&rec.Principal.DisplayName,
		&rec.Principal.Grade,
		&rec.Principal.PreferredLocale,
		&rec.Principal.Timezone,
		&rec.Principal.AvatarItemID,
		&rec.Principal.TotalSP,
		&rec.Principal.AstroCoins,
		&rec.Principal.PilotLevel,
		&rec.Principal.CurrentStreak,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rec.Principal.Role = RoleStudent
	return &rec, nil
}

func (s *PGStore) findUserAccountByEmail(ctx context.Context, email string) (*CredentialRecord, error) {
	var rec CredentialRecord
	var roleStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, role, display_name, password_hash, otp_enabled, preferred_locale, timezone
		FROM user_accounts
		WHERE lower(email) = lower($1)
		LIMIT 1
	`, email).Scan(
		&rec.Principal.ID,
		&roleStr,
		&rec.Principal.DisplayName,
		&rec.PasswordHash,
		&rec.Principal.OTPEnabled,
		&rec.Principal.PreferredLocale,
		&rec.Principal.Timezone,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rec.Principal.Role = Role(roleStr)
	return &rec, nil
}

func (s *PGStore) findUserAccountByPhone(ctx context.Context, phone string) (*CredentialRecord, error) {
	var rec CredentialRecord
	var roleStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, role, display_name, password_hash, otp_enabled, preferred_locale, timezone
		FROM user_accounts
		WHERE phone = $1
		LIMIT 1
	`, phone).Scan(
		&rec.Principal.ID,
		&roleStr,
		&rec.Principal.DisplayName,
		&rec.PasswordHash,
		&rec.Principal.OTPEnabled,
		&rec.Principal.PreferredLocale,
		&rec.Principal.Timezone,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rec.Principal.Role = Role(roleStr)
	return &rec, nil
}

func (s *PGStore) FindByIDAndRole(ctx context.Context, id string, role Role) (*Principal, error) {
	if s.pool == nil {
		return nil, ErrNotFound
	}
	if role == RoleStudent {
		var p Principal
		err := s.pool.QueryRow(ctx, `
			SELECT sp.id::text, sp.first_name, sp.grade, sp.preferred_locale, sp.timezone,
			       sp.avatar_item_id, COALESCE(w.total_sp, 0), COALESCE(w.astro_coins, 0), COALESCE(w.pilot_level, 1),
			       COALESCE(st.current_streak, 0)
			FROM student_profiles sp
			LEFT JOIN student_wallets w ON w.student_id = sp.id
			LEFT JOIN subjects sub ON sub.code = 'math'
			LEFT JOIN student_streaks st ON st.student_id = sp.id AND st.subject_id = sub.id
			WHERE sp.id::text = $1
		`, id).Scan(
			&p.ID, &p.DisplayName, &p.Grade, &p.PreferredLocale, &p.Timezone,
			&p.AvatarItemID, &p.TotalSP, &p.AstroCoins, &p.PilotLevel, &p.CurrentStreak,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		p.Role = RoleStudent
		return &p, nil
	}

	var p Principal
	var roleStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, role, display_name, otp_enabled, preferred_locale, timezone
		FROM user_accounts
		WHERE id::text = $1
	`, id).Scan(&p.ID, &roleStr, &p.DisplayName, &p.OTPEnabled, &p.PreferredLocale, &p.Timezone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Role = Role(roleStr)
	return &p, nil
}

func formatFeedSourceLabel(sourceType string) string {
	switch strings.ToUpper(strings.TrimSpace(sourceType)) {
	case "DAILY_ORBIT":
		return "Daily Orbit"
	case "STAR_DUEL_WIN", "STAR_DUEL_DRAW", "STAR_DUEL_LOSS", "STAR_DUELS":
		return "Star Duels"
	case "SUNDAY_METEOR":
		return "Sunday Meteor"
	case "SUPER_SIGHT", "SUPER_WARP":
		return "Super Nova"
	default:
		return "Daily Orbit"
	}
}

func (s *PGStore) RecentStarPointsFeed(ctx context.Context, limit int) ([]LiveFeedItem, error) {
	if limit <= 0 || limit > 25 {
		limit = 10
	}
	if s.pool == nil {
		return defaultLiveFeed(), nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT ct.id::text, sp.masked_display_name, sp.grade, sp.avatar_item_id,
		       ct.delta, ct.source_type, ct.created_at
		FROM currency_transactions ct
		JOIN student_profiles sp ON sp.id = ct.student_id
		WHERE ct.currency = 'SP' AND ct.delta > 0
		ORDER BY ct.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]LiveFeedItem, 0, limit)
	for rows.Next() {
		var item LiveFeedItem
		var rawSource string
		if err := rows.Scan(&item.ID, &item.MaskedName, &item.Grade, &item.AvatarID, &item.Points, &rawSource, &item.SubmittedAt); err != nil {
			return nil, err
		}
		item.SourceLabel = formatFeedSourceLabel(rawSource)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return defaultLiveFeed(), nil
	}
	return items, nil
}

func defaultLiveFeed() []LiveFeedItem {
	now := time.Now().UTC()
	return []LiveFeedItem{
		{ID: "seed-1", MaskedName: "Raka S.", Grade: 1, AvatarID: "astro_fox", Points: 3, SourceLabel: "Daily Orbit", SubmittedAt: now.Add(-1 * time.Minute)},
		{ID: "seed-2", MaskedName: "Maya K.", Grade: 3, AvatarID: "cosmic_dino", Points: 2, SourceLabel: "Daily Orbit", SubmittedAt: now.Add(-3 * time.Minute)},
		{ID: "seed-3", MaskedName: "Ethan L.", Grade: 5, AvatarID: "mini_bot", Points: 3, SourceLabel: "Star Duels", SubmittedAt: now.Add(-5 * time.Minute)},
		{ID: "seed-4", MaskedName: "Siti R.", Grade: 2, AvatarID: "star_owl", Points: 2, SourceLabel: "Daily Orbit", SubmittedAt: now.Add(-8 * time.Minute)},
		{ID: "seed-5", MaskedName: "Lucas W.", Grade: 6, AvatarID: "mecha_pilot", Points: 3, SourceLabel: "Daily Orbit", SubmittedAt: now.Add(-11 * time.Minute)},
	}
}
