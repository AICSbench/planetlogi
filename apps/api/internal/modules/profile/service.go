package profile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/reports"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
)

var (
	ErrOTPRequired  = errors.New("parent OTP verification is required to update protected settings")
	ErrInvalidInput = errors.New("invalid input")
)

const (
	maxCurricula      = 8
	maxCurriculumCode = 32
)

var hhmmPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

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

func (s *Service) GetBundle(studentID string) ProfileBundle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.getOrCreateLocked(studentID)
}

type UpdateSettingsInput struct {
	OTPEnabled             *bool    `json:"otpEnabled,omitempty"`
	OTPCode                string   `json:"otpCode,omitempty"`
	Language               string   `json:"language,omitempty"`
	DailyOrbitCurricula    []string `json:"dailyOrbitCurricula,omitempty"`
	StarDuelCurricula      []string `json:"starDuelCurricula,omitempty"`
	BrainGalaxyDailyCapMin *int     `json:"brainGalaxyDailyCapMin,omitempty"`
	BrainGalaxyWindowStart string   `json:"brainGalaxyWindowStart,omitempty"`
	BrainGalaxyWindowEnd   string   `json:"brainGalaxyWindowEnd,omitempty"`
}

func (s *Service) UpdateSettings(ctx context.Context, studentID string, actor auth.Principal, in UpdateSettingsInput) (ProfileBundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateSettingsInput(in); err != nil {
		return ProfileBundle{}, err
	}
	b := s.getOrCreateLocked(studentID)

	// Enforce Parent OTP gate when OTP security is enabled. Fail closed: without an OTP
	// verifier no code can be accepted.
	if b.Settings.OTPEnabled {
		if in.OTPCode == "" || s.authSvc == nil {
			return ProfileBundle{}, ErrOTPRequired
		}
		if err := s.authSvc.VerifyParentOTP(ctx, actor, in.OTPCode); err != nil {
			return ProfileBundle{}, ErrOTPRequired
		}
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
	return *b, nil
}

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

func (s *Service) GetParentOverview(studentID, locale string) (ParentChildOverview, error) {
	s.mu.Lock()
	bundle := *s.getOrCreateLocked(studentID)
	s.mu.Unlock()

	ov := ParentChildOverview{
		Child:          bundle.Child,
		StreakDays:     3,
		CryoShields:    2,
		FocusSkills:    []training.SkillView{},
		StrengthSkills: []training.SkillView{},
	}
	if s.streakSvc != nil {
		st, err := s.streakSvc.GetStatus(studentID, "Asia/Jakarta")
		if err != nil {
			return ParentChildOverview{}, err
		}
		ov.StreakDays = st.CurrentStreak
		ov.CryoShields = st.CryoShields
	}
	if s.reportsH != nil {
		prof, err := s.reportsH.BuildProficiency(studentID, "ALL", bundle.Child.Grade, locale)
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

var authSecurity = []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}}

func (h *Handler) resolve(ctx context.Context, c reqguard.Credentials, requireAuth bool) (auth.Principal, error) {
	var r reqguard.PrincipalResolver
	if h.authSvc != nil {
		r = h.authSvc
	}
	return reqguard.Resolve(ctx, r, c, requireAuth)
}

// resolveParent requires an authenticated parent session (AGENTS.md guardrail #6).
func (h *Handler) resolveParent(ctx context.Context, c reqguard.Credentials) (auth.Principal, error) {
	p, err := h.resolve(ctx, c, true)
	if err != nil {
		return auth.Principal{}, huma.Error401Unauthorized("authentication required")
	}
	if p.Role != auth.RoleParent {
		return auth.Principal{}, huma.Error403Forbidden("parent account required")
	}
	return p, nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-get",
		Method:      http.MethodGet,
		Path:        "/api/v1/profile/bundle",
		Summary:     "Get Parent & Child profile cards and family settings",
		Tags:        []string{"Profile"},
	}, func(ctx context.Context, in *reqguard.Credentials) (*struct{ Body ProfileBundle }, error) {
		p, err := h.resolve(ctx, *in, false)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		return &struct{ Body ProfileBundle }{Body: h.svc.GetBundle(p.ID)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "profile-update-settings",
		Method:      http.MethodPatch,
		Path:        "/api/v1/profile/settings",
		Summary:     "Update OTP-protected family language, curricula, and Brain Galaxy screen-time limits (parent only)",
		Tags:        []string{"Profile"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		reqguard.Credentials
		Body UpdateSettingsInput
	}) (*struct{ Body ProfileBundle }, error) {
		actor, err := h.resolveParent(ctx, in.Credentials)
		if err != nil {
			return nil, err
		}
		updated, err := h.svc.UpdateSettings(ctx, actor.ID, actor, in.Body)
		if err != nil {
			switch {
			case errors.Is(err, ErrOTPRequired):
				return nil, huma.Error403Forbidden(err.Error())
			case errors.Is(err, ErrInvalidInput):
				return nil, huma.Error400BadRequest(err.Error())
			default:
				return nil, huma.Error500InternalServerError("internal error")
			}
		}
		return &struct{ Body ProfileBundle }{Body: updated}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "parent-overview",
		Method:      http.MethodGet,
		Path:        "/api/v1/parent/overview",
		Summary:     "Parent Portal family dashboard snapshot (streaks, mastery counts, top focus areas; parent only)",
		Tags:        []string{"Parent"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *reqguard.Credentials) (*struct{ Body ParentChildOverview }, error) {
		actor, err := h.resolveParent(ctx, *in)
		if err != nil {
			return nil, err
		}
		ov, err := h.svc.GetParentOverview(actor.ID, in.NormalizedLocale())
		if err != nil {
			slog.ErrorContext(ctx, "parent overview failed", "err", err)
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &struct{ Body ParentChildOverview }{Body: ov}, nil
	})
}
