package social

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
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
	DailyEnergyMax    = 3
	DuelQuestionCount = 5
	DuelWinSP         = 12
	DuelDrawSP        = 6
	DuelLossSP        = 3
)

var (
	ErrNoEnergy      = errors.New("no Star Duel energy cells remaining today (resets at midnight)")
	ErrDuelNotFound  = errors.New("star duel not found")
	ErrDuelCompleted = errors.New("star duel already completed")
	ErrCrewNotFound  = errors.New("star crew code not found")
	ErrAlreadyInCrew = errors.New("already a member of this star crew")
	ErrInvalidCode   = errors.New("star crew code must be 9 digits")
)

type OpponentProfile struct {
	ID         string `json:"id"`
	MaskedName string `json:"maskedName"`
	Grade      int    `json:"grade"`
	Relation   string `json:"relation"` // CLASSMATE | CREW | RANDOM
	BuddyColor string `json:"buddyColor"`
	BuddyName  string `json:"buddyName"`
	WeeklySP   int64  `json:"weeklySp"`
}

type DuelRecord struct {
	ID            string                        `json:"id"`
	Mode          string                        `json:"mode"`      // CLASSMATE | CREW | RANDOM
	Direction     string                        `json:"direction"` // OUTGOING | INCOMING
	Opponent      OpponentProfile               `json:"opponent"`
	Status        string                        `json:"status"`            // PENDING | IN_PROGRESS | COMPLETED
	Outcome       string                        `json:"outcome,omitempty"` // WIN | DRAW | LOSS
	PlayerScore   int                           `json:"playerScore"`
	OpponentScore int                           `json:"opponentScore"`
	RewardSP      int64                         `json:"rewardSp"`
	CreatedAt     time.Time                     `json:"createdAt"`
	CompletedAt   *time.Time                    `json:"completedAt,omitempty"`
	Questions     []mathengine.RenderedQuestion `json:"questions,omitempty"`
}

type DuelLobby struct {
	LocalDate  string            `json:"localDate"`
	EnergyLeft int               `json:"energyLeft"`
	EnergyMax  int               `json:"energyMax"`
	Opponents  []OpponentProfile `json:"opponents"`
	Incoming   []DuelRecord      `json:"incoming"`
	History    []DuelRecord      `json:"history"`
	Rules      []string          `json:"rules"`
}

type CrewMember struct {
	StudentID  string `json:"studentId"`
	MaskedName string `json:"maskedName"`
	Grade      int    `json:"grade"`
	BuddyColor string `json:"buddyColor"`
	WeeklySP   int64  `json:"weeklySp"`
	IsLeader   bool   `json:"isLeader"`
}

type StarCrewView struct {
	CrewCode      string       `json:"crewCode"`
	FormattedCode string       `json:"formattedCode"`
	Name          string       `json:"name"`
	WeeklyTotalSP int64        `json:"weeklyTotalSp"`
	Members       []CrewMember `json:"members"`
}

type duelInternal struct {
	record    DuelRecord
	studentID string
	templates []string
	seeds     []int64
}

type crewInternal struct {
	code    string
	name    string
	members []CrewMember
}

type Service struct {
	mu           sync.Mutex
	ledger       economy.Ledger
	energyUsed   map[string]int // studentID:YYYY-MM-DD -> used
	duels        map[string]*duelInternal
	studentDuels map[string][]string
	crewsByCode  map[string]*crewInternal
	studentCrew  map[string]string // studentID -> crewCode
	peers        []OpponentProfile
	seq          int
	nowClock     func() time.Time
}

func NewService(ledger economy.Ledger) *Service {
	s := &Service{
		ledger:       ledger,
		energyUsed:   make(map[string]int),
		duels:        make(map[string]*duelInternal),
		studentDuels: make(map[string][]string),
		crewsByCode:  make(map[string]*crewInternal),
		studentCrew:  make(map[string]string),
		nowClock:     time.Now,
	}
	s.peers = seedPeers()
	s.crewsByCode["482910375"] = &crewInternal{
		code: "482910375",
		name: "Nebula Navigators",
		members: []CrewMember{
			{StudentID: "peer-g1-1", MaskedName: "Maya K.", Grade: 1, BuddyColor: "#22D3EE", WeeklySP: 68, IsLeader: true},
			{StudentID: "peer-g1-2", MaskedName: "Ethan L.", Grade: 1, BuddyColor: "#FBBF24", WeeklySP: 52, IsLeader: false},
		},
	}
	return s
}

func seedPeers() []OpponentProfile {
	return []OpponentProfile{
		{ID: "peer-g1-1", MaskedName: "Maya K.", Grade: 1, Relation: "CLASSMATE", BuddyColor: "#22D3EE", BuddyName: "Cosmic Maya", WeeklySP: 68},
		{ID: "peer-g1-2", MaskedName: "Ethan L.", Grade: 1, Relation: "CREW", BuddyColor: "#FBBF24", BuddyName: "Turbo Ethan", WeeklySP: 52},
		{ID: "peer-g1-3", MaskedName: "Aisha N.", Grade: 1, Relation: "RANDOM", BuddyColor: "#A855F7", BuddyName: "Starry Aisha", WeeklySP: 41},
		{ID: "peer-g2-1", MaskedName: "Siti R.", Grade: 2, Relation: "CLASSMATE", BuddyColor: "#EC4899", BuddyName: "Nova Siti", WeeklySP: 74},
		{ID: "peer-g2-2", MaskedName: "Budi P.", Grade: 2, Relation: "RANDOM", BuddyColor: "#10B981", BuddyName: "Rocket Budi", WeeklySP: 60},
		{ID: "peer-g3-1", MaskedName: "Nadia W.", Grade: 3, Relation: "CLASSMATE", BuddyColor: "#6366F1", BuddyName: "Solar Nadia", WeeklySP: 83},
		{ID: "peer-g3-2", MaskedName: "Lucas T.", Grade: 3, Relation: "RANDOM", BuddyColor: "#F97316", BuddyName: "Comet Lucas", WeeklySP: 71},
		{ID: "peer-g4-1", MaskedName: "Putri D.", Grade: 4, Relation: "CLASSMATE", BuddyColor: "#14B8A6", BuddyName: "Aurora Putri", WeeklySP: 95},
		{ID: "peer-g4-2", MaskedName: "Ryan C.", Grade: 4, Relation: "RANDOM", BuddyColor: "#3B82F6", BuddyName: "Quasar Ryan", WeeklySP: 88},
		{ID: "peer-g5-1", MaskedName: "Hana M.", Grade: 5, Relation: "CLASSMATE", BuddyColor: "#F43F5E", BuddyName: "Lunar Hana", WeeklySP: 110},
		{ID: "peer-g5-2", MaskedName: "Fajar B.", Grade: 5, Relation: "RANDOM", BuddyColor: "#8B5CF6", BuddyName: "Meteor Fajar", WeeklySP: 102},
		{ID: "peer-g6-1", MaskedName: "Chloe Z.", Grade: 6, Relation: "CLASSMATE", BuddyColor: "#06B6D4", BuddyName: "Prism Chloe", WeeklySP: 126},
		{ID: "peer-g6-2", MaskedName: "Dimas A.", Grade: 6, Relation: "RANDOM", BuddyColor: "#F59E0B", BuddyName: "Hyper Dimas", WeeklySP: 118},
	}
}

func (s *Service) SetClockForTest(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowClock = fn
}

func (s *Service) energyKey(studentID, tz string) (string, string) {
	localDate := timewindow.LocalDateString(s.nowClock(), tz)
	return studentID + ":" + localDate, localDate
}

func templatesForGrade(grade int) []string {
	if grade < 1 || grade > 6 {
		grade = 1
	}
	list := mathengine.TemplatesForGrade(grade)
	codes := make([]string, 0, DuelQuestionCount)
	for _, t := range list {
		codes = append(codes, t.Code)
	}
	if len(codes) == 0 {
		codes = []string{"g1_add_within_20", "g1_sub_within_20"}
	}
	out := make([]string, DuelQuestionCount)
	for i := 0; i < DuelQuestionCount; i++ {
		out[i] = codes[i%len(codes)]
	}
	return out
}

func (s *Service) ensureIncomingLocked(studentID string, grade int, locale string) {
	for _, id := range s.studentDuels[studentID] {
		if d := s.duels[id]; d != nil && d.record.Direction == "INCOMING" {
			return
		}
	}
	opp := s.pickOpponentLocked(grade, "CLASSMATE", "")
	s.seq++
	id := fmt.Sprintf("duel-inc-%d", s.seq)
	templates := templatesForGrade(grade)
	seeds := make([]int64, DuelQuestionCount)
	qs := make([]mathengine.RenderedQuestion, 0, DuelQuestionCount)
	for i := 0; i < DuelQuestionCount; i++ {
		seeds[i] = int64(9000 + s.seq*17 + i*31)
		if prob, ok := mathengine.GenerateByCode(templates[i], seeds[i], locale); ok {
			qs = append(qs, prob.Question)
		}
	}
	rec := DuelRecord{
		ID:            id,
		Mode:          "CLASSMATE",
		Direction:     "INCOMING",
		Opponent:      opp,
		Status:        "PENDING",
		OpponentScore: 3,
		CreatedAt:     s.nowClock().Add(-20 * time.Minute),
		Questions:     qs,
	}
	s.duels[id] = &duelInternal{
		record:    rec,
		studentID: studentID,
		templates: templates,
		seeds:     seeds,
	}
	s.studentDuels[studentID] = append([]string{id}, s.studentDuels[studentID]...)
}

func (s *Service) pickOpponentLocked(grade int, mode, opponentID string) OpponentProfile {
	if grade < 1 || grade > 6 {
		grade = 1
	}
	if opponentID != "" {
		for _, p := range s.peers {
			if p.ID == opponentID && p.Grade == grade {
				return p
			}
		}
	}
	for _, p := range s.peers {
		if p.Grade == grade && (mode == "" || p.Relation == mode) {
			return p
		}
	}
	for _, p := range s.peers {
		if p.Grade == grade {
			return p
		}
	}
	return s.peers[0]
}

func (s *Service) GetLobby(studentID string, grade int, tz, locale string) DuelLobby {
	s.mu.Lock()
	defer s.mu.Unlock()
	if grade < 1 || grade > 6 {
		grade = 1
	}
	s.ensureIncomingLocked(studentID, grade, locale)
	eKey, localDate := s.energyKey(studentID, tz)
	used := s.energyUsed[eKey]
	left := DailyEnergyMax - used
	if left < 0 {
		left = 0
	}

	opps := make([]OpponentProfile, 0)
	for _, p := range s.peers {
		if p.Grade == grade {
			opps = append(opps, p)
		}
	}

	incoming := make([]DuelRecord, 0)
	history := make([]DuelRecord, 0)
	for _, id := range s.studentDuels[studentID] {
		d := s.duels[id]
		if d == nil {
			continue
		}
		summary := d.record
		summary.Questions = nil
		if d.record.Direction == "INCOMING" && d.record.Status == "PENDING" {
			incoming = append(incoming, summary)
		} else if d.record.Status == "COMPLETED" {
			history = append(history, summary)
		}
	}

	rules := []string{
		mathengine.LocaleText{
			EN: "You receive 3 Energy Cells every day to launch Star Duels against pilots in your grade.",
			ID: "Kamu mendapat 3 Sel Energi setiap hari untuk memulai Duel Bintang melawan pilot di kelasmu.",
		}.For(locale),
		mathengine.LocaleText{
			EN: "Each duel has 5 quick math problems. Beat your opponent's score to earn +12 Star Points!",
			ID: "Setiap duel berisi 5 soal matematika cepat. Kalahkan skor lawanmu untuk meraih +12 Star Points!",
		}.For(locale),
		mathengine.LocaleText{
			EN: "All pilot names are safely masked and zero chat messages are allowed for a friendly galaxy.",
			ID: "Semua nama pilot disamarkan demi keamanan dan tanpa fitur chat agar galaksi tetap ramah.",
		}.For(locale),
	}

	return DuelLobby{
		LocalDate:  localDate,
		EnergyLeft: left,
		EnergyMax:  DailyEnergyMax,
		Opponents:  opps,
		Incoming:   incoming,
		History:    history,
		Rules:      rules,
	}
}

func (s *Service) StartDuel(studentID string, grade int, tz, locale, mode, opponentID string) (DuelRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if grade < 1 || grade > 6 {
		grade = 1
	}
	eKey, _ := s.energyKey(studentID, tz)
	if s.energyUsed[eKey] >= DailyEnergyMax {
		return DuelRecord{}, 0, ErrNoEnergy
	}

	mode = strings.ToUpper(strings.TrimSpace(mode))
	if mode != "CLASSMATE" && mode != "CREW" && mode != "RANDOM" {
		mode = "RANDOM"
	}
	opp := s.pickOpponentLocked(grade, mode, opponentID)
	s.energyUsed[eKey]++
	energyLeft := DailyEnergyMax - s.energyUsed[eKey]

	s.seq++
	id := fmt.Sprintf("duel-%d", s.seq)
	templates := templatesForGrade(grade)
	seeds := make([]int64, DuelQuestionCount)
	qs := make([]mathengine.RenderedQuestion, 0, DuelQuestionCount)
	for i := 0; i < DuelQuestionCount; i++ {
		seeds[i] = int64(5000 + s.seq*29 + i*13)
		if prob, ok := mathengine.GenerateByCode(templates[i], seeds[i], locale); ok {
			qs = append(qs, prob.Question)
		}
	}
	oppScore := 2 + (s.seq % 3)
	rec := DuelRecord{
		ID:            id,
		Mode:          mode,
		Direction:     "OUTGOING",
		Opponent:      opp,
		Status:        "IN_PROGRESS",
		OpponentScore: oppScore,
		CreatedAt:     s.nowClock(),
		Questions:     qs,
	}
	s.duels[id] = &duelInternal{
		record:    rec,
		studentID: studentID,
		templates: templates,
		seeds:     seeds,
	}
	s.studentDuels[studentID] = append([]string{id}, s.studentDuels[studentID]...)
	return rec, energyLeft, nil
}

func (s *Service) AcceptIncoming(studentID, duelID, locale string) (DuelRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.duels[duelID]
	if !ok || d.studentID != studentID {
		return DuelRecord{}, ErrDuelNotFound
	}
	if d.record.Status == "COMPLETED" {
		return DuelRecord{}, ErrDuelCompleted
	}
	d.record.Status = "IN_PROGRESS"
	qs := make([]mathengine.RenderedQuestion, 0, len(d.templates))
	for i, tpl := range d.templates {
		if prob, ok := mathengine.GenerateByCode(tpl, d.seeds[i], locale); ok {
			qs = append(qs, prob.Question)
		}
	}
	out := d.record
	out.Questions = qs
	return out, nil
}

func (s *Service) SubmitDuel(ctx context.Context, studentID, duelID, locale string, answers map[int]map[string]string) (DuelRecord, error) {
	s.mu.Lock()
	d, ok := s.duels[duelID]
	if !ok || d.studentID != studentID {
		s.mu.Unlock()
		return DuelRecord{}, ErrDuelNotFound
	}
	if d.record.Status == "COMPLETED" {
		rec := d.record
		s.mu.Unlock()
		return rec, nil
	}

	score := 0
	for i, tpl := range d.templates {
		ans := answers[i]
		if len(ans) == 0 {
			continue
		}
		if prob, ok := mathengine.GenerateByCode(tpl, d.seeds[i], locale); ok {
			if mathengine.Grade(prob, ans).IsCorrect {
				score++
			}
		}
	}

	outcome := "LOSS"
	var rewardSP int64 = DuelLossSP
	if score > d.record.OpponentScore {
		outcome = "WIN"
		rewardSP = DuelWinSP
	} else if score == d.record.OpponentScore {
		outcome = "DRAW"
		rewardSP = DuelDrawSP
	}

	now := s.nowClock()
	d.record.PlayerScore = score
	d.record.Outcome = outcome
	d.record.RewardSP = rewardSP
	d.record.Status = "COMPLETED"
	d.record.CompletedAt = &now
	out := d.record
	out.Questions = nil
	s.mu.Unlock()

	if s.ledger != nil && rewardSP > 0 {
		_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencySP,
			Delta:          rewardSP,
			SourceType:     "STAR_DUEL_" + outcome,
			SourceID:       duelID,
			IdempotencyKey: fmt.Sprintf("duel:%s:reward:%s", duelID, studentID),
		})
	}
	return out, nil
}

func (s *Service) generateUniqueCrewCodeLocked() string {
	for attempt := 0; attempt < 100; attempt++ {
		n, err := rand.Int(rand.Reader, big.NewInt(900000000))
		if err != nil {
			s.seq++
			code := fmt.Sprintf("%09d", 100000000+s.seq)
			if _, exists := s.crewsByCode[code]; !exists {
				return code
			}
			continue
		}
		code := fmt.Sprintf("%09d", 100000000+n.Int64())
		if _, exists := s.crewsByCode[code]; !exists {
			return code
		}
	}
	s.seq++
	return fmt.Sprintf("%09d", 999000000+s.seq)
}

func formatCrewCode(code string) string {
	if len(code) != 9 {
		return code
	}
	return code[0:3] + "-" + code[3:6] + "-" + code[6:9]
}

func maskDisplayName(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "Pilot P."
	}
	if len(parts) == 1 {
		return parts[0] + " P."
	}
	last := parts[len(parts)-1]
	return parts[0] + " " + strings.ToUpper(last[:1]) + "."
}

func buildCrewView(c *crewInternal) StarCrewView {
	var total int64
	members := make([]CrewMember, len(c.members))
	copy(members, c.members)
	for _, m := range members {
		total += m.WeeklySP
	}
	return StarCrewView{
		CrewCode:      c.code,
		FormattedCode: formatCrewCode(c.code),
		Name:          c.name,
		WeeklyTotalSP: total,
		Members:       members,
	}
}

func (s *Service) GetOrCreateCrew(studentID, displayName string, grade int) StarCrewView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if code, ok := s.studentCrew[studentID]; ok {
		if crew, exists := s.crewsByCode[code]; exists {
			return buildCrewView(crew)
		}
	}
	if grade < 1 || grade > 6 {
		grade = 1
	}
	code := s.generateUniqueCrewCodeLocked()
	crew := &crewInternal{
		code: code,
		name: "Star Crew #" + code[0:3],
		members: []CrewMember{
			{
				StudentID:  studentID,
				MaskedName: maskDisplayName(displayName),
				Grade:      grade,
				BuddyColor: "#6366F1",
				WeeklySP:   35,
				IsLeader:   true,
			},
		},
	}
	s.crewsByCode[code] = crew
	s.studentCrew[studentID] = code
	return buildCrewView(crew)
}

func (s *Service) JoinCrew(studentID, displayName string, grade int, rawCode string) (StarCrewView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clean := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(rawCode), "-", ""), " ", "")
	if len(clean) != 9 {
		return StarCrewView{}, ErrInvalidCode
	}
	for _, r := range clean {
		if r < '0' || r > '9' {
			return StarCrewView{}, ErrInvalidCode
		}
	}
	target, ok := s.crewsByCode[clean]
	if !ok {
		return StarCrewView{}, ErrCrewNotFound
	}
	for _, m := range target.members {
		if m.StudentID == studentID {
			return buildCrewView(target), ErrAlreadyInCrew
		}
	}
	if oldCode, ok := s.studentCrew[studentID]; ok {
		if oldCrew, exists := s.crewsByCode[oldCode]; exists {
			filtered := make([]CrewMember, 0, len(oldCrew.members))
			for _, m := range oldCrew.members {
				if m.StudentID != studentID {
					filtered = append(filtered, m)
				}
			}
			oldCrew.members = filtered
		}
	}
	if grade < 1 || grade > 6 {
		grade = 1
	}
	target.members = append(target.members, CrewMember{
		StudentID:  studentID,
		MaskedName: maskDisplayName(displayName),
		Grade:      grade,
		BuddyColor: "#22D3EE",
		WeeklySP:   35,
		IsLeader:   false,
	})
	s.studentCrew[studentID] = clean
	return buildCrewView(target), nil
}

func (s *Service) LeaveCrew(studentID, displayName string, grade int) StarCrewView {
	s.mu.Lock()
	if oldCode, ok := s.studentCrew[studentID]; ok {
		if oldCrew, exists := s.crewsByCode[oldCode]; exists {
			filtered := make([]CrewMember, 0, len(oldCrew.members))
			for _, m := range oldCrew.members {
				if m.StudentID != studentID {
					filtered = append(filtered, m)
				}
			}
			oldCrew.members = filtered
		}
		delete(s.studentCrew, studentID)
	}
	s.mu.Unlock()
	return s.GetOrCreateCrew(studentID, displayName, grade)
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
		OperationID: "duels-lobby",
		Method:      http.MethodGet,
		Path:        "/api/v1/duels/lobby",
		Summary:     "Get Star Duels lobby with daily energy cells, masked opponents, incoming & history",
		Tags:        []string{"Social"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body DuelLobby }, error) {
		ident, locale, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		return &struct{ Body DuelLobby }{Body: h.svc.GetLobby(ident.ID, ident.Grade, ident.Timezone, locale)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "duels-start",
		Method:      http.MethodPost,
		Path:        "/api/v1/duels/start",
		Summary:     "Launch a 1v1 Star Duel (consumes 1 of 3 daily energy cells)",
		Tags:        []string{"Social"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			Mode       string `json:"mode" default:"RANDOM"`
			OpponentID string `json:"opponentId,omitempty"`
		}
	}) (*struct {
		Body struct {
			Duel       DuelRecord `json:"duel"`
			EnergyLeft int        `json:"energyLeft"`
		}
	}, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		duel, left, err := h.svc.StartDuel(ident.ID, ident.Grade, ident.Timezone, locale, in.Body.Mode, in.Body.OpponentID)
		if err != nil {
			if errors.Is(err, ErrNoEnergy) {
				return nil, huma.Error429TooManyRequests(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Duel       DuelRecord `json:"duel"`
				EnergyLeft int        `json:"energyLeft"`
			}
		}{}
		out.Body.Duel = duel
		out.Body.EnergyLeft = left
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "duels-accept",
		Method:      http.MethodPost,
		Path:        "/api/v1/duels/{id}/accept",
		Summary:     "Accept an incoming Star Duel challenge",
		Tags:        []string{"Social"},
	}, func(ctx context.Context, in *struct {
		authIn
		ID string `path:"id"`
	}) (*struct{ Body DuelRecord }, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		duel, err := h.svc.AcceptIncoming(ident.ID, in.ID, locale)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body DuelRecord }{Body: duel}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "duels-submit",
		Method:      http.MethodPost,
		Path:        "/api/v1/duels/{id}/submit",
		Summary:     "Grade Star Duel answers and award Star Points",
		Tags:        []string{"Social"},
	}, func(ctx context.Context, in *struct {
		authIn
		ID   string `path:"id"`
		Body struct {
			Answers map[int]map[string]string `json:"answers"`
		}
	}) (*struct{ Body DuelRecord }, error) {
		ident, locale, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		duel, err := h.svc.SubmitDuel(ctx, ident.ID, in.ID, locale, in.Body.Answers)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body DuelRecord }{Body: duel}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "crew-get",
		Method:      http.MethodGet,
		Path:        "/api/v1/crew",
		Summary:     "Get or initialize the student's Star Crew (BFF) with 9-digit invite code",
		Tags:        []string{"Social"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body StarCrewView }, error) {
		ident, _, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		return &struct{ Body StarCrewView }{Body: h.svc.GetOrCreateCrew(ident.ID, ident.DisplayName, ident.Grade)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "crew-join",
		Method:      http.MethodPost,
		Path:        "/api/v1/crew/join",
		Summary:     "Join another pilot's Star Crew using a 9-digit invite code",
		Tags:        []string{"Social"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			CrewCode string `json:"crewCode"`
		}
	}) (*struct{ Body StarCrewView }, error) {
		ident, _, err := h.resolve(ctx, in.authIn)
		if err != nil {
			return nil, err
		}
		view, err := h.svc.JoinCrew(ident.ID, ident.DisplayName, ident.Grade, in.Body.CrewCode)
		if err != nil {
			if errors.Is(err, ErrCrewNotFound) {
				return nil, huma.Error404NotFound(err.Error())
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body StarCrewView }{Body: view}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "crew-leave",
		Method:      http.MethodPost,
		Path:        "/api/v1/crew/leave",
		Summary:     "Leave current Star Crew and create a fresh personal crew",
		Tags:        []string{"Social"},
	}, func(ctx context.Context, in *authIn) (*struct{ Body StarCrewView }, error) {
		ident, _, err := h.resolve(ctx, *in)
		if err != nil {
			return nil, err
		}
		return &struct{ Body StarCrewView }{Body: h.svc.LeaveCrew(ident.ID, ident.DisplayName, ident.Grade)}, nil
	})
}
