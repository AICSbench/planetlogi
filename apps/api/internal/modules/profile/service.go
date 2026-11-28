package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/reports"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
)

var (
	ErrOTPRequired     = errors.New("parent OTP verification is required to update protected settings")
	ErrNoLinkedChild   = errors.New("no linked child found for this parent")
	ErrProfileNotFound = errors.New("student profile not found")
	ErrInvalidInput    = errors.New("invalid settings")
	ErrParentOnly      = errors.New("parent access required to change family settings")
)

type ParentProfile struct {
	Name        string `json:"name"`
	MaskedEmail string `json:"maskedEmail"`
	MaskedPhone string `json:"maskedPhone"`
}

type ChildProfile struct {
	StudentID         string `json:"studentId"`
	FirstName         string `json:"firstName"`
	Grade             int    `json:"grade"`
	SchoolName        string `json:"schoolName"`
	NextPromotionDate string `json:"nextPromotionDate"`
	Birthday          string `json:"birthday"`
	Gender            string `json:"gender"`
	AvatarItemID      string `json:"avatarItemId"`
}

type FamilySettings struct {
	OTPEnabled             bool     `json:"otpEnabled"`
	Language               string   `json:"language"`
	DailyOrbitCurricula    []string `json:"dailyOrbitCurricula"`
	StarDuelCurricula      []string `json:"starDuelCurricula"`
	BrainGalaxyDailyCapMin int      `json:"brainGalaxyDailyCapMin"`
	BrainGalaxyWindowStart string   `json:"brainGalaxyWindowStart"`
	BrainGalaxyWindowEnd   string   `json:"brainGalaxyWindowEnd"`
	SubscriptionPlanName   string   `json:"subscriptionPlanName"`
	SubscriptionValidUntil string   `json:"subscriptionValidUntil"`
	BillingEnabled         bool     `json:"billingEnabled"`
}

type ProfileBundle struct {
	Parent   ParentProfile  `json:"parent"`
	Child    ChildProfile   `json:"child"`
	Settings FamilySettings `json:"settings"`
}

type ParentChildOverview struct {
	Child           ChildProfile         `json:"child"`
	StreakDays      int                  `json:"streakDays"`
	CryoShields     int                  `json:"cryoShields"`
	MasteryCount    int                  `json:"masteryCount"`
	CompetentCount  int                  `json:"competentCount"`
	BeginningCount  int                  `json:"beginningCount"`
	IncompleteCount int                  `json:"incompleteCount"`
	FocusSkills     []training.SkillView `json:"focusSkills"`
	StrengthSkills  []training.SkillView `json:"strengthSkills"`
}

type Service struct {
	mu          sync.Mutex
	profiles    map[string]*ProfileBundle
	pool        *pgxpool.Pool
	reportsH    *reports.Handler
	streakSvc   *streak.Service
	authSvc     *auth.Service
	billingFlag bool
}

func NewService(reportsH *reports.Handler, streakSvc *streak.Service, authSvc *auth.Service, billingEnabled bool) *Service {
	return &Service{
		profiles:    make(map[string]*ProfileBundle),
		reportsH:    reportsH,
		streakSvc:   streakSvc,
		authSvc:     authSvc,
		billingFlag: billingEnabled,
	}
}

func NewServiceWithPG(pool *pgxpool.Pool, reportsH *reports.Handler, streakSvc *streak.Service, authSvc *auth.Service, billingEnabled bool) *Service {
	svc := NewService(reportsH, streakSvc, authSvc, billingEnabled)
	svc.pool = pool
	return svc
}

// isUUID reports whether studentID is a real (PostgreSQL-backed) ID.
func isUUID(s string) bool { return reqguard.IsUUID(s) }

func (s *Service) resolveChildID(ctx context.Context, actor auth.Principal, preferred string) (string, error) {
	if actor.Role != auth.RoleParent {
		return actor.ID, nil
	}
	if s.pool != nil {
		if !isUUID(actor.ID) {
			if actor.ID == "demo-parent" && (preferred == "" || preferred == "demo-pilot1") {
				return "demo-pilot1", nil
			}
			return "", ErrNoLinkedChild
		}
		if preferred != "" {
			// The parent explicitly picked a child: only a student linked to this
			// parent's account is acceptable, whatever was requested.
			if !isUUID(preferred) {
				return "", ErrNoLinkedChild
			}
			var child string
			err := s.pool.QueryRow(ctx, `
				SELECT id::text FROM student_profiles
				WHERE id = $1::uuid AND parent_account_id = $2::uuid
			`, preferred, actor.ID).Scan(&child)
			if errors.Is(err, pgx.ErrNoRows) {
				return "", ErrNoLinkedChild
			}
			return child, err
		}
		var child string
		err := s.pool.QueryRow(ctx, `
			SELECT id::text FROM student_profiles
			WHERE parent_account_id = $1::uuid
			ORDER BY created_at, id
			LIMIT 1
		`, actor.ID).Scan(&child)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNoLinkedChild
		}
		return child, err
	}
	if actor.ID != "demo-parent" {
		return "", ErrNoLinkedChild
	}
	if preferred != "" && preferred != "demo-pilot1" {
		return "", ErrNoLinkedChild
	}
	return "demo-pilot1", nil
}

func maskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return ""
	}
	return email[:1] + "***" + email[at:]
}

func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return phone[:len(phone)-4] + "-****-" + phone[len(phone)-2:]
}

func clockMinutes(raw string) string {
	if len(raw) >= 5 {
		return raw[:5]
	}
	return raw
}

func (s *Service) loadBundlePG(ctx context.Context, childID string) (ProfileBundle, error) {
	var b ProfileBundle
	var dob, promo *time.Time
	var gender *string
	var school, winStart, winEnd string
	var dailyRaw, duelRaw []byte
	var capSec int
	var parentID, email, phone string
	var otp bool
	err := s.pool.QueryRow(ctx, `
		SELECT sp.first_name, sp.grade, sp.birthday, sp.gender, sp.avatar_item_id,
		       sp.next_promotion_date, sp.preferred_locale,
		       sp.active_daily_curricula_by_subject, sp.active_duel_curricula_by_subject,
		       sp.brain_galaxy_daily_cap_sec, sp.brain_galaxy_window_start::text,
		       sp.brain_galaxy_window_end::text, COALESCE(sc.name, ''),
		       sp.parent_account_id::text, ua.display_name, COALESCE(ua.email, ''),
		       COALESCE(ua.phone, ''), ua.otp_enabled
		FROM student_profiles sp
		JOIN user_accounts ua ON ua.id = sp.parent_account_id
		LEFT JOIN schools sc ON sc.id = sp.school_id
		WHERE sp.id = $1::uuid
	`, childID).Scan(
		&b.Child.FirstName, &b.Child.Grade, &dob, &gender, &b.Child.AvatarItemID,
		&promo, &b.Settings.Language, &dailyRaw, &duelRaw, &capSec, &winStart, &winEnd,
		&school, &parentID, &b.Parent.Name, &email, &phone, &otp,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProfileBundle{}, ErrProfileNotFound
	}
	if err != nil {
		return ProfileBundle{}, err
	}
	b.Child.StudentID = childID
	b.Child.SchoolName = school
	if dob != nil {
		b.Child.Birthday = dob.Format("2006-01-02")
	}
	if gender != nil {
		b.Child.Gender = *gender
	}
	if promo != nil {
		b.Child.NextPromotionDate = promo.Format("2 Jan 2006")
	}
	b.Parent.MaskedEmail = maskEmail(email)
	b.Parent.MaskedPhone = maskPhone(phone)
	var daily, duel map[string][]string
	_ = json.Unmarshal(dailyRaw, &daily)
	_ = json.Unmarshal(duelRaw, &duel)
	b.Settings.DailyOrbitCurricula = daily["math"]
	b.Settings.StarDuelCurricula = duel["math"]
	b.Settings.BrainGalaxyDailyCapMin = capSec / 60
	b.Settings.BrainGalaxyWindowStart = clockMinutes(winStart)
	b.Settings.BrainGalaxyWindowEnd = clockMinutes(winEnd)
	b.Settings.OTPEnabled = otp
	b.Settings.SubscriptionPlanName = "PlanetLogi Galaxy All-Access (Feature-First)"
	b.Settings.SubscriptionValidUntil = "Open Beta"
	b.Settings.BillingEnabled = s.billingFlag
	_ = parentID
	return b, nil
}

func (s *Service) getOrCreateLocked(studentID string) *ProfileBundle {
	b, ok := s.profiles[studentID]
	if !ok {
		b = &ProfileBundle{
			Parent: ParentProfile{
				Name:        "Budi Santoso",
				MaskedEmail: "b***@planetlogi.local",
				MaskedPhone: "+62 812-****-8901",
			},
			Child: ChildProfile{
				StudentID:         studentID,
				FirstName:         "Raka",
				Grade:             1,
				SchoolName:        "SDN Bintang Galaksi 01",
				NextPromotionDate: "1 Jul 2027",
				Birthday:          "2019-08-14",
				Gender:            "MALE",
				AvatarItemID:      "astro_fox",
			},
			Settings: FamilySettings{
				OTPEnabled:             false,
				Language:               "en",
				DailyOrbitCurricula:    []string{"SINGAPORE_MOE", "HOTS_WHEEL"},
				StarDuelCurricula:      []string{"SINGAPORE_MOE"},
				BrainGalaxyDailyCapMin: 25,
				BrainGalaxyWindowStart: "08:00",
				BrainGalaxyWindowEnd:   "20:00",
				SubscriptionPlanName:   "PlanetLogi Galaxy All-Access (Feature-First)",
				SubscriptionValidUntil: "Open Beta",
				BillingEnabled:         s.billingFlag,
			},
		}
		s.profiles[studentID] = b
	}
	return b
}

func (s *Service) GetBundle(ctx context.Context, actor auth.Principal, childID string) (ProfileBundle, error) {
	childID, err := s.resolveChildID(ctx, actor, childID)
	if err != nil {
		return ProfileBundle{}, err
	}
	if s.pool != nil && isUUID(childID) {
		return s.loadBundlePG(ctx, childID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.getOrCreateLocked(childID), nil
}

type UpdateSettingsInput struct {
	// ChildID selects which linked child the settings apply to; empty means the
	// parent's first linked child. The parent-child link is re-validated server-side.
	ChildID                string   `json:"childId,omitempty"`
	OTPEnabled             *bool    `json:"otpEnabled,omitempty"`
	OTPCode                string   `json:"otpCode,omitempty"`
	Language               string   `json:"language,omitempty"`
	DailyOrbitCurricula    []string `json:"dailyOrbitCurricula,omitempty"`
	StarDuelCurricula      []string `json:"starDuelCurricula,omitempty"`
	BrainGalaxyDailyCapMin *int     `json:"brainGalaxyDailyCapMin,omitempty"`
	BrainGalaxyWindowStart string   `json:"brainGalaxyWindowStart,omitempty"`
	BrainGalaxyWindowEnd   string   `json:"brainGalaxyWindowEnd,omitempty"`
}

const (
	maxCurricula      = 8
	maxCurriculumCode = 32
)

var hhmmPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func validateSettingsInput(in UpdateSettingsInput) error {
	if in.Language != "" && in.Language != "en" && in.Language != "id" {
		return fmt.Errorf("%w: language must be en or id", ErrInvalidInput)
	}
	for _, list := range [][]string{in.DailyOrbitCurricula, in.StarDuelCurricula} {
		if len(list) > maxCurricula {
			return fmt.Errorf("%w: at most %d curricula", ErrInvalidInput, maxCurricula)
		}
		for _, c := range list {
			if c == "" || len(c) > maxCurriculumCode {
				return fmt.Errorf("%w: curriculum code must be 1..%d characters", ErrInvalidInput, maxCurriculumCode)
			}
		}
	}
	for _, t := range []string{in.BrainGalaxyWindowStart, in.BrainGalaxyWindowEnd} {
		if t != "" && !hhmmPattern.MatchString(t) {
			return fmt.Errorf("%w: Brain Galaxy window times must be HH:MM", ErrInvalidInput)
		}
	}
	return nil
}

func (s *Service) UpdateSettings(ctx context.Context, actor auth.Principal, in UpdateSettingsInput) (ProfileBundle, error) {
	if actor.Role != auth.RoleParent {
		return ProfileBundle{}, ErrParentOnly
	}
	if err := validateSettingsInput(in); err != nil {
		return ProfileBundle{}, err
	}
	childID, err := s.resolveChildID(ctx, actor, in.ChildID)
	if err != nil {
		return ProfileBundle{}, err
	}
	var b ProfileBundle
	if s.pool != nil && isUUID(childID) {
		b, err = s.loadBundlePG(ctx, childID)
	} else {
		s.mu.Lock()
		b = *s.getOrCreateLocked(childID)
		s.mu.Unlock()
	}
	if err != nil {
		return ProfileBundle{}, err
	}

	// OTP gate. Turning OTP on also needs a verified code: it proves the parent can actually
	// receive codes, so enabling it can never lock them out of their own settings (e.g. when no
	// delivery provider is configured). Fail closed without a verifier.
	enablingOTP := in.OTPEnabled != nil && *in.OTPEnabled && !b.Settings.OTPEnabled
	otpVerified := false
	if b.Settings.OTPEnabled || enablingOTP {
		if in.OTPCode == "" || s.authSvc == nil {
			return ProfileBundle{}, ErrOTPRequired
		}
		if err := s.authSvc.VerifyParentOTP(ctx, actor, in.OTPCode); err != nil {
			return ProfileBundle{}, ErrOTPRequired
		}
		otpVerified = true
	}

	if in.OTPEnabled != nil {
		b.Settings.OTPEnabled = *in.OTPEnabled
	}
	if in.Language == "en" || in.Language == "id" {
		b.Settings.Language = in.Language
	}
	if len(in.DailyOrbitCurricula) > 0 {
		b.Settings.DailyOrbitCurricula = in.DailyOrbitCurricula
	}
	if len(in.StarDuelCurricula) > 0 {
		b.Settings.StarDuelCurricula = in.StarDuelCurricula
	}
	if in.BrainGalaxyDailyCapMin != nil {
		capMin := *in.BrainGalaxyDailyCapMin
		if capMin < 0 {
			capMin = 0
		}
		if capMin > 25 {
			capMin = 25
		}
		b.Settings.BrainGalaxyDailyCapMin = capMin
	}
	if in.BrainGalaxyWindowStart != "" {
		b.Settings.BrainGalaxyWindowStart = in.BrainGalaxyWindowStart
	}
	if in.BrainGalaxyWindowEnd != "" {
		b.Settings.BrainGalaxyWindowEnd = in.BrainGalaxyWindowEnd
	}
	if s.pool != nil && isUUID(childID) {
		// One transaction that re-reads the OTP flag under row locks: a request authorized while
		// OTP was off cannot commit after another request turned it on, and the child settings
		// and the parent's OTP flag change together or not at all.
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return ProfileBundle{}, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		var parentID string
		var otpNow bool
		if err := tx.QueryRow(ctx, `
			SELECT sp.parent_account_id::text, ua.otp_enabled
			FROM student_profiles sp
			JOIN user_accounts ua ON ua.id = sp.parent_account_id
			WHERE sp.id = $1::uuid
			FOR UPDATE OF sp, ua
		`, childID).Scan(&parentID, &otpNow); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ProfileBundle{}, ErrProfileNotFound
			}
			return ProfileBundle{}, err
		}
		if otpNow && !otpVerified {
			return ProfileBundle{}, ErrOTPRequired
		}
		sets := make([]string, 0, 6)
		args := []any{childID}
		next := 2
		if in.Language == "en" || in.Language == "id" {
			sets = append(sets, fmt.Sprintf("preferred_locale = $%d", next))
			args = append(args, in.Language)
			next++
		}
		if len(in.DailyOrbitCurricula) > 0 {
			daily, _ := json.Marshal(map[string][]string{"math": in.DailyOrbitCurricula})
			sets = append(sets, fmt.Sprintf("active_daily_curricula_by_subject = $%d::jsonb", next))
			args = append(args, daily)
			next++
		}
		if len(in.StarDuelCurricula) > 0 {
			duel, _ := json.Marshal(map[string][]string{"math": in.StarDuelCurricula})
			sets = append(sets, fmt.Sprintf("active_duel_curricula_by_subject = $%d::jsonb", next))
			args = append(args, duel)
			next++
		}
		if in.BrainGalaxyDailyCapMin != nil {
			sets = append(sets, fmt.Sprintf("brain_galaxy_daily_cap_sec = $%d", next))
			args = append(args, b.Settings.BrainGalaxyDailyCapMin*60)
			next++
		}
		if in.BrainGalaxyWindowStart != "" {
			sets = append(sets, fmt.Sprintf("brain_galaxy_window_start = $%d::time", next))
			args = append(args, in.BrainGalaxyWindowStart)
			next++
		}
		if in.BrainGalaxyWindowEnd != "" {
			sets = append(sets, fmt.Sprintf("brain_galaxy_window_end = $%d::time", next))
			args = append(args, in.BrainGalaxyWindowEnd)
		}
		if len(sets) > 0 {
			query := "UPDATE student_profiles SET " + strings.Join(sets, ", ") + ", updated_at = NOW() WHERE id = $1::uuid"
			if _, err := tx.Exec(ctx, query, args...); err != nil {
				return ProfileBundle{}, fmt.Errorf("persist family settings: %w", err)
			}
		}
		if in.OTPEnabled != nil && isUUID(parentID) {
			if _, err := tx.Exec(ctx, `UPDATE user_accounts SET otp_enabled = $2, updated_at = NOW() WHERE id = $1::uuid`, parentID, *in.OTPEnabled); err != nil {
				return ProfileBundle{}, fmt.Errorf("persist otp setting: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return ProfileBundle{}, err
		}
		return s.loadBundlePG(ctx, childID)
	}

	s.mu.Lock()
	b = *s.getOrCreateLocked(childID)
	if in.OTPEnabled != nil {
		b.Settings.OTPEnabled = *in.OTPEnabled
	}
	if in.Language == "en" || in.Language == "id" {
		b.Settings.Language = in.Language
	}
	if len(in.DailyOrbitCurricula) > 0 {
		b.Settings.DailyOrbitCurricula = in.DailyOrbitCurricula
	}
	if len(in.StarDuelCurricula) > 0 {
		b.Settings.StarDuelCurricula = in.StarDuelCurricula
	}
	if in.BrainGalaxyDailyCapMin != nil {
		capMin := *in.BrainGalaxyDailyCapMin
		if capMin < 0 {
			capMin = 0
		}
		if capMin > 25 {
			capMin = 25
		}
		b.Settings.BrainGalaxyDailyCapMin = capMin
	}
	if in.BrainGalaxyWindowStart != "" {
		b.Settings.BrainGalaxyWindowStart = in.BrainGalaxyWindowStart
	}
	if in.BrainGalaxyWindowEnd != "" {
		b.Settings.BrainGalaxyWindowEnd = in.BrainGalaxyWindowEnd
	}
	*s.getOrCreateLocked(childID) = b
	s.mu.Unlock()
	return b, nil
}

// ListChildren returns every child linked to the authenticated parent, or the
// student's own profile, so the assignment builder can select a child.
func (s *Service) ListChildren(ctx context.Context, actor auth.Principal) ([]ChildProfile, error) {
	if actor.Role != auth.RoleParent {
		bundle, err := s.GetBundle(ctx, actor, "")
		if err != nil {
			return nil, err
		}
		return []ChildProfile{bundle.Child}, nil
	}
	if s.pool == nil || !isUUID(actor.ID) {
		if actor.ID == "demo-parent" {
			return []ChildProfile{{StudentID: "demo-pilot1", FirstName: "Demo Pilot", Grade: 1}}, nil
		}
		return nil, ErrNoLinkedChild
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text FROM student_profiles
		WHERE parent_account_id = $1::uuid
		ORDER BY created_at, id
	`, actor.ID)
	if err != nil {
		return nil, err
	}
	childIDs := make([]string, 0)
	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			rows.Close()
			return nil, err
		}
		childIDs = append(childIDs, childID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	children := make([]ChildProfile, 0, len(childIDs))
	for _, childID := range childIDs {
		bundle, err := s.loadBundlePG(ctx, childID)
		if err != nil {
			return nil, err
		}
		children = append(children, bundle.Child)
	}
	if len(children) == 0 {
		return nil, ErrNoLinkedChild
	}
	return children, nil
}

func (s *Service) GetParentOverview(ctx context.Context, actor auth.Principal, locale, childID string) (ParentChildOverview, error) {
	childID, err := s.resolveChildID(ctx, actor, childID)
	if err != nil {
		return ParentChildOverview{}, err
	}
	bundle, err := s.GetBundle(ctx, actor, childID)
	if err != nil {
		return ParentChildOverview{}, err
	}

	ov := ParentChildOverview{
		Child:          bundle.Child,
		StreakDays:     3,
		CryoShields:    2,
		FocusSkills:    []training.SkillView{},
		StrengthSkills: []training.SkillView{},
	}
	if s.streakSvc != nil {
		tz := actor.Timezone
		if tz == "" {
			tz = "Asia/Jakarta"
		}
		st, err := s.streakSvc.GetStatus(childID, tz)
		if err != nil {
			return ParentChildOverview{}, err
		}
		ov.StreakDays = st.CurrentStreak
		ov.CryoShields = st.CryoShields
	}
	if s.reportsH != nil {
		prof, err := s.reportsH.BuildProficiency(childID, "ALL", bundle.Child.Grade, locale)
		if err != nil {
			return ParentChildOverview{}, err
		}
		ov.MasteryCount = prof.MasteryCount
		ov.CompetentCount = prof.CompetentCount
		ov.BeginningCount = prof.BeginningCount
		ov.IncompleteCount = prof.IncompleteCount
		for _, sk := range prof.Skills {
			if sk.ProficiencyTier == training.TierMastery && len(ov.StrengthSkills) < 3 {
				ov.StrengthSkills = append(ov.StrengthSkills, sk)
			} else if (sk.ProficiencyTier == training.TierBeginning || sk.ProficiencyTier == training.TierIncomplete) && len(ov.FocusSkills) < 3 {
				ov.FocusSkills = append(ov.FocusSkills, sk)
			}
		}
	}
	return ov, nil
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

type AuthIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

func (h *Handler) resolvePrincipal(ctx context.Context, in AuthIn) (auth.Principal, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	if h.authSvc == nil {
		return auth.Principal{ID: "demo-parent", Role: auth.RoleParent, DisplayName: "Budi Santoso"}, locale, nil
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	if tok != "" {
		if p, err := h.authSvc.ResolvePrincipal(ctx, tok); err == nil {
			return *p, locale, nil
		}
	}
	return auth.Principal{}, locale, auth.ErrInvalidCredentials
}

var authSecurity = []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}}

// lookupError returns 404 for known "nothing linked/found" outcomes and a generic 500 otherwise,
// so database errors are logged instead of being echoed to the client.
func lookupError(ctx context.Context, err error) error {
	if errors.Is(err, ErrNoLinkedChild) || errors.Is(err, ErrProfileNotFound) {
		return huma.Error404NotFound(err.Error())
	}
	slog.ErrorContext(ctx, "profile lookup failed", "err", err)
	return huma.Error500InternalServerError("internal error")
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-get",
		Method:      http.MethodGet,
		Path:        "/api/v1/profile/bundle",
		Summary:     "Get Parent & Child profile cards and family settings",
		Tags:        []string{"Profile"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		ChildID string `query:"childId" doc:"Linked child the bundle is for; defaults to the parent's first child"`
	}) (*struct{ Body ProfileBundle }, error) {
		actor, _, err := h.resolvePrincipal(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		bundle, err := h.svc.GetBundle(ctx, actor, in.ChildID)
		if err != nil {
			return nil, lookupError(ctx, err)
		}
		return &struct{ Body ProfileBundle }{Body: bundle}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "profile-children",
		Method:      http.MethodGet,
		Path:        "/api/v1/profile/children",
		Summary:     "List the authenticated parent's linked children",
		Tags:        []string{"Profile"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *AuthIn) (*struct {
		Body struct {
			Children []ChildProfile `json:"children"`
		}
	}, error) {
		actor, _, err := h.resolvePrincipal(ctx, *in)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		children, err := h.svc.ListChildren(ctx, actor)
		if err != nil {
			return nil, lookupError(ctx, err)
		}
		out := &struct {
			Body struct {
				Children []ChildProfile `json:"children"`
			}
		}{}
		out.Body.Children = children
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "profile-update-settings",
		Method:      http.MethodPatch,
		Path:        "/api/v1/profile/settings",
		Summary:     "Update OTP-protected family language, curricula, and Brain Galaxy screen-time limits",
		Tags:        []string{"Profile"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Body UpdateSettingsInput
	}) (*struct{ Body ProfileBundle }, error) {
		actor, _, err := h.resolvePrincipal(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		// Family settings are parent-sensitive (AGENTS.md guardrail #6).
		if actor.Role != auth.RoleParent {
			return nil, huma.Error403Forbidden("parent account required")
		}
		updated, err := h.svc.UpdateSettings(ctx, actor, in.Body)
		if err != nil {
			switch {
			case errors.Is(err, ErrOTPRequired), errors.Is(err, ErrParentOnly):
				return nil, huma.Error403Forbidden(err.Error())
			case errors.Is(err, ErrInvalidInput):
				return nil, huma.Error400BadRequest(err.Error())
			case errors.Is(err, ErrNoLinkedChild), errors.Is(err, ErrProfileNotFound):
				return nil, huma.Error404NotFound(err.Error())
			default:
				slog.ErrorContext(ctx, "update family settings failed", "err", err)
				return nil, huma.Error500InternalServerError("internal error")
			}
		}
		return &struct{ Body ProfileBundle }{Body: updated}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "parent-overview",
		Method:      http.MethodGet,
		Path:        "/api/v1/parent/overview",
		Summary:     "Parent Portal family dashboard snapshot (streaks, mastery counts, top focus areas)",
		Tags:        []string{"Parent"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		ChildID string `query:"childId" doc:"Linked child the overview is for; defaults to the parent's first child"`
	}) (*struct{ Body ParentChildOverview }, error) {
		actor, locale, err := h.resolvePrincipal(ctx, in.AuthIn)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if actor.Role != auth.RoleParent {
			return nil, huma.Error403Forbidden("parent account required")
		}
		ov, err := h.svc.GetParentOverview(ctx, actor, locale, in.ChildID)
		if err != nil {
			if errors.Is(err, ErrNoLinkedChild) || errors.Is(err, ErrProfileNotFound) {
				return nil, huma.Error404NotFound(err.Error())
			}
			slog.ErrorContext(ctx, "parent overview failed", "err", err)
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &struct{ Body ParentChildOverview }{Body: ov}, nil
	})
}
