package social

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
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
	ErrDuelNotActive = errors.New("star duel is not in progress; accept it first")
	ErrCrewNotFound  = errors.New("star crew code not found")
	ErrAlreadyInCrew = errors.New("already a member of this star crew")
	ErrInvalidCode   = errors.New("star crew code must be 9 digits")
	// ErrRewardPending signals the Star Points ledger could not be reached, so
	// the completion was not recorded; the client can safely retry.
	ErrRewardPending = errors.New("star duel reward could not be recorded; try again")
)

// duelEnergyDoc is the durable per-student, per-local-date energy counter. It
// lives in the shared phase2_state store, so the 3-cells/day quota is enforced
// across restarts and API instances instead of per process.
type duelEnergyDoc struct {
	Used int `json:"used"`
}

func duelEnergyModule(localDate string) string { return "duel_energy_" + localDate }

// persistedDuel keeps everything needed to keep a duel playable and gradeable
// after a restart: the record, its question templates, and the seeds that
// regenerate the questions for grading.
type persistedDuel struct {
	ID        string     `json:"id"`
	Record    DuelRecord `json:"record"`
	Templates []string   `json:"templates"`
	Seeds     []int64    `json:"seeds"`
}

// duelsStateDoc is the durable per-student Star Duel document (newest duel
// first, capped at duelsStateCap entries). Stored in the shared phase2_state
// store so duel records and history survive restarts.
type duelsStateDoc struct {
	Duels []persistedDuel `json:"duels"`
}

const (
	duelsStateModule = "star_duels_v1"
	duelsStateCap    = 50
)

// newDuelID returns a random duel ID so the reward ledger idempotency key
// (duel:<id>:reward:<student>) stays unique across process restarts instead of
// colliding with an older duel's key (same bug class as CR-10).
func newDuelID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("duel-%d", time.Now().UnixNano())
	}
	return "duel-" + hex.EncodeToString(b[:])
}

// crewStateDoc is the durable Star Crew roster, shared by every member. It
// lives in the shared_state store (scope "crew") keyed by the 9-digit code,
// because a crew spans several students.
type crewStateDoc struct {
	Code    string       `json:"code"`
	Name    string       `json:"name"`
	Members []CrewMember `json:"members"`
}

// crewMembershipDoc is a student's durable pointer to their crew.
type crewMembershipDoc struct {
	Code string `json:"code"`
}

const (
	crewScope       = "crew"
	crewKeyPrefix   = "code:"
	memberKeyPrefix = "member:"
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
	mu             sync.Mutex
	ledger         economy.Ledger
	pool           *pgxpool.Pool
	store          *state.PGStore // nil in in-memory (test) mode
	energyUsed     map[string]int // studentID:YYYY-MM-DD -> used (memory mode only)
	duels          map[string]*duelInternal
	studentDuels   map[string][]string
	duelsLoaded    map[string]bool // studentID -> duel state hydrated from PG
	duelHydrateErr map[string]bool // studentID -> durable load failed; skip persisting
	crewsByCode    map[string]*crewInternal
	studentCrew    map[string]string // studentID -> crewCode
	crewLoaded     map[string]bool   // studentID -> crew state hydrated from PG
	peers          []OpponentProfile
	seq            int
	nowClock       func() time.Time
}

// NewService creates an in-memory service for unit tests.
func NewService(ledger economy.Ledger) *Service {
	s := &Service{
		ledger:         ledger,
		energyUsed:     make(map[string]int),
		duels:          make(map[string]*duelInternal),
		studentDuels:   make(map[string][]string),
		duelsLoaded:    make(map[string]bool),
		duelHydrateErr: make(map[string]bool),
		crewsByCode:    make(map[string]*crewInternal),
		studentCrew:    make(map[string]string),
		crewLoaded:     make(map[string]bool),
		nowClock:       time.Now,
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

// NewServiceWithPG creates a production service whose Star Duel energy quota,
// duel records, and Star Crew state are enforced/stored from durable
// PostgreSQL state, shared across restarts and instances.
func NewServiceWithPG(pool *pgxpool.Pool, ledger economy.Ledger) *Service {
	s := NewService(ledger)
	s.pool = pool
	s.store = state.NewPGStore(pool)
	return s
}

func (s *Service) localDateFor(tz string) string {
	return timewindow.LocalDateString(s.nowClock(), tz)
}

// energyUsedFor reads the student's durable (or in-memory) energy usage for the
// given local date.
func (s *Service) energyUsedFor(ctx context.Context, studentID, localDate string) int {
	if s.store != nil {
		var doc duelEnergyDoc
		if found, err := s.store.Load(ctx, studentID, duelEnergyModule(localDate), &doc); err == nil && found {
			return doc.Used
		}
		return 0
	}
	return s.energyUsed[studentID+":"+localDate]
}

// consumeEnergy atomically consumes one energy cell for the local date. In
// durable mode the row is locked and the quota check happens inside the
// transaction, so concurrent requests and other instances cannot exceed the
// daily maximum.
func (s *Service) consumeEnergy(ctx context.Context, studentID, localDate string) error {
	if s.store == nil {
		key := studentID + ":" + localDate
		if s.energyUsed[key] >= DailyEnergyMax {
			return ErrNoEnergy
		}
		s.energyUsed[key]++
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.store.SaveMergedTx(ctx, tx, studentID, duelEnergyModule(localDate), func(current []byte) (any, error) {
		var doc duelEnergyDoc
		_ = json.Unmarshal(current, &doc)
		if doc.Used >= DailyEnergyMax {
			return nil, ErrNoEnergy
		}
		doc.Used++
		return doc, nil
	}); err != nil {
		if errors.Is(err, ErrNoEnergy) {
			return ErrNoEnergy
		}
		return err
	}
	return tx.Commit(ctx)
}

// hydrateDuels loads the student's durable duel document into the in-memory
// working set on first touch (once per process, like supernova progress).
// Caller must hold s.mu.
func (s *Service) hydrateDuels(ctx context.Context, studentID string) {
	if s.store == nil || s.duelsLoaded[studentID] {
		return
	}
	s.duelsLoaded[studentID] = true
	var doc duelsStateDoc
	found, err := s.store.Load(ctx, studentID, duelsStateModule, &doc)
	if err != nil {
		// Keep operating in memory, but do not later overwrite the durable
		// snapshot with partial data.
		s.duelHydrateErr[studentID] = true
		return
	}
	if !found {
		return
	}
	ids := make([]string, 0, len(doc.Duels))
	for _, p := range doc.Duels {
		if _, exists := s.duels[p.ID]; exists {
			continue
		}
		s.duels[p.ID] = &duelInternal{
			record:    p.Record,
			studentID: studentID,
			templates: p.Templates,
			seeds:     p.Seeds,
		}
		ids = append(ids, p.ID)
	}
	// doc.Duels is newest-first, and s.studentDuels keeps that order.
	s.studentDuels[studentID] = append(ids, s.studentDuels[studentID]...)
}

// persistDuels writes the student's current duel working set to the durable
// per-student document. Best-effort: duel records are gameplay state, so a
// persistence failure never blocks play this process (unlike energy and
// ledger, which are strictly durable) — at worst the record is re-lost on
// restart. Caller must hold s.mu.
func (s *Service) persistDuels(ctx context.Context, studentID string) {
	if s.store == nil || s.duelHydrateErr[studentID] {
		return
	}
	doc := duelsStateDoc{Duels: make([]persistedDuel, 0, len(s.studentDuels[studentID]))}
	for _, id := range s.studentDuels[studentID] {
		if d := s.duels[id]; d != nil && d.studentID == studentID {
			doc.Duels = append(doc.Duels, persistedDuel{
				ID:        id,
				Record:    d.record,
				Templates: d.templates,
				Seeds:     d.seeds,
			})
		}
	}
	if len(doc.Duels) > duelsStateCap {
		doc.Duels = doc.Duels[:duelsStateCap]
	}
	_ = s.store.Save(ctx, studentID, duelsStateModule, doc)
}

// hydrateCrew loads the student's durable crew membership and crew roster
// into the in-memory working set on first touch. Caller must hold s.mu.
func (s *Service) hydrateCrew(ctx context.Context, studentID string) {
	if s.store == nil || s.crewLoaded[studentID] {
		return
	}
	s.crewLoaded[studentID] = true
	if code, ok := s.studentCrew[studentID]; ok {
		// Membership already known in memory; make sure the crew roster is
		// loaded too (it may have been created by another instance).
		s.loadCrewDoc(ctx, code)
		return
	}
	var m crewMembershipDoc
	found, err := s.store.LoadShared(ctx, crewScope, memberKeyPrefix+studentID, &m)
	if err != nil || !found || m.Code == "" {
		return
	}
	if s.loadCrewDoc(ctx, m.Code) {
		s.studentCrew[studentID] = m.Code
	}
}

// loadCrewDoc loads (or refreshes) the durable crew roster into the in-memory
// cache. The durable document is authoritative for the roster after any
// locked merge. Caller must hold s.mu.
func (s *Service) loadCrewDoc(ctx context.Context, code string) bool {
	var doc crewStateDoc
	found, err := s.store.LoadShared(ctx, crewScope, crewKeyPrefix+code, &doc)
	if err != nil || !found || doc.Code == "" {
		return false
	}
	s.crewsByCode[code] = &crewInternal{code: doc.Code, name: doc.Name, members: doc.Members}
	return true
}

// persistNewCrew writes a freshly created crew roster and the student's
// membership pointer durably. Caller must hold s.mu.
func (s *Service) persistNewCrew(ctx context.Context, studentID string, crew *crewInternal) {
	if s.store == nil {
		return
	}
	_ = s.store.SaveShared(ctx, crewScope, crewKeyPrefix+crew.code, crewStateDoc{
		Code:    crew.code,
		Name:    crew.name,
		Members: crew.members,
	})
	_ = s.store.SaveShared(ctx, crewScope, memberKeyPrefix+studentID, crewMembershipDoc{Code: crew.code})
}

// removeMemberDurable removes the student from the durable crew roster under a
// row lock. When the durable document does not exist yet (for example the
// in-memory demo seed crew), the in-memory snapshot seeds it. Caller must
// hold s.mu.
func (s *Service) removeMemberDurable(ctx context.Context, code, studentID string, base *crewInternal) {
	if s.store == nil {
		return
	}
	b := crewStateDoc{}
	if base != nil {
		b = crewStateDoc{Code: base.code, Name: base.name, Members: append([]CrewMember(nil), base.members...)}
	}
	if err := s.store.SaveSharedMerged(ctx, crewScope, crewKeyPrefix+code, func(current []byte) (any, error) {
		var doc crewStateDoc
		_ = json.Unmarshal(current, &doc)
		if doc.Code == "" {
			doc = b
		}
		filtered := make([]CrewMember, 0, len(doc.Members))
		for _, m := range doc.Members {
			if m.StudentID != studentID {
				filtered = append(filtered, m)
			}
		}
		doc.Members = filtered
		return doc, nil
	}); err == nil {
		s.loadCrewDoc(ctx, code)
	}
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

func (s *Service) ensureIncomingLocked(ctx context.Context, studentID string, grade int, locale string) {
	for _, id := range s.studentDuels[studentID] {
		if d := s.duels[id]; d != nil && d.record.Direction == "INCOMING" {
			return
		}
	}
	opp := s.pickOpponentLocked(grade, "CLASSMATE", "")
	s.seq++
	id := newDuelID()
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
	s.persistDuels(ctx, studentID)
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

func (s *Service) GetLobby(ctx context.Context, studentID string, grade int, tz, locale string) DuelLobby {
	s.mu.Lock()
	defer s.mu.Unlock()
	if grade < 1 || grade > 6 {
		grade = 1
	}
	s.hydrateDuels(ctx, studentID)
	s.ensureIncomingLocked(ctx, studentID, grade, locale)
	localDate := s.localDateFor(tz)
	used := s.energyUsedFor(ctx, studentID, localDate)
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

func (s *Service) StartDuel(ctx context.Context, studentID string, grade int, tz, locale, mode, opponentID string) (DuelRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if grade < 1 || grade > 6 {
		grade = 1
	}
	s.hydrateDuels(ctx, studentID)
	localDate := s.localDateFor(tz)
	if s.energyUsedFor(ctx, studentID, localDate) >= DailyEnergyMax {
		return DuelRecord{}, 0, ErrNoEnergy
	}

	mode = strings.ToUpper(strings.TrimSpace(mode))
	if mode != "CLASSMATE" && mode != "CREW" && mode != "RANDOM" {
		mode = "RANDOM"
	}
	opp := s.pickOpponentLocked(grade, mode, opponentID)
	if err := s.consumeEnergy(ctx, studentID, localDate); err != nil {
		return DuelRecord{}, 0, err
	}
	energyLeft := DailyEnergyMax - s.energyUsedFor(ctx, studentID, localDate)

	s.seq++
	id := newDuelID()
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
	s.persistDuels(ctx, studentID)
	return rec, energyLeft, nil
}

func (s *Service) AcceptIncoming(ctx context.Context, studentID, duelID, tz, locale string) (DuelRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hydrateDuels(ctx, studentID)
	d, ok := s.duels[duelID]
	if !ok || d.studentID != studentID {
		return DuelRecord{}, ErrDuelNotFound
	}
	if d.record.Status == "COMPLETED" {
		return DuelRecord{}, ErrDuelCompleted
	}
	if d.record.Status == "IN_PROGRESS" {
		return DuelRecord{}, ErrDuelNotActive
	}
	// Accepting an incoming challenge consumes an energy cell under the same
	// daily quota as launching a duel, enforced durably.
	if err := s.consumeEnergy(ctx, studentID, s.localDateFor(tz)); err != nil {
		return DuelRecord{}, err
	}
	d.record.Status = "IN_PROGRESS"
	s.persistDuels(ctx, studentID)
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
	s.hydrateDuels(ctx, studentID)
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
	// Only a duel this student accepted/launched (IN_PROGRESS) can be graded;
	// pending incoming duels can no longer be submitted after energy runs out.
	if d.record.Status != "IN_PROGRESS" {
		s.mu.Unlock()
		return DuelRecord{}, ErrDuelNotActive
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

	// Apply the ledger BEFORE recording completion: a failed award leaves the
	// duel IN_PROGRESS so the student can retry and still receive the reward
	// (the idempotency key prevents double-awarding on success).
	if s.ledger != nil && rewardSP > 0 {
		if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencySP,
			Delta:          rewardSP,
			SourceType:     "STAR_DUEL_" + outcome,
			SourceID:       duelID,
			IdempotencyKey: fmt.Sprintf("duel:%s:reward:%s", duelID, studentID),
		}); err != nil {
			s.mu.Unlock()
			return DuelRecord{}, fmt.Errorf("%w: %v", ErrRewardPending, err)
		}
	}

	now := s.nowClock()
	d.record.PlayerScore = score
	d.record.Outcome = outcome
	d.record.RewardSP = rewardSP
	d.record.Status = "COMPLETED"
	d.record.CompletedAt = &now
	s.persistDuels(ctx, studentID)
	out := d.record
	out.Questions = nil
	s.mu.Unlock()
	return out, nil
}

// crewCodeAvailable reports whether a 9-digit code is unused in memory and in
// the durable crew roster (so restarts cannot hand out a code again).
// Caller must hold s.mu.
func (s *Service) crewCodeAvailable(ctx context.Context, code string) bool {
	if _, exists := s.crewsByCode[code]; exists {
		return false
	}
	if s.store != nil {
		if found, _ := s.store.LoadShared(ctx, crewScope, crewKeyPrefix+code, &crewStateDoc{}); found {
			return false
		}
	}
	return true
}

func (s *Service) generateUniqueCrewCode(ctx context.Context) string {
	for attempt := 0; attempt < 100; attempt++ {
		n, err := rand.Int(rand.Reader, big.NewInt(900000000))
		if err != nil {
			s.seq++
			code := fmt.Sprintf("%09d", 100000000+s.seq)
			if s.crewCodeAvailable(ctx, code) {
				return code
			}
			continue
		}
		code := fmt.Sprintf("%09d", 100000000+n.Int64())
		if s.crewCodeAvailable(ctx, code) {
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

// GetOrCreateCrew returns the student's Star Crew, creating (and durably
// persisting) a personal crew when they have none.
func (s *Service) GetOrCreateCrew(ctx context.Context, studentID, displayName string, grade int) StarCrewView {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hydrateCrew(ctx, studentID)
	if code, ok := s.studentCrew[studentID]; ok {
		if crew, exists := s.crewsByCode[code]; exists {
			return buildCrewView(crew)
		}
	}
	if grade < 1 || grade > 6 {
		grade = 1
	}
	code := s.generateUniqueCrewCode(ctx)
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
	s.persistNewCrew(ctx, studentID, crew)
	return buildCrewView(crew)
}

func (s *Service) JoinCrew(ctx context.Context, studentID, displayName string, grade int, rawCode string) (StarCrewView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hydrateCrew(ctx, studentID)
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
	if !ok && s.store != nil {
		// The roster may live only in the durable store (created by another
		// instance or before this process started).
		s.loadCrewDoc(ctx, clean)
		target, ok = s.crewsByCode[clean]
	}
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
			s.removeMemberDurable(ctx, oldCode, studentID, oldCrew)
		}
	}
	if grade < 1 || grade > 6 {
		grade = 1
	}
	member := CrewMember{
		StudentID:  studentID,
		MaskedName: maskDisplayName(displayName),
		Grade:      grade,
		BuddyColor: "#22D3EE",
		WeeklySP:   35,
		IsLeader:   false,
	}
	target.members = append(target.members, member)
	if s.store != nil {
		// Lock the durable roster and add the member there, so concurrent
		// joins from other instances cannot drop each other's member. If the
		// durable document does not exist yet (demo seed crew), the in-memory
		// snapshot seeds it.
		if err := s.store.SaveSharedMerged(ctx, crewScope, crewKeyPrefix+clean, func(current []byte) (any, error) {
			var doc crewStateDoc
			_ = json.Unmarshal(current, &doc)
			if doc.Code == "" {
				doc = crewStateDoc{Code: target.code, Name: target.name, Members: append([]CrewMember(nil), target.members...)}
			}
			present := false
			for _, m := range doc.Members {
				if m.StudentID == studentID {
					present = true
					break
				}
			}
			if !present {
				doc.Members = append(doc.Members, member)
			}
			return doc, nil
		}); err == nil {
			s.loadCrewDoc(ctx, clean)
			target = s.crewsByCode[clean]
		}
		_ = s.store.SaveShared(ctx, crewScope, memberKeyPrefix+studentID, crewMembershipDoc{Code: clean})
	}
	s.studentCrew[studentID] = clean
	return buildCrewView(target), nil
}

func (s *Service) LeaveCrew(ctx context.Context, studentID, displayName string, grade int) StarCrewView {
	s.mu.Lock()
	s.hydrateCrew(ctx, studentID)
	if oldCode, ok := s.studentCrew[studentID]; ok {
		if oldCrew, exists := s.crewsByCode[oldCode]; exists {
			filtered := make([]CrewMember, 0, len(oldCrew.members))
			for _, m := range oldCrew.members {
				if m.StudentID != studentID {
					filtered = append(filtered, m)
				}
			}
			oldCrew.members = filtered
			s.removeMemberDurable(ctx, oldCode, studentID, oldCrew)
		}
		delete(s.studentCrew, studentID)
		if s.store != nil {
			_ = s.store.DeleteShared(ctx, crewScope, memberKeyPrefix+studentID)
		}
	}
	s.mu.Unlock()
	return s.GetOrCreateCrew(ctx, studentID, displayName, grade)
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
		return &struct{ Body DuelLobby }{Body: h.svc.GetLobby(ctx, ident.ID, ident.Grade, ident.Timezone, locale)}, nil
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
		duel, left, err := h.svc.StartDuel(ctx, ident.ID, ident.Grade, ident.Timezone, locale, in.Body.Mode, in.Body.OpponentID)
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
		duel, err := h.svc.AcceptIncoming(ctx, ident.ID, in.ID, ident.Timezone, locale)
		if err != nil {
			if errors.Is(err, ErrDuelNotActive) {
				return nil, huma.Error409Conflict(err.Error())
			}
			if errors.Is(err, ErrNoEnergy) {
				return nil, huma.Error429TooManyRequests(err.Error())
			}
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
			if errors.Is(err, ErrDuelNotActive) {
				return nil, huma.Error409Conflict(err.Error())
			}
			if errors.Is(err, ErrRewardPending) {
				return nil, huma.Error503ServiceUnavailable(err.Error())
			}
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
		return &struct{ Body StarCrewView }{Body: h.svc.GetOrCreateCrew(ctx, ident.ID, ident.DisplayName, ident.Grade)}, nil
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
		view, err := h.svc.JoinCrew(ctx, ident.ID, ident.DisplayName, ident.Grade, in.Body.CrewCode)
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
		return &struct{ Body StarCrewView }{Body: h.svc.LeaveCrew(ctx, ident.ID, ident.DisplayName, ident.Grade)}, nil
	})
}
