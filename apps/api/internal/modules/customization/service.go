package customization

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
)

const MissedBadgeRecoverCostAC = 20

type AvatarPack struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

type AstroBuddyConfig struct {
	BodyShape    string `json:"bodyShape"` // round | star | pear | cube
	ColorHex     string `json:"colorHex"`
	HornsStyle   string `json:"hornsStyle"` // antennae | cat_ears | comet_horns | halo
	EyesStyle    string `json:"eyesStyle"`  // bright | starry | cyclops | visor
	MouthStyle   string `json:"mouthStyle"` // smile | grin | cat | whistle
	Adjective    string `json:"adjective"`
	FullName     string `json:"fullName"`
	AvatarPackID string `json:"avatarPackId"`
	AvatarItemID string `json:"avatarItemId"`
}

type ShowcaseEntry struct {
	Rank       int              `json:"rank"`
	MaskedName string           `json:"maskedName"`
	Grade      int              `json:"grade"`
	StarPoints int64            `json:"starPoints"`
	Buddy      AstroBuddyConfig `json:"buddy"`
}

type DailyBadgeCard struct {
	Date        string `json:"date"`
	DayOfMonth  int    `json:"dayOfMonth"`
	Tier        int    `json:"tier"` // 0=locked, 1=Bronze, 2=Silver, 3=Gold, 4=Holographic
	TierName    string `json:"tierName"`
	Category    string `json:"category"`
	TriviaTitle string `json:"triviaTitle"`
	TriviaFact  string `json:"triviaFact"`
	RecoverCost int    `json:"recoverCostAc"`
}

type Service struct {
	mu       sync.Mutex
	ledger   economy.Ledger
	buddies  map[string]AstroBuddyConfig
	badges   map[string]map[string]int // studentID -> YYYY-MM-DD -> tier (1..4)
	showcase []ShowcaseEntry
}

var CosmicColors = []string{
	"#6366F1", "#22D3EE", "#10B981", "#FBBF24", "#F97316",
	"#EC4899", "#A855F7", "#3B82F6", "#14B8A6", "#84CC16",
	"#EF4444", "#F43F5E", "#8B5CF6", "#06B6D4", "#F59E0B",
}

var CosmicAdjectives = []string{
	"Starry", "Cosmic", "Turbo", "Nebula", "Bouncy", "Solar", "Lunar", "Sparkling",
	"Comet", "Quasar", "Meteor", "Aurora", "Galactic", "Rocket", "Orbit", "Nova",
	"Crystal", "Velvet", "Sunny", "Swift", "Clever", "Mighty", "Zippy", "Dazzling",
	"Astro", "Plucky", "Radiant", "Bright", "Jolly", "Hyper", "Prism", "Stellar",
	"Glowing", "Breezy", "Cheery", "Zesty", "Nimble", "Brave", "Happy", "Lucky",
	"Super", "Mega", "Ultra", "Pixel", "Flash", "Cloudy", "Misty", "Dreamy",
	"Snappy", "Spunky",
}

func NewService(ledger economy.Ledger) *Service {
	s := &Service{
		ledger:  ledger,
		buddies: make(map[string]AstroBuddyConfig),
		badges:  make(map[string]map[string]int),
	}
	s.showcase = []ShowcaseEntry{
		{MaskedName: "Maya K.", Grade: 1, StarPoints: 68, Buddy: AstroBuddyConfig{BodyShape: "star", ColorHex: "#22D3EE", HornsStyle: "halo", EyesStyle: "starry", MouthStyle: "smile", Adjective: "Cosmic", FullName: "Cosmic Maya"}},
		{MaskedName: "Ethan L.", Grade: 1, StarPoints: 52, Buddy: AstroBuddyConfig{BodyShape: "round", ColorHex: "#FBBF24", HornsStyle: "antennae", EyesStyle: "visor", MouthStyle: "grin", Adjective: "Turbo", FullName: "Turbo Ethan"}},
		{MaskedName: "Raka S.", Grade: 1, StarPoints: 29, Buddy: AstroBuddyConfig{BodyShape: "round", ColorHex: "#6366F1", HornsStyle: "cat_ears", EyesStyle: "bright", MouthStyle: "smile", Adjective: "Starry", FullName: "Starry Raka"}},
		{MaskedName: "Siti R.", Grade: 2, StarPoints: 44, Buddy: AstroBuddyConfig{BodyShape: "pear", ColorHex: "#EC4899", HornsStyle: "comet_horns", EyesStyle: "bright", MouthStyle: "cat", Adjective: "Nova", FullName: "Nova Siti"}},
		{MaskedName: "Zero S.", Grade: 1, StarPoints: 0, Buddy: AstroBuddyConfig{BodyShape: "cube", ColorHex: "#10B981", FullName: "Hidden Zero"}}, // MUST be filtered out (<1 SP)
	}
	return s
}

func AvatarPacks(locale string) []AvatarPack {
	return []AvatarPack{
		{ID: "space_pets", Name: mathengine.LocaleText{EN: "Space Pets", ID: "Hewan Antariksa"}.For(locale), Items: []string{"astro_fox", "orbit_pup", "lunar_cat"}},
		{ID: "sea_creatures", Name: mathengine.LocaleText{EN: "Cosmic Sea Creatures", ID: "Makhluk Laut Kosmik"}.For(locale), Items: []string{"star_whale", "nebula_octo", "coral_turtle"}},
		{ID: "frost_explorers", Name: mathengine.LocaleText{EN: "Frost Explorers", ID: "Penjelajah Es"}.For(locale), Items: []string{"cryo_penguin", "polar_bear", "snow_owl"}},
		{ID: "star_royals", Name: mathengine.LocaleText{EN: "Star Royals", ID: "Bangsawan Bintang"}.For(locale), Items: []string{"comet_princess", "solar_prince", "aurora_knight"}},
		{ID: "astro_dinos", Name: mathengine.LocaleText{EN: "Astro Dinos", ID: "Dino Astro"}.For(locale), Items: []string{"cosmic_rex", "tri_star", "stego_nova"}},
		{ID: "logi_crew", Name: mathengine.LocaleText{EN: "Logi Crew", ID: "Kru Logi"}.For(locale), Items: []string{"captain_logi", "engineer_miko", "scout_zara"}},
		{ID: "mini_bots", Name: mathengine.LocaleText{EN: "Mini Bots", ID: "Robot Mini"}.For(locale), Items: []string{"beep_bot", "spark_drone", "cube_ai"}},
		{ID: "mecha_pilots", Name: mathengine.LocaleText{EN: "Mecha Pilots", ID: "Pilot Mecha"}.For(locale), Items: []string{"mecha_alpha", "mecha_nova", "mecha_solaris"}},
	}
}

func (s *Service) GetBuddy(studentID, firstName string) AstroBuddyConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.buddies[studentID]; ok {
		return b
	}
	if firstName == "" {
		firstName = "Raka"
	}
	return AstroBuddyConfig{
		BodyShape:    "round",
		ColorHex:     "#6366F1",
		HornsStyle:   "antennae",
		EyesStyle:    "bright",
		MouthStyle:   "smile",
		Adjective:    "Starry",
		FullName:     "Starry " + firstName,
		AvatarPackID: "space_pets",
		AvatarItemID: "astro_fox",
	}
}

func (s *Service) SaveBuddy(studentID, firstName string, cfg AstroBuddyConfig) AstroBuddyConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if firstName == "" {
		firstName = "Raka"
	}
	// Strictly construct 2-part name from safe cosmic adjective list + first name (zero free-text risk!)
	adj := "Starry"
	for _, a := range CosmicAdjectives {
		if a == cfg.Adjective {
			adj = a
			break
		}
	}
	cfg.Adjective = adj
	cfg.FullName = adj + " " + firstName
	s.buddies[studentID] = cfg
	return cfg
}

// GetShowcase returns buddies ranked by SP for the selected grade (requires >=1 SP, masked names only).
func (s *Service) GetShowcase(grade int) []ShowcaseEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := make([]ShowcaseEntry, 0, len(s.showcase))
	for _, e := range s.showcase {
		if e.StarPoints < 1 {
			continue
		}
		if grade > 0 && e.Grade != grade {
			continue
		}
		filtered = append(filtered, e)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].StarPoints > filtered[j].StarPoints })
	for i := range filtered {
		filtered[i].Rank = i + 1
	}
	return filtered
}

func tierName(tier int) string {
	switch tier {
	case 4:
		return "Holographic"
	case 3:
		return "Gold"
	case 2:
		return "Silver"
	case 1:
		return "Bronze"
	default:
		return "Locked"
	}
}

func (s *Service) GetMonthlyBadges(studentID, monthPrefix, locale string) []DailyBadgeCard {
	s.mu.Lock()
	defer s.mu.Unlock()
	if monthPrefix == "" {
		monthPrefix = "2026-11"
	}
	if s.badges[studentID] == nil {
		s.badges[studentID] = map[string]int{
			monthPrefix + "-01": 1,
			monthPrefix + "-02": 2,
			monthPrefix + "-03": 3,
			monthPrefix + "-04": 4,
		}
	}
	facts := []struct {
		cat string
		en  string
		id  string
	}{
		{"Space", "Jupiter could fit over 1,300 Earths inside it!", "Jupiter bisa memuat lebih dari 1.300 planet Bumi!"},
		{"Geometry", "Hexagons tile space with the least border length!", "Segi enam menyusun ruang dengan keliling paling hemat!"},
		{"Numbers", "Zero is the only number that cannot be written in Roman numerals!", "Nol adalah satu-satunya angka tanpa simbol Romawi!"},
		{"Patterns", "Sunflower seeds spiral in Fibonacci numbers!", "Biji bunga matahari berputar mengikuti pola Fibonacci!"},
	}

	cards := make([]DailyBadgeCard, 30)
	for d := 1; d <= 30; d++ {
		dt := fmt.Sprintf("%s-%02d", monthPrefix, d)
		tier := s.badges[studentID][dt]
		f := facts[(d-1)%len(facts)]
		cards[d-1] = DailyBadgeCard{
			Date:        dt,
			DayOfMonth:  d,
			Tier:        tier,
			TierName:    tierName(tier),
			Category:    f.cat,
			TriviaTitle: mathengine.LocaleText{EN: fmt.Sprintf("Star Trivia #%d", d), ID: fmt.Sprintf("Trivia Bintang #%d", d)}.For(locale),
			TriviaFact:  mathengine.LocaleText{EN: f.en, ID: f.id}.For(locale),
			RecoverCost: MissedBadgeRecoverCostAC,
		}
	}
	return cards
}

// RecoverMissedBadge atomically deducts 20 AC to unlock a missed day at 1★ Bronze (idempotent).
func (s *Service) RecoverMissedBadge(ctx context.Context, studentID, dateStr, locale string) (DailyBadgeCard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.badges[studentID] == nil {
		s.badges[studentID] = make(map[string]int)
	}
	if s.badges[studentID][dateStr] == 0 {
		if s.ledger != nil {
			if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          -MissedBadgeRecoverCostAC,
				SourceType:     "BADGE_RECOVERY",
				SourceID:       dateStr,
				IdempotencyKey: fmt.Sprintf("badge_recover:%s:%s", studentID, dateStr),
			}); err != nil {
				return DailyBadgeCard{}, err
			}
		}
		s.badges[studentID][dateStr] = 1
	}
	return DailyBadgeCard{
		Date:        dateStr,
		Tier:        s.badges[studentID][dateStr],
		TierName:    tierName(s.badges[studentID][dateStr]),
		TriviaTitle: mathengine.LocaleText{EN: "Recovered Star Card", ID: "Kartu Bintang Terbuka"}.For(locale),
		RecoverCost: MissedBadgeRecoverCostAC,
	}, nil
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

// resolve returns the authenticated student ID, first name, and locale. Missing
// or invalid credentials map to HTTP 401 and non-students to HTTP 403.
func (h *Handler) resolve(ctx context.Context, in authIn) (string, string, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	ident, err := h.authSvc.ResolveStudent(ctx, tok)
	if err != nil {
		return "", "", locale, auth.StudentAuthHTTPError(err)
	}
	firstName := ident.DisplayName
	if fields := strings.Fields(firstName); len(fields) > 0 {
		firstName = fields[0]
	}
	return ident.ID, firstName, locale, nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "customization-options",
		Method:      http.MethodGet,
		Path:        "/api/v1/customization/options",
		Summary:     "Get 8 Avatar theme packs, 15 cosmic colors, 50 adjectives, and current Astro Buddy",
		Tags:        []string{"Customization"},
	}, func(ctx context.Context, in *authIn) (*struct {
		Body struct {
			Buddy       AstroBuddyConfig `json:"buddy"`
			AvatarPacks []AvatarPack     `json:"avatarPacks"`
			Colors      []string         `json:"colors"`
			Adjectives  []string         `json:"adjectives"`
		}
	}, error) {
		id, name, locale, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		out := &struct {
			Body struct {
				Buddy       AstroBuddyConfig `json:"buddy"`
				AvatarPacks []AvatarPack     `json:"avatarPacks"`
				Colors      []string         `json:"colors"`
				Adjectives  []string         `json:"adjectives"`
			}
		}{}
		out.Body.Buddy = h.svc.GetBuddy(id, name)
		out.Body.AvatarPacks = AvatarPacks(locale)
		out.Body.Colors = CosmicColors
		out.Body.Adjectives = CosmicAdjectives
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "customization-save-buddy",
		Method:      http.MethodPut,
		Path:        "/api/v1/customization/buddy",
		Summary:     "Save My Astro Buddy customization and Avatar choice",
		Tags:        []string{"Customization"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body AstroBuddyConfig
	}) (*struct{ Body AstroBuddyConfig }, error) {
		id, name, _, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		return &struct{ Body AstroBuddyConfig }{Body: h.svc.SaveBuddy(id, name, in.Body)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "showcase-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/showcase",
		Summary:     "Galaxy Showcase ranked by Star Points across Grades 1-6 (requires >=1 SP)",
		Tags:        []string{"Customization"},
	}, func(_ context.Context, in *struct {
		Grade int `query:"grade" default:"1"`
	}) (*struct {
		Body struct {
			Entries []ShowcaseEntry `json:"entries"`
		}
	}, error) {
		out := &struct {
			Body struct {
				Entries []ShowcaseEntry `json:"entries"`
			}
		}{}
		out.Body.Entries = h.svc.GetShowcase(in.Grade)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "badges-calendar",
		Method:      http.MethodGet,
		Path:        "/api/v1/badges/calendar",
		Summary:     "Monthly Daily Star Badges trivia card calendar (Bronze/Silver/Gold/Holographic)",
		Tags:        []string{"Badges"},
	}, func(ctx context.Context, in *struct {
		authIn
		Month string `query:"month" default:"2026-11"`
	}) (*struct {
		Body struct {
			Cards []DailyBadgeCard `json:"cards"`
		}
	}, error) {
		id, _, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		out := &struct {
			Body struct {
				Cards []DailyBadgeCard `json:"cards"`
			}
		}{}
		out.Body.Cards = h.svc.GetMonthlyBadges(id, in.Month, locale)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "badges-recover",
		Method:      http.MethodPost,
		Path:        "/api/v1/badges/recover",
		Summary:     "Unlock a missed Daily Star Badge for 20 Astro Coins",
		Tags:        []string{"Badges"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			Date string `json:"date"`
		}
	}) (*struct{ Body DailyBadgeCard }, error) {
		id, _, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		card, err := h.svc.RecoverMissedBadge(ctx, id, in.Body.Date, locale)
		if err != nil {
			if errors.Is(err, economy.ErrInsufficientFunds) {
				return nil, huma.Error400BadRequest("insufficient Astro Coins (20 AC required)")
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body DailyBadgeCard }{Body: card}, nil
	})
}
