package customization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
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

// customizationProgressDoc is the durable per-student document (Astro Buddy
// config + earned Daily Star Badge tiers). Stored in the shared phase2_state
// store so it survives restarts and is visible to every API instance.
type customizationProgressDoc struct {
	Buddy  *AstroBuddyConfig `json:"buddy,omitempty"`
	Badges map[string]int    `json:"badges,omitempty"` // YYYY-MM-DD -> tier (1..4)
}

const customizationProgressModule = "customization_progress_v1"

type Service struct {
	mu       sync.Mutex
	ledger   economy.Ledger
	pool     *pgxpool.Pool
	store    *state.PGStore // nil in in-memory (test) mode
	buddies  map[string]AstroBuddyConfig
	badges   map[string]map[string]int // studentID -> YYYY-MM-DD -> tier (1..4) (memory mode only)
	showcase []ShowcaseEntry
	nowClock func() time.Time
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

// NewService creates an in-memory service for unit tests.
func NewService(ledger economy.Ledger) *Service {
	s := &Service{
		ledger:   ledger,
		buddies:  make(map[string]AstroBuddyConfig),
		badges:   make(map[string]map[string]int),
		nowClock: time.Now,
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

// NewServiceWithPG creates a production service whose Astro Buddy config and
// earned Daily Star Badge tiers are stored durably in PostgreSQL.
func NewServiceWithPG(pool *pgxpool.Pool, ledger economy.Ledger) *Service {
	s := NewService(ledger)
	s.pool = pool
	s.store = state.NewPGStore(pool)
	return s
}

func (s *Service) loadProgressPG(ctx context.Context, studentID string) customizationProgressDoc {
	var doc customizationProgressDoc
	if s.store == nil {
		return doc
	}
	if found, err := s.store.Load(ctx, studentID, customizationProgressModule, &doc); err != nil || !found {
		return customizationProgressDoc{}
	}
	return doc
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

func (s *Service) GetBuddy(ctx context.Context, studentID, firstName string) AstroBuddyConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store != nil {
		if b := s.loadProgressPG(ctx, studentID).Buddy; b != nil {
			return *b
		}
	} else if b, ok := s.buddies[studentID]; ok {
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

// SaveBuddy persists the Astro Buddy config durably (PG mode); a persistence
// failure is propagated so the client knows the save did not stick instead of
// receiving a false success.
func (s *Service) SaveBuddy(ctx context.Context, studentID, firstName string, cfg AstroBuddyConfig) (AstroBuddyConfig, error) {
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
	if s.store != nil {
		if err := s.store.SaveMerged(ctx, studentID, customizationProgressModule, func(current []byte) (any, error) {
			var doc customizationProgressDoc
			_ = json.Unmarshal(current, &doc)
			doc.Buddy = &cfg
			return doc, nil
		}); err != nil {
			return AstroBuddyConfig{}, err
		}
		return cfg, nil
	}
	s.buddies[studentID] = cfg
	return cfg, nil
}

// showcaseScope stores the Galaxy Showcase entry list in the shared
// shared_state store, so the entry set is consistent across restarts and API
// instances and future writes (e.g. moderation-approved entries) persist.
// Wiring real students' buddies and SP into the showcase (CR-13) is a
// deferred product decision — this only makes the entry store durable.
const (
	showcaseScope = "showcase"
	showcaseKey   = "entries"
)

// GetShowcase returns buddies ranked by SP for the selected grade (requires >=1 SP, masked names only).
// In durable (PG) mode the entries are loaded from the shared store; the demo
// seed list is written once on first use, and read errors fall back to the
// in-memory seed so the endpoint keeps working.
func (s *Service) GetShowcase(ctx context.Context, grade int) []ShowcaseEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := s.showcase
	if s.store != nil {
		var durable []ShowcaseEntry
		found, err := s.store.LoadShared(ctx, showcaseScope, showcaseKey, &durable)
		if err == nil && found && len(durable) > 0 {
			entries = durable
		} else if err == nil {
			_ = s.store.SaveShared(ctx, showcaseScope, showcaseKey, s.showcase)
		}
	}
	filtered := make([]ShowcaseEntry, 0, len(entries))
	for _, e := range entries {
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

func (s *Service) GetMonthlyBadges(ctx context.Context, studentID, monthPrefix, locale string) []DailyBadgeCard {
	s.mu.Lock()
	defer s.mu.Unlock()
	if monthPrefix == "" {
		monthPrefix = "2026-11"
	}
	badgeTiers := s.badges[studentID]
	if s.store != nil {
		badgeTiers = s.loadProgressPG(ctx, studentID).Badges
	} else if badgeTiers == nil {
		// In-memory (test) mode only: simulate the first four days so the
		// calendar shows a mix of tiers.
		s.badges[studentID] = map[string]int{
			monthPrefix + "-01": 1,
			monthPrefix + "-02": 2,
			monthPrefix + "-03": 3,
			monthPrefix + "-04": 4,
		}
		badgeTiers = s.badges[studentID]
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
		tier := badgeTiers[dt]
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

// RecoverMissedBadge deducts 20 AC and persists the recovered tier in the same
// database transaction (idempotent per student+date). Dates must be valid
// calendar dates not in the future — recovering a day that has not happened
// yet, or a malformed date string, is rejected without touching the wallet.
func (s *Service) RecoverMissedBadge(ctx context.Context, studentID, tz, dateStr, locale string) (DailyBadgeCard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parsed, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
	if err != nil {
		return DailyBadgeCard{}, fmt.Errorf("invalid badge date %q", dateStr)
	}
	localNow := timewindow.LocalDateString(s.nowClock(), tz)
	today, err := time.ParseInLocation("2006-01-02", localNow, time.UTC)
	if err != nil {
		return DailyBadgeCard{}, fmt.Errorf("invalid server timezone %q", tz)
	}
	if parsed.After(today) {
		return DailyBadgeCard{}, fmt.Errorf("cannot recover a future badge date")
	}

	if s.store != nil {
		doc := s.loadProgressPG(ctx, studentID)
		if doc.Badges[dateStr] == 0 {
			if s.ledger != nil {
				tx, err := s.pool.Begin(ctx)
				if err != nil {
					return DailyBadgeCard{}, err
				}
				defer func() { _ = tx.Rollback(ctx) }()
				if _, err := economy.ApplyBatchInTx(ctx, tx, s.ledger, []economy.MutationRequest{
					{
						StudentID:      studentID,
						Currency:       economy.CurrencyAC,
						Delta:          -MissedBadgeRecoverCostAC,
						SourceType:     "BADGE_RECOVERY",
						SourceID:       dateStr,
						IdempotencyKey: fmt.Sprintf("badge_recover:%s:%s", studentID, dateStr),
					},
				}); err != nil {
					return DailyBadgeCard{}, err
				}
				if err := s.store.SaveMergedTx(ctx, tx, studentID, customizationProgressModule, func(current []byte) (any, error) {
					var d customizationProgressDoc
					_ = json.Unmarshal(current, &d)
					if d.Badges == nil {
						d.Badges = make(map[string]int)
					}
					d.Badges[dateStr] = 1
					return d, nil
				}); err != nil {
					return DailyBadgeCard{}, err
				}
				if err := tx.Commit(ctx); err != nil {
					return DailyBadgeCard{}, err
				}
			} else {
				if err := s.store.SaveMerged(ctx, studentID, customizationProgressModule, func(current []byte) (any, error) {
					var d customizationProgressDoc
					_ = json.Unmarshal(current, &d)
					if d.Badges == nil {
						d.Badges = make(map[string]int)
					}
					d.Badges[dateStr] = 1
					return d, nil
				}); err != nil {
					return DailyBadgeCard{}, err
				}
			}
		}
		tier := s.loadProgressPG(ctx, studentID).Badges[dateStr]
		return DailyBadgeCard{
			Date:        dateStr,
			Tier:        tier,
			TierName:    tierName(tier),
			TriviaTitle: mathengine.LocaleText{EN: "Recovered Star Card", ID: "Kartu Bintang Terbuka"}.For(locale),
			RecoverCost: MissedBadgeRecoverCostAC,
		}, nil
	}

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

// resolve returns the authenticated student identity, first name, and locale.
// Missing or invalid credentials map to HTTP 401 and non-students to HTTP 403.
func (h *Handler) resolve(ctx context.Context, in authIn) (auth.StudentIdentity, string, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	ident, err := h.authSvc.ResolveStudent(ctx, tok)
	if err != nil {
		return auth.StudentIdentity{}, "", locale, auth.StudentAuthHTTPError(err)
	}
	firstName := ident.DisplayName
	if fields := strings.Fields(firstName); len(fields) > 0 {
		firstName = fields[0]
	}
	return ident, firstName, locale, nil
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
		ident, name, locale, err := h.resolve(ctx, *in)
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
		out.Body.Buddy = h.svc.GetBuddy(ctx, ident.ID, name)
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
		ident, name, _, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		cfg, err := h.svc.SaveBuddy(ctx, ident.ID, name, in.Body)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("could not save Astro Buddy; try again")
		}
		return &struct{ Body AstroBuddyConfig }{Body: cfg}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "showcase-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/showcase",
		Summary:     "Galaxy Showcase ranked by Star Points across Grades 1-6 (requires >=1 SP)",
		Tags:        []string{"Customization"},
	}, func(ctx context.Context, in *struct {
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
		out.Body.Entries = h.svc.GetShowcase(ctx, in.Grade)
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
		ident, _, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		out := &struct {
			Body struct {
				Cards []DailyBadgeCard `json:"cards"`
			}
		}{}
		out.Body.Cards = h.svc.GetMonthlyBadges(ctx, ident.ID, in.Month, locale)
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
		ident, _, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		card, err := h.svc.RecoverMissedBadge(ctx, ident.ID, ident.Timezone, in.Body.Date, locale)
		if err != nil {
			if errors.Is(err, economy.ErrInsufficientFunds) {
				return nil, huma.Error400BadRequest("insufficient Astro Coins (20 AC required)")
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body DailyBadgeCard }{Body: card}, nil
	})
}
