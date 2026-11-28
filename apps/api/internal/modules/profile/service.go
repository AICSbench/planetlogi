package profile

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/reports"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/streak"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
)

var ErrOTPRequired = errors.New("parent OTP verification is required to update protected settings")

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
	OTPEnabled               bool     `json:"otpEnabled"`
	Language                 string   `json:"language"`
	DailyOrbitCurricula      []string `json:"dailyOrbitCurricula"`
	StarDuelCurricula        []string `json:"starDuelCurricula"`
	BrainGalaxyDailyCapMin   int      `json:"brainGalaxyDailyCapMin"`
	BrainGalaxyWindowStart   string   `json:"brainGalaxyWindowStart"`
	BrainGalaxyWindowEnd     string   `json:"brainGalaxyWindowEnd"`
	SubscriptionPlanName     string   `json:"subscriptionPlanName"`
	SubscriptionValidUntil   string   `json:"subscriptionValidUntil"`
	BillingEnabled           bool     `json:"billingEnabled"`
}

type ProfileBundle struct {
	Parent   ParentProfile  `json:"parent"`
	Child    ChildProfile   `json:"child"`
	Settings FamilySettings `json:"settings"`
}

type ParentChildOverview struct {
	Child             ChildProfile         `json:"child"`
	StreakDays        int                  `json:"streakDays"`
	CryoShields       int                  `json:"cryoShields"`
	MasteryCount      int                  `json:"masteryCount"`
	CompetentCount    int                  `json:"competentCount"`
	BeginningCount    int                  `json:"beginningCount"`
	IncompleteCount   int                  `json:"incompleteCount"`
	FocusSkills       []training.SkillView `json:"focusSkills"`
	StrengthSkills    []training.SkillView `json:"strengthSkills"`
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
	b := s.getOrCreateLocked(studentID)

	// Enforce Parent OTP gate when OTP security is enabled
	if b.Settings.OTPEnabled {
		if in.OTPCode == "" {
			return ProfileBundle{}, ErrOTPRequired
		}
		if s.authSvc != nil {
			if err := s.authSvc.VerifyParentOTP(ctx, actor, in.OTPCode); err != nil {
				return ProfileBundle{}, ErrOTPRequired
			}
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

type authIn struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Locale        string `query:"locale" default:"en"`
}

func (h *Handler) resolvePrincipal(ctx context.Context, in authIn) (string, auth.Principal, string) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	if h.authSvc != nil {
		tok := in.CookieAccess
		if tok == "" && len(in.Authorization) > 7 {
			tok = in.Authorization[7:]
		}
		if tok != "" {
			if p, err := h.authSvc.ResolvePrincipal(ctx, tok); err == nil {
				return p.ID, *p, locale
			}
		}
	}
	return "demo-pilot1", auth.Principal{ID: "demo-parent", Role: auth.RoleParent, DisplayName: "Budi Santoso"}, locale
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profile-get",
		Method:      http.MethodGet,
		Path:        "/api/v1/profile/bundle",
		Summary:     "Get Parent & Child profile cards and family settings",
		Tags:        []string{"Profile"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body ProfileBundle }, error) {
		sid, _, _ := h.resolvePrincipal(ctx, *in)
		return &struct{ Body ProfileBundle }{Body: h.svc.GetBundle(sid)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "profile-update-settings",
		Method:      http.MethodPatch,
		Path:        "/api/v1/profile/settings",
		Summary:     "Update OTP-protected family language, curricula, and Brain Galaxy screen-time limits",
		Tags:        []string{"Profile"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body UpdateSettingsInput
	}) (*struct{ Body ProfileBundle }, error) {
		sid, actor, _ := h.resolvePrincipal(ctx, in.authIn)
		updated, err := h.svc.UpdateSettings(ctx, sid, actor, in.Body)
		if err != nil {
			if errors.Is(err, ErrOTPRequired) {
				return nil, huma.Error403Forbidden(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body ProfileBundle }{Body: updated}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "parent-overview",
		Method:      http.MethodGet,
		Path:        "/api/v1/parent/overview",
		Summary:     "Parent Portal family dashboard snapshot (streaks, mastery counts, top focus areas)",
		Tags:        []string{"Parent"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body ParentChildOverview }, error) {
		sid, _, locale := h.resolvePrincipal(ctx, *in)
		ov, err := h.svc.GetParentOverview(sid, locale)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return &struct{ Body ParentChildOverview }{Body: ov}, nil
	})
}
