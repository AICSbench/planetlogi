package leaderboards

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

const (
	RefreshIntervalSec = 300 // 5 minutes
	DailyBonusGiftAC   = 2
	SundayMeteorCount  = 10
	SundayMeteorSPPerQ = 5
)

var (
	ErrAlreadyClaimedToday = errors.New("daily bonus gift already claimed today")
	ErrEventNotFound       = errors.New("sunday meteor session not found")
)

type ChallengeMeta struct {
	Code        string `json:"code"`
	Category    string `json:"category"` // OPEN | SEASONAL
	Title       string `json:"title"`
	Description string `json:"description"`
	MetricLabel string `json:"metricLabel"`
	Icon        string `json:"icon"`
	Active      bool   `json:"active"`
}

type LeaderboardEntry struct {
	Rank       int    `json:"rank"`
	StudentID  string `json:"studentId"`
	MaskedName string `json:"maskedName"`
	Grade      int    `json:"grade"`
	SchoolCode string `json:"schoolCode"`
	ClassCode  string `json:"classCode"`
	BuddyColor string `json:"buddyColor"`
	Score      int64  `json:"score"`
	IsCurrent  bool   `json:"isCurrent"`
}

type LeaderboardBoard struct {
	Challenge      ChallengeMeta      `json:"challenge"`
	Scope          string             `json:"scope"` // CLASS | SCHOOL | NATIONAL
	Grade          int                `json:"grade"`
	ViewMode       string             `json:"viewMode"` // TOP50 | NEARBY
	TotalPilots    int                `json:"totalPilots"`
	CurrentRank    int                `json:"currentRank"`
	RefreshedAt    time.Time          `json:"refreshedAt"`
	NextRefreshSec int                `json:"nextRefreshSec"`
	Entries        []LeaderboardEntry `json:"entries"`
	HallOfFame     []HallOfFameEntry  `json:"hallOfFame"`
}

type HallOfFameEntry struct {
	Season     string `json:"season"`
	Challenge  string `json:"challenge"`
	WinnerName string `json:"winnerName"`
	Grade      int    `json:"grade"`
	Score      int64  `json:"score"`
}

type NotificationItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Category  string    `json:"category"` // DUEL | STREAK | EVENT | REWARD
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

type BonusGiftStatus struct {
	LocalDate string `json:"localDate"`
	Claimed   bool   `json:"claimed"`
	RewardAC  int64  `json:"rewardAc"`
}

type CreatorSubmission struct {
	ID          string    `json:"id"`
	AuthorMask  string    `json:"authorMask"`
	Grade       int       `json:"grade"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Status      string    `json:"status"` // PENDING | APPROVED
	SubmittedAt time.Time `json:"submittedAt"`
}

type EventsHub struct {
	SundayMeteorOpen   bool                `json:"sundayMeteorOpen"`
	SundayWindowNotice string              `json:"sundayWindowNotice"`
	CreatorQuarter     string              `json:"creatorQuarter"`
	CreatorTheme       string              `json:"creatorTheme"`
	CreatorEntries     []CreatorSubmission `json:"creatorEntries"`
}

type SundayMeteorStart struct {
	SessionID string                        `json:"sessionId"`
	Questions []mathengine.RenderedQuestion `json:"questions"`
}

type SundayMeteorResult struct {
	SessionID string `json:"sessionId"`
	Correct   int    `json:"correct"`
	Total     int    `json:"total"`
	RewardSP  int64  `json:"rewardSp"`
}

type meteorSession struct {
	id        string
	studentID string
	templates []string
	seeds     []int64
	done      bool
}

type Service struct {
	mu              sync.Mutex
	ledger          economy.Ledger
	pilots          []LeaderboardEntry
	bonusClaimed    map[string]bool // studentID:YYYY-MM-DD
	notifications   map[string][]NotificationItem
	creatorList     []CreatorSubmission            // approved entries only (public)
	pendingCreators map[string][]CreatorSubmission // studentID -> moderation queue
	meteorSess      map[string]*meteorSession
	seq             int
	nowClock        func() time.Time
}

func NewService(ledger economy.Ledger) *Service {
	s := &Service{
		ledger:          ledger,
		pilots:          seedPilots(120),
		bonusClaimed:    make(map[string]bool),
		notifications:   make(map[string][]NotificationItem),
		pendingCreators: make(map[string][]CreatorSubmission),
		meteorSess:      make(map[string]*meteorSession),
		nowClock:        time.Now,
	}
	s.creatorList = []CreatorSubmission{
		{
			ID:          "cr-1",
			AuthorMask:  "Maya K.",
			Grade:       3,
			Title:       "Rocket Fuel Fraction Relay",
			Summary:     "Combine 1/4 and 3/4 fuel canisters to launch the PlanetLogi shuttle!",
			Status:      "APPROVED",
			SubmittedAt: time.Now().Add(-48 * time.Hour),
		},
		{
			ID:          "cr-2",
			AuthorMask:  "Budi P.",
			Grade:       4,
			Title:       "Crater Perimeter Adventure",
			Summary:     "Measure rectangular moon bases to shield Astro Buddies from meteor showers.",
			Status:      "APPROVED",
			SubmittedAt: time.Now().Add(-24 * time.Hour),
		},
	}
	return s
}

func seedPilots(count int) []LeaderboardEntry {
	firstNames := []string{
		"Maya", "Ethan", "Raka", "Siti", "Budi", "Aisha", "Nadia", "Lucas",
		"Putri", "Ryan", "Hana", "Fajar", "Chloe", "Dimas", "Zara", "Kevin",
	}
	initials := []string{"K.", "L.", "S.", "R.", "P.", "N.", "W.", "T.", "D.", "C.", "M.", "B.", "Z.", "A."}
	colors := []string{"#6366F1", "#22D3EE", "#10B981", "#FBBF24", "#F97316", "#EC4899", "#A855F7"}
	schools := []string{"SCH-01", "SCH-02", "SCH-03"}
	classes := []string{"CLS-1A", "CLS-1B", "CLS-2A"}

	out := make([]LeaderboardEntry, 0, count)
	for i := 0; i < count; i++ {
		grade := (i % 6) + 1
		fn := firstNames[i%len(firstNames)]
		init := initials[(i/len(firstNames)+i)%len(initials)]
		out = append(out, LeaderboardEntry{
			StudentID:  fmt.Sprintf("pilot-%05d", i+1),
			MaskedName: fn + " " + init,
			Grade:      grade,
			SchoolCode: schools[i%len(schools)],
			ClassCode:  classes[i%len(classes)],
			BuddyColor: colors[i%len(colors)],
			Score:      int64(250 - (i*7)%230 + grade*10),
		})
	}
	return out
}

func (s *Service) SeedPilotsForBenchmark(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pilots = seedPilots(count)
}

func (s *Service) SetClockForTest(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowClock = fn
}

func (s *Service) ListChallenges(locale string) []ChallengeMeta {
	return []ChallengeMeta{
		{Code: "HERO_OF_DAY", Category: "OPEN", Title: mathengine.LocaleText{EN: "Hero of the Day", ID: "Pahlawan Hari Ini"}.For(locale), Description: mathengine.LocaleText{EN: "Top daily Star Points earners across the galaxy", ID: "Peraih Star Points harian tertinggi di galaksi"}.For(locale), MetricLabel: "SP", Icon: "🌟", Active: true},
		{Code: "TOP_BRAIN", Category: "OPEN", Title: mathengine.LocaleText{EN: "Top Brain", ID: "Otak Terbaik"}.For(locale), Description: mathengine.LocaleText{EN: "Highest weekly total Star Points", ID: "Total Star Points mingguan tertinggi"}.For(locale), MetricLabel: "SP", Icon: "🧠", Active: true},
		{Code: "CHAMPION_CLASS", Category: "OPEN", Title: mathengine.LocaleText{EN: "Champion Class", ID: "Kelas Juara"}.For(locale), Description: mathengine.LocaleText{EN: "Combined Star Points for your classroom squad", ID: "Gabungan Star Points untuk regu kelasmu"}.For(locale), MetricLabel: "Class SP", Icon: "🏫", Active: true},
		{Code: "STAR_CREW", Category: "OPEN", Title: mathengine.LocaleText{EN: "Star Crew", ID: "Kru Bintang"}.For(locale), Description: mathengine.LocaleText{EN: "Best-friend crew weekly points race", ID: "Balapan poin mingguan kru sahabat"}.For(locale), MetricLabel: "Crew SP", Icon: "🤝", Active: true},
		{Code: "SUPER_NOVA", Category: "OPEN", Title: mathengine.LocaleText{EN: "Super Nova", ID: "Super Nova"}.For(locale), Description: mathengine.LocaleText{EN: "Weekly Super Sight & Super Warp diamond S score", ID: "Skor berlian S mingguan Super Sight & Super Warp"}.For(locale), MetricLabel: "S", Icon: "💎", Active: true},
		{Code: "ORBIT_STREAK", Category: "OPEN", Title: mathengine.LocaleText{EN: "Orbit Streak", ID: "Streak Orbit"}.For(locale), Description: mathengine.LocaleText{EN: "Longest unbroken daily learning streaks", ID: "Rekor hari belajar beruntun terpanjang"}.For(locale), MetricLabel: "Days", Icon: "🔥", Active: true},
		{Code: "ZODIAC_WARP", Category: "SEASONAL", Title: mathengine.LocaleText{EN: "Zodiac Warp", ID: "Warp Zodiak"}.For(locale), Description: mathengine.LocaleText{EN: "Lunar New Year cosmic constellation marathon", ID: "Maraton rasi bintang Tahun Baru Imlek"}.For(locale), MetricLabel: "Stars", Icon: "🐉", Active: true},
		{Code: "COMET_VS_METEOR", Category: "SEASONAL", Title: mathengine.LocaleText{EN: "Comet vs Meteor", ID: "Komet vs Meteor"}.For(locale), Description: mathengine.LocaleText{EN: "Two-team galaxy faction showdown!", ID: "Pertarungan dua tim faksi galaksi!"}.For(locale), MetricLabel: "Faction SP", Icon: "☄️", Active: true},
		{Code: "STAR_ACADEMY_CUP", Category: "SEASONAL", Title: mathengine.LocaleText{EN: "Star Academy Cup", ID: "Piala Akademi Bintang"}.For(locale), Description: mathengine.LocaleText{EN: "Inter-school seasonal mastery tournament", ID: "Turnamen penguasaan antarsekolah musiman"}.For(locale), MetricLabel: "Cup SP", Icon: "🏆", Active: true},
		{Code: "GALACTIC_CITY_RACE", Category: "SEASONAL", Title: mathengine.LocaleText{EN: "Galactic City Race", ID: "Balapan Kota Galaksi"}.For(locale), Description: mathengine.LocaleText{EN: "Regional city vs city math relay", ID: "Estafet matematika antar kota regional"}.For(locale), MetricLabel: "City SP", Icon: "🌆", Active: true},
	}
}

func challengeMultiplier(code string) int64 {
	switch strings.ToUpper(code) {
	case "TOP_BRAIN":
		return 3
	case "CHAMPION_CLASS":
		return 5
	case "STAR_CREW":
		return 2
	case "SUPER_NOVA":
		return 4
	default:
		return 1
	}
}

func (s *Service) GetBoard(studentID, displayName string, studentGrade int, challengeCode, scope string, filterGrade int, viewMode, locale string) LeaderboardBoard {
	s.mu.Lock()
	pilotsCopy := make([]LeaderboardEntry, len(s.pilots))
	copy(pilotsCopy, s.pilots)
	now := s.nowClock()
	s.mu.Unlock()

	challenges := s.ListChallenges(locale)
	meta := challenges[0]
	for _, c := range challenges {
		if strings.EqualFold(c.Code, challengeCode) {
			meta = c
			break
		}
	}

	scope = strings.ToUpper(strings.TrimSpace(scope))
	if scope != "CLASS" && scope != "SCHOOL" && scope != "NATIONAL" {
		scope = "NATIONAL"
	}
	viewMode = strings.ToUpper(strings.TrimSpace(viewMode))
	if viewMode != "NEARBY" {
		viewMode = "TOP50"
	}

	mult := challengeMultiplier(meta.Code)
	currentMask := maskName(displayName)
	if studentGrade < 1 || studentGrade > 6 {
		studentGrade = 1
	}
	filtered := make([]LeaderboardEntry, 0, len(pilotsCopy)+1)
	if filterGrade == 0 || filterGrade == studentGrade {
		filtered = append(filtered, LeaderboardEntry{
			StudentID:  studentID,
			MaskedName: currentMask,
			Grade:      studentGrade,
			SchoolCode: "SCH-01",
			ClassCode:  "CLS-1A",
			BuddyColor: "#6366F1",
			Score:      92 * mult,
			IsCurrent:  true,
		})
	}
	for _, p := range pilotsCopy {
		if filterGrade >= 1 && filterGrade <= 6 && p.Grade != filterGrade {
			continue
		}
		if scope == "SCHOOL" && p.SchoolCode != "SCH-01" {
			continue
		}
		if scope == "CLASS" && (p.SchoolCode != "SCH-01" || p.ClassCode != "CLS-1A") {
			continue
		}
		p.Score = p.Score * mult
		if meta.Code == "ORBIT_STREAK" {
			p.Score = (p.Score % 45) + 1
		}
		filtered = append(filtered, p)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Score == filtered[j].Score {
			return filtered[i].StudentID < filtered[j].StudentID
		}
		return filtered[i].Score > filtered[j].Score
	})

	currentIdx := -1
	for i := range filtered {
		filtered[i].Rank = i + 1
		if filtered[i].IsCurrent {
			currentIdx = i
		}
	}

	var entries []LeaderboardEntry
	if viewMode == "NEARBY" && currentIdx >= 0 {
		start := currentIdx - 5
		if start < 0 {
			start = 0
		}
		end := currentIdx + 6
		if end > len(filtered) {
			end = len(filtered)
		}
		entries = filtered[start:end]
	} else {
		limit := 50
		if len(filtered) < limit {
			limit = len(filtered)
		}
		entries = filtered[:limit]
	}

	secInBucket := now.Unix() % RefreshIntervalSec
	nextRefresh := int(RefreshIntervalSec - secInBucket)
	refreshedAt := now.Add(-time.Duration(secInBucket) * time.Second).Truncate(time.Second)

	hall := []HallOfFameEntry{
		{Season: "2026-Q3", Challenge: meta.Title, WinnerName: "Maya K.", Grade: 4, Score: 1480},
		{Season: "2026-Q2", Challenge: meta.Title, WinnerName: "Dimas A.", Grade: 6, Score: 1395},
		{Season: "2026-Q1", Challenge: meta.Title, WinnerName: "Chloe Z.", Grade: 5, Score: 1320},
	}

	currentRank := 0
	if currentIdx >= 0 {
		currentRank = currentIdx + 1
	}

	return LeaderboardBoard{
		Challenge:      meta,
		Scope:          scope,
		Grade:          filterGrade,
		ViewMode:       viewMode,
		TotalPilots:    len(filtered),
		CurrentRank:    currentRank,
		RefreshedAt:    refreshedAt,
		NextRefreshSec: nextRefresh,
		Entries:        entries,
		HallOfFame:     hall,
	}
}

func maskName(full string) string {
	parts := strings.Fields(strings.TrimSpace(full))
	if len(parts) == 0 {
		return "Pilot P."
	}
	if len(parts) == 1 {
		return parts[0] + " P."
	}
	last := parts[len(parts)-1]
	return parts[0] + " " + strings.ToUpper(last[:1]) + "."
}

func (s *Service) GetBonusGift(studentID, tz string) BonusGiftStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	localDate := timewindow.LocalDateString(s.nowClock(), tz)
	key := studentID + ":" + localDate
	return BonusGiftStatus{
		LocalDate: localDate,
		Claimed:   s.bonusClaimed[key],
		RewardAC:  DailyBonusGiftAC,
	}
}

func (s *Service) ClaimBonusGift(ctx context.Context, studentID, tz string) (BonusGiftStatus, error) {
	s.mu.Lock()
	localDate := timewindow.LocalDateString(s.nowClock(), tz)
	key := studentID + ":" + localDate
	if s.bonusClaimed[key] {
		s.mu.Unlock()
		return BonusGiftStatus{LocalDate: localDate, Claimed: true, RewardAC: DailyBonusGiftAC}, ErrAlreadyClaimedToday
	}
	s.bonusClaimed[key] = true
	s.mu.Unlock()

	if s.ledger != nil {
		_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencyAC,
			Delta:          DailyBonusGiftAC,
			SourceType:     "DAILY_BONUS_GIFT",
			SourceID:       localDate,
			IdempotencyKey: fmt.Sprintf("bonus-gift:%s:%s", studentID, localDate),
		})
	}
	return BonusGiftStatus{LocalDate: localDate, Claimed: true, RewardAC: DailyBonusGiftAC}, nil
}

func (s *Service) ListNotifications(studentID, locale string) []NotificationItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	if items, ok := s.notifications[studentID]; ok {
		out := make([]NotificationItem, len(items))
		copy(out, items)
		return out
	}
	now := s.nowClock()
	seed := []NotificationItem{
		{
			ID:        "notif-1",
			Category:  "DUEL",
			Title:     mathengine.LocaleText{EN: "New Star Duel Challenge!", ID: "Tantangan Duel Bintang Baru!"}.For(locale),
			Body:      mathengine.LocaleText{EN: "Maya K. challenged you to a 5-question math duel.", ID: "Maya K. menantangmu dalam duel matematika 5 soal."}.For(locale),
			Read:      false,
			CreatedAt: now.Add(-15 * time.Minute),
		},
		{
			ID:        "notif-2",
			Category:  "REWARD",
			Title:     mathengine.LocaleText{EN: "Daily Bonus Gift Ready!", ID: "Hadiah Bonus Harian Siap!"}.For(locale),
			Body:      mathengine.LocaleText{EN: "Tap the gift box in the top bar to claim +2 Astro Coins.", ID: "Ketuk kotak hadiah di bilah atas untuk klaim +2 Astro Coins."}.For(locale),
			Read:      false,
			CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:        "notif-3",
			Category:  "EVENT",
			Title:     mathengine.LocaleText{EN: "Sunday Meteor Challenge", ID: "Tantangan Meteor Minggu"}.For(locale),
			Body:      mathengine.LocaleText{EN: "Every Sunday 06:00-22:00: solve 10 bonus meteor questions for extra SP!", ID: "Setiap Minggu 06:00-22:00: jawab 10 soal meteor bonus untuk SP ekstra!"}.For(locale),
			Read:      true,
			CreatedAt: now.Add(-6 * time.Hour),
		},
	}
	s.notifications[studentID] = seed
	return seed
}

func (s *Service) MarkAllNotificationsRead(studentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.notifications[studentID] {
		s.notifications[studentID][i].Read = true
	}
}

func (s *Service) GetEventsHub(tz, locale string) EventsHub {
	s.mu.Lock()
	defer s.mu.Unlock()
	eval := timewindow.Evaluate(s.nowClock(), tz, timewindow.SundayMeteorWindow)

	notice := mathengine.LocaleText{
		EN: "Open every Sunday 06:00–22:00 local time (practice preview always open during pilot).",
		ID: "Buka setiap Minggu 06:00–22:00 waktu lokal (pratinjau latihan selalu terbuka selama pilot).",
	}.For(locale)
	entries := make([]CreatorSubmission, len(s.creatorList))
	copy(entries, s.creatorList)
	return EventsHub{
		SundayMeteorOpen:   eval.IsOpen,
		SundayWindowNotice: notice,
		CreatorQuarter:     "2026-Q4",
		CreatorTheme: mathengine.LocaleText{
			EN: "Design a Space Station Word Problem using Fractions or Area!",
			ID: "Buat Soal Cerita Stasiun Luar Angkasa menggunakan Pecahan atau Luas!",
		}.For(locale),
		CreatorEntries: entries,
	}
}

func (s *Service) StartSundayMeteor(studentID string, grade int, locale string) SundayMeteorStart {
	s.mu.Lock()
	defer s.mu.Unlock()
	if grade < 1 || grade > 6 {
		grade = 1
	}
	list := mathengine.TemplatesForGrade(grade)
	codes := make([]string, 0, len(list))
	for _, t := range list {
		codes = append(codes, t.Code)
	}
	if len(codes) == 0 {
		codes = []string{"g1_add_within_20", "g1_sub_within_20"}
	}
	s.seq++
	sid := fmt.Sprintf("meteor-%d", s.seq)
	tpls := make([]string, SundayMeteorCount)
	seeds := make([]int64, SundayMeteorCount)
	qs := make([]mathengine.RenderedQuestion, 0, SundayMeteorCount)
	for i := 0; i < SundayMeteorCount; i++ {
		tpls[i] = codes[i%len(codes)]
		seeds[i] = int64(7000 + s.seq*41 + i*19)
		if prob, ok := mathengine.GenerateByCode(tpls[i], seeds[i], locale); ok {
			qs = append(qs, prob.Question)
		}
	}
	s.meteorSess[sid] = &meteorSession{
		id:        sid,
		studentID: studentID,
		templates: tpls,
		seeds:     seeds,
	}
	return SundayMeteorStart{SessionID: sid, Questions: qs}
}

func (s *Service) SubmitSundayMeteor(ctx context.Context, studentID, sessionID, locale string, answers map[int]map[string]string) (SundayMeteorResult, error) {
	s.mu.Lock()
	sess, ok := s.meteorSess[sessionID]
	if !ok || sess.studentID != studentID {
		s.mu.Unlock()
		return SundayMeteorResult{}, ErrEventNotFound
	}
	correct := 0
	for i, tpl := range sess.templates {
		if ans := answers[i]; len(ans) > 0 {
			if prob, ok := mathengine.GenerateByCode(tpl, sess.seeds[i], locale); ok {
				if mathengine.Grade(prob, ans).IsCorrect {
					correct++
				}
			}
		}
	}
	sess.done = true
	rewardSP := int64(correct * SundayMeteorSPPerQ)
	s.mu.Unlock()

	if s.ledger != nil && rewardSP > 0 {
		_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencySP,
			Delta:          rewardSP,
			SourceType:     "SUNDAY_METEOR",
			SourceID:       sessionID,
			IdempotencyKey: fmt.Sprintf("sunday-meteor:%s:%s", sessionID, studentID),
		})
	}
	return SundayMeteorResult{
		SessionID: sessionID,
		Correct:   correct,
		Total:     len(sess.templates),
		RewardSP:  rewardSP,
	}, nil
}

// SubmitCreatorChallenge stores a child's submission in a private moderation
// queue with status PENDING. Nothing is added to the public Creator Entries
// list until a moderator approves it, so raw child-submitted text (phone
// numbers, emails, unsafe content) never reaches other readers unreviewed.
func (s *Service) SubmitCreatorChallenge(studentID, displayName string, grade int, title, summary string) CreatorSubmission {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	entry := CreatorSubmission{
		ID:          fmt.Sprintf("cr-%d", s.seq+10),
		AuthorMask:  maskName(displayName),
		Grade:       grade,
		Title:       strings.TrimSpace(title),
		Summary:     strings.TrimSpace(summary),
		Status:      "PENDING",
		SubmittedAt: s.nowClock(),
	}
	s.pendingCreators[studentID] = append(s.pendingCreators[studentID], entry)
	return entry
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

// resolve returns the authenticated student identity. Grade and timezone come
// from the trusted principal — never from client query strings — and missing,
// invalid, or non-student credentials are rejected.
func (h *Handler) resolve(ctx context.Context, in authIn) (auth.StudentIdentity, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	ident, err := h.authSvc.ResolveStudent(ctx, tok)
	if err != nil {
		return auth.StudentIdentity{}, locale, auth.StudentAuthHTTPError(err)
	}
	return ident, locale, nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "leaderboards-challenges",
		Method:      http.MethodGet,
		Path:        "/api/v1/leaderboards/challenges",
		Summary:     "List 6 Open Challenges and 4 Seasonal Challenges",
		Tags:        []string{"Leaderboards"},
	}, func(_ context.Context, in *struct {
		Locale string `query:"locale" default:"en"`
	}) (*struct {
		Body struct {
			Challenges []ChallengeMeta `json:"challenges"`
		}
	}, error) {
		out := &struct {
			Body struct {
				Challenges []ChallengeMeta `json:"challenges"`
			}
		}{}
		out.Body.Challenges = h.svc.ListChallenges(in.Locale)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "leaderboards-board",
		Method:      http.MethodGet,
		Path:        "/api/v1/leaderboards/{challenge}",
		Summary:     "Get Galaxy Leaderboard (Class/School/National, Top 50 or Nearby +/-5, masked names)",
		Tags:        []string{"Leaderboards"},
	}, func(ctx context.Context, in *struct {
		authIn
		Challenge   string `path:"challenge"`
		Scope       string `query:"scope" default:"NATIONAL"`
		FilterGrade int    `query:"filterGrade" default:"0"`
		ViewMode    string `query:"viewMode" default:"TOP50"`
	}) (*struct{ Body LeaderboardBoard }, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		board := h.svc.GetBoard(ident.ID, ident.DisplayName, ident.Grade, in.Challenge, in.Scope, in.FilterGrade, in.ViewMode, locale)
		return &struct{ Body LeaderboardBoard }{Body: board}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/notifications",
		Summary:     "List student notifications",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, in *authIn) (*struct {
		Body struct {
			Items []NotificationItem `json:"items"`
		}
	}, error) {
		ident, locale, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		out := &struct {
			Body struct {
				Items []NotificationItem `json:"items"`
			}
		}{}
		out.Body.Items = h.svc.ListNotifications(ident.ID, locale)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "notifications-mark-read",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/read",
		Summary:     "Mark all notifications as read",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, in *authIn) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		ident, _, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		h.svc.MarkAllNotificationsRead(ident.ID)
		out := &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{}
		out.Body.OK = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "bonus-gift-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/bonus-gift",
		Summary:     "Get Daily Bonus Gift status",
		Tags:        []string{"Events"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body BonusGiftStatus }, error) {
		ident, _, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		return &struct{ Body BonusGiftStatus }{Body: h.svc.GetBonusGift(ident.ID, ident.Timezone)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "bonus-gift-claim",
		Method:      http.MethodPost,
		Path:        "/api/v1/bonus-gift/claim",
		Summary:     "Claim Daily Bonus Gift (+2 Astro Coins, idempotent once per day)",
		Tags:        []string{"Events"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body BonusGiftStatus }, error) {
		ident, _, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		status, err := h.svc.ClaimBonusGift(ctx, ident.ID, ident.Timezone)
		if err != nil {
			if errors.Is(err, ErrAlreadyClaimedToday) {
				return nil, huma.Error409Conflict(err.Error())
			}
			return nil, err
		}
		return &struct{ Body BonusGiftStatus }{Body: status}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "events-hub",
		Method:      http.MethodGet,
		Path:        "/api/v1/events/hub",
		Summary:     "Get Cosmic Events hub (Sunday Meteor Challenge + Quarterly Creator Challenge)",
		Tags:        []string{"Events"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body EventsHub }, error) {
		ident, locale, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		return &struct{ Body EventsHub }{Body: h.svc.GetEventsHub(ident.Timezone, locale)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "events-sunday-meteor-start",
		Method:      http.MethodPost,
		Path:        "/api/v1/events/sunday-meteor/start",
		Summary:     "Start a 10-question Sunday Meteor Challenge set",
		Tags:        []string{"Events"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body SundayMeteorStart }, error) {
		ident, locale, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		start := h.svc.StartSundayMeteor(ident.ID, ident.Grade, locale)
		return &struct{ Body SundayMeteorStart }{Body: start}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "events-sunday-meteor-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/events/sunday-meteor/submit",
		Summary:     "Grade Sunday Meteor Challenge answers and award SP",
		Tags:        []string{"Events"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			SessionID string                    `json:"sessionId"`
			Answers   map[int]map[string]string `json:"answers"`
		}
	}) (*struct{ Body SundayMeteorResult }, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		res, err := h.svc.SubmitSundayMeteor(ctx, ident.ID, in.Body.SessionID, locale, in.Body.Answers)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body SundayMeteorResult }{Body: res}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "events-creator-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/events/creator-challenge/submit",
		Summary:     "Submit a Quarterly Creator Challenge entry (masked author name)",
		Tags:        []string{"Events"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			Title   string `json:"title" minLength:"3" maxLength:"100"`
			Summary string `json:"summary" minLength:"10" maxLength:"400"`
		}
	}) (*struct{ Body CreatorSubmission }, error) {
		ident, _, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		entry := h.svc.SubmitCreatorChallenge(ident.ID, ident.DisplayName, ident.Grade, in.Body.Title, in.Body.Summary)
		return &struct{ Body CreatorSubmission }{Body: entry}, nil
	})
}
