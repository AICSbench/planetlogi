package stories

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
)

const ChapterUnlockCostAC = 20

type StoryPage struct {
	Page       int    `json:"page"`
	SceneTheme string `json:"sceneTheme"`
	Text       string `json:"text"`
}

type ChapterSummary struct {
	Code         string `json:"code"`
	Index        int    `json:"index"`
	Title        string `json:"title"`
	UnlockCostAC int    `json:"unlockCostAc"`
	Unlocked     bool   `json:"unlocked"`
	LastPageRead int    `json:"lastPageRead"`
	PageCount    int    `json:"pageCount"`
}

type PlanetWorld struct {
	Grade       int              `json:"grade"`
	Code        string           `json:"code"`
	Name        string           `json:"name"`
	AccentColor string           `json:"accentColor"`
	Chapters    []ChapterSummary `json:"chapters"`
}

type ChapterViewer struct {
	PlanetCode   string      `json:"planetCode"`
	PlanetName   string      `json:"planetName"`
	ChapterCode  string      `json:"chapterCode"`
	Title        string      `json:"title"`
	AccentColor  string      `json:"accentColor"`
	LastPageRead int         `json:"lastPageRead"`
	Pages        []StoryPage `json:"pages"`
}

type chapterDef struct {
	code         string
	planetCode   string
	index        int
	unlockCostAC int
	title        mathengine.LocaleText
	heroName     mathengine.LocaleText
	planetName   mathengine.LocaleText
	grade        int
	accentColor  string
	pageCount    int
}

type planetDef struct {
	grade       int
	code        string
	name        mathengine.LocaleText
	accentColor string
}

type Service struct {
	mu        sync.Mutex
	ledger    economy.Ledger
	planets   []planetDef
	chapters  []chapterDef
	unlocked  map[string]map[string]bool // studentID -> chapterCode -> unlocked
	bookmarks map[string]map[string]int  // studentID -> chapterCode -> page
}

func NewService(ledger economy.Ledger) *Service {
	p, c := buildPlanets()
	return &Service{
		ledger:    ledger,
		planets:   p,
		chapters:  c,
		unlocked:  make(map[string]map[string]bool),
		bookmarks: make(map[string]map[string]int),
	}
}

func (s *Service) isUnlockedLocked(studentID string, ch chapterDef) bool {
	if ch.unlockCostAC == 0 {
		return true
	}
	return s.unlocked[studentID] != nil && s.unlocked[studentID][ch.code]
}

func (s *Service) ListPlanets(studentID, locale string) []PlanetWorld {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PlanetWorld, 0, len(s.planets))
	for _, p := range s.planets {
		chs := make([]ChapterSummary, 0)
		for _, ch := range s.chapters {
			if ch.planetCode != p.code {
				continue
			}
			lastPage := 1
			if s.bookmarks[studentID] != nil && s.bookmarks[studentID][ch.code] > 0 {
				lastPage = s.bookmarks[studentID][ch.code]
			}
			chs = append(chs, ChapterSummary{
				Code:         ch.code,
				Index:        ch.index,
				Title:        ch.title.For(locale),
				UnlockCostAC: ch.unlockCostAC,
				Unlocked:     s.isUnlockedLocked(studentID, ch),
				LastPageRead: lastPage,
				PageCount:    ch.pageCount,
			})
		}
		out = append(out, PlanetWorld{
			Grade:       p.grade,
			Code:        p.code,
			Name:        p.name.For(locale),
			AccentColor: p.accentColor,
			Chapters:    chs,
		})
	}
	return out
}

func (s *Service) findChapter(code string) (chapterDef, bool) {
	for _, ch := range s.chapters {
		if ch.code == code {
			return ch, true
		}
	}
	return chapterDef{}, false
}

// UnlockChapter atomically deducts 20 Astro Coins (idempotent if already unlocked).
func (s *Service) UnlockChapter(ctx context.Context, studentID, chapterCode, locale string) (ChapterViewer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.findChapter(chapterCode)
	if !ok {
		return ChapterViewer{}, fmt.Errorf("chapter %q not found", chapterCode)
	}
	if !s.isUnlockedLocked(studentID, ch) {
		if s.ledger != nil {
			if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyAC,
				Delta:          -int64(ch.unlockCostAC),
				SourceType:     "STORY_UNLOCK",
				SourceID:       ch.code,
				IdempotencyKey: fmt.Sprintf("story_unlock:%s:%s", studentID, ch.code),
			}); err != nil {
				return ChapterViewer{}, err
			}
		}
		if s.unlocked[studentID] == nil {
			s.unlocked[studentID] = make(map[string]bool)
		}
		s.unlocked[studentID][ch.code] = true
	}
	return s.buildViewerLocked(studentID, ch, locale), nil
}

func (s *Service) OpenChapter(studentID, chapterCode, locale string) (ChapterViewer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.findChapter(chapterCode)
	if !ok {
		return ChapterViewer{}, fmt.Errorf("chapter %q not found", chapterCode)
	}
	if !s.isUnlockedLocked(studentID, ch) {
		return ChapterViewer{}, economy.ErrInsufficientFunds
	}
	return s.buildViewerLocked(studentID, ch, locale), nil
}

func (s *Service) BookmarkPage(studentID, chapterCode string, page int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.findChapter(chapterCode)
	if !ok {
		return fmt.Errorf("chapter not found")
	}
	if page < 1 {
		page = 1
	}
	if page > ch.pageCount {
		page = ch.pageCount
	}
	if s.bookmarks[studentID] == nil {
		s.bookmarks[studentID] = make(map[string]int)
	}
	s.bookmarks[studentID][chapterCode] = page
	return nil
}

func (s *Service) buildViewerLocked(studentID string, ch chapterDef, locale string) ChapterViewer {
	pages := make([]StoryPage, ch.pageCount)
	for i := 0; i < ch.pageCount; i++ {
		pNum := i + 1
		pages[i] = StoryPage{
			Page:       pNum,
			SceneTheme: fmt.Sprintf("%s_scene_%d", ch.planetCode, (i%4)+1),
			Text: mathengine.LocaleText{
				EN: fmt.Sprintf("Page %d/%d — %s journeys across %s and solves Star Clue #%d using Grade %d math!", pNum, ch.pageCount, ch.heroName.EN, ch.planetName.EN, pNum, ch.grade),
				ID: fmt.Sprintf("Halaman %d/%d — %s menjelajahi %s dan memecahkan Petunjuk Bintang #%d dengan matematika Kelas %d!", pNum, ch.pageCount, ch.heroName.ID, ch.planetName.ID, pNum, ch.grade),
			}.For(locale),
		}
	}
	lastPage := 1
	if s.bookmarks[studentID] != nil && s.bookmarks[studentID][ch.code] > 0 {
		lastPage = s.bookmarks[studentID][ch.code]
	}
	return ChapterViewer{
		PlanetCode:   ch.planetCode,
		PlanetName:   ch.planetName.For(locale),
		ChapterCode:  ch.code,
		Title:        ch.title.For(locale),
		AccentColor:  ch.accentColor,
		LastPageRead: lastPage,
		Pages:        pages,
	}
}

func buildPlanets() ([]planetDef, []chapterDef) {
	defs := []struct {
		grade  int
		code   string
		enName string
		idName string
		enHero string
		idHero string
		color  string
	}{
		{1, "terra_pebble", "Terra Pebble", "Planet Kerikil", "Miko the Star Fox", "Miko si Rubah Bintang", "#10B981"},
		{2, "tropica", "Planet Tropica", "Planet Tropika", "Kiki of the Canopy", "Kiki dari Hutan Awan", "#14B8A6"},
		{3, "aquaris", "Planet Aquaris", "Planet Akuaris", "Nori the Reef Diver", "Nori Penyelam Karang", "#0EA5E9"},
		{4, "crystalia", "Planet Crystalia", "Planet Kristalia", "Zara & the Prism Bridge", "Zara & Jembatan Prisma", "#A855F7"},
		{5, "lumina", "Planet Lumina", "Planet Lumina", "Taro of Neon Rail", "Taro dari Rel Neon", "#22D3EE"},
		{6, "solaris", "Planet Solaris", "Planet Solaris", "Aria at the Citadel of Light", "Aria di Benteng Cahaya", "#FBBF24"},
	}

	planets := make([]planetDef, 0, len(defs))
	chapters := make([]chapterDef, 0, len(defs)*3)
	for _, d := range defs {
		pName := mathengine.LocaleText{EN: d.enName, ID: d.idName}
		hero := mathengine.LocaleText{EN: d.enHero, ID: d.idHero}
		planets = append(planets, planetDef{grade: d.grade, code: d.code, name: pName, accentColor: d.color})
		chapters = append(chapters,
			chapterDef{
				code: d.code + "-ch0", planetCode: d.code, index: 0, unlockCostAC: 0, pageCount: 20,
				title:    mathengine.LocaleText{EN: "Opening Chapter — " + d.enHero, ID: "Bab Pembuka — " + d.idHero},
				heroName: hero, planetName: pName, grade: d.grade, accentColor: d.color,
			},
			chapterDef{
				code: d.code + "-ch1", planetCode: d.code, index: 1, unlockCostAC: ChapterUnlockCostAC, pageCount: 20,
				title:    mathengine.LocaleText{EN: "Chapter 1 — Hidden Orbit", ID: "Bab 1 — Orbit Rahasia"},
				heroName: hero, planetName: pName, grade: d.grade, accentColor: d.color,
			},
			chapterDef{
				code: d.code + "-ch2", planetCode: d.code, index: 2, unlockCostAC: ChapterUnlockCostAC, pageCount: 20,
				title:    mathengine.LocaleText{EN: "Chapter 2 — Star Core Mystery", ID: "Bab 2 — Misteri Inti Bintang"},
				heroName: hero, planetName: pName, grade: d.grade, accentColor: d.color,
			},
		)
	}
	return planets, chapters
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

// resolve returns the authenticated student ID and locale. Missing or invalid
// credentials map to HTTP 401 and non-student principals to HTTP 403; there is
// no shared demo identity in production routes.
func (h *Handler) resolve(ctx context.Context, in authIn) (string, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	ident, err := h.authSvc.ResolveStudent(ctx, tok)
	if err != nil {
		return "", locale, auth.StudentAuthHTTPError(err)
	}
	return ident.ID, locale, nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "stories-list-planets",
		Method:      http.MethodGet,
		Path:        "/api/v1/stories/planets",
		Summary:     "List all 6 Planet Stories worlds and chapter unlock statuses",
		Tags:        []string{"Stories"},
	}, func(ctx context.Context, in *authIn) (*struct {
		Body struct {
			Planets []PlanetWorld `json:"planets"`
		}
	}, error) {
		id, locale, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		out := &struct {
			Body struct {
				Planets []PlanetWorld `json:"planets"`
			}
		}{}
		out.Body.Planets = h.svc.ListPlanets(id, locale)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "stories-open-chapter",
		Method:      http.MethodGet,
		Path:        "/api/v1/stories/chapters/{code}",
		Summary:     "Open an unlocked 20-page Planet Stories digital storybook",
		Tags:        []string{"Stories"},
	}, func(ctx context.Context, in *struct {
		authIn
		Code string `path:"code"`
	}) (*struct{ Body ChapterViewer }, error) {
		id, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		viewer, err := h.svc.OpenChapter(id, in.Code, locale)
		if err != nil {
			if errors.Is(err, economy.ErrInsufficientFunds) {
				return nil, huma.Error403Forbidden("chapter locked; unlock for 20 Astro Coins")
			}
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body ChapterViewer }{Body: viewer}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "stories-unlock-chapter",
		Method:      http.MethodPost,
		Path:        "/api/v1/stories/chapters/{code}/unlock",
		Summary:     "Atomically deduct 20 Astro Coins to unlock a Planet Stories chapter",
		Tags:        []string{"Stories"},
	}, func(ctx context.Context, in *struct {
		authIn
		Code string `path:"code"`
	}) (*struct{ Body ChapterViewer }, error) {
		id, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		viewer, err := h.svc.UnlockChapter(ctx, id, in.Code, locale)
		if err != nil {
			if errors.Is(err, economy.ErrInsufficientFunds) {
				return nil, huma.Error400BadRequest("insufficient Astro Coins (20 AC required)")
			}
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body ChapterViewer }{Body: viewer}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "stories-bookmark",
		Method:      http.MethodPatch,
		Path:        "/api/v1/stories/chapters/{code}/bookmark",
		Summary:     "Bookmark current storybook page number",
		Tags:        []string{"Stories"},
	}, func(ctx context.Context, in *struct {
		authIn
		Code string `path:"code"`
		Body struct {
			Page int `json:"page" minimum:"1" maximum:"20"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		id, _, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		if err := h.svc.BookmarkPage(id, in.Code, in.Body.Page); err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		out := &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{}
		out.Body.OK = true
		return out, nil
	})
}
