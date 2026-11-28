package streak

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/timewindow"
)

const (
	DailyStreakMilestone = 5
	MaxCryoShields       = 3
)

type DayStatus string

const (
	DayCompleted DayStatus = "COMPLETED"
	DayFrozen    DayStatus = "FROZEN"
	DayMissed    DayStatus = "MISSED"
)

type Status struct {
	CurrentStreak        int    `json:"currentStreak"`
	LongestStreak        int    `json:"longestStreak"`
	CryoShields          int    `json:"cryoShields"`
	MaxCryoShields       int    `json:"maxCryoShields"`
	NovaPetStage         int    `json:"novaPetStage"`
	TodayCorrectCount    int    `json:"todayCorrectCount"`
	DailyMilestoneTarget int    `json:"dailyMilestoneTarget"`
	LastCompletedDate    string `json:"lastCompletedDate,omitempty"`
	ReminderEnabled      bool   `json:"reminderEnabled"`
	ReminderTimeLocal    string `json:"reminderTimeLocal,omitempty"`
}

type CalendarEntry struct {
	Date   string    `json:"date"`
	Status DayStatus `json:"status"`
}

type record struct {
	status    Status
	calendar  map[string]DayStatus
	todayDate string
}

type Service struct {
	mu        sync.Mutex
	records   map[string]*record
	pool      *pgxpool.Pool
	subjectID string
	nowClock  func() time.Time
}

func NewService() *Service {
	return &Service{
		records:  make(map[string]*record),
		nowClock: time.Now,
	}
}

func NewServiceWithPG(pool *pgxpool.Pool) *Service {
	return &Service{
		records:  make(map[string]*record),
		pool:     pool,
		nowClock: time.Now,
	}
}

func (s *Service) getOrCreateLocked(studentID string) (*record, error) {
	return s.getOrCreate(context.Background(), s.pool, studentID)
}

func (s *Service) getOrCreate(ctx context.Context, db pgExecutor, studentID string) (*record, error) {
	if s.pool != nil && isUUID(studentID) {
		return s.loadRecordPG(ctx, db, studentID)
	}
	r, ok := s.records[studentID]
	if !ok {
		r = &record{
			status: Status{
				CurrentStreak:        3,
				LongestStreak:        3,
				CryoShields:          2,
				MaxCryoShields:       MaxCryoShields,
				NovaPetStage:         NovaPetStageForStreak(3),
				DailyMilestoneTarget: DailyStreakMilestone,
				ReminderTimeLocal:    "19:00",
			},
			calendar: map[string]DayStatus{},
		}
		s.records[studentID] = r
	}
	return r, nil
}

func NovaPetStageForStreak(streak int) int {
	switch {
	case streak >= 30:
		return 5
	case streak >= 14:
		return 4
	case streak >= 7:
		return 3
	case streak >= 3:
		return 2
	default:
		return 1
	}
}

func (s *Service) GetStatus(studentID, tz string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getOrCreateLocked(studentID)
	if err != nil {
		return Status{}, err
	}
	out := r.status
	out.MaxCryoShields = MaxCryoShields
	out.DailyMilestoneTarget = DailyStreakMilestone
	_ = tz
	return out, nil
}

func (s *Service) GetCalendar(studentID, monthPrefix string) ([]CalendarEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getOrCreateLocked(studentID)
	if err != nil {
		return nil, err
	}
	entries := make([]CalendarEntry, 0, len(r.calendar))
	for dt, st := range r.calendar {
		if monthPrefix == "" || (len(dt) >= len(monthPrefix) && dt[:len(monthPrefix)] == monthPrefix) {
			entries = append(entries, CalendarEntry{Date: dt, Status: st})
		}
	}
	return entries, nil
}

// RecordDailyOrbitProgress updates today's correct count and advances the Orbit Streak
// once the student reaches 5 correct answers on localDate.
func (s *Service) RecordDailyOrbitProgress(studentID, tz string, now time.Time, correctCount int) (Status, error) {
	return s.recordDailyOrbitProgress(context.Background(), nil, studentID, tz, now, correctCount)
}

// RecordDailyOrbitProgressTx writes the streak inside the caller's transaction.
func (s *Service) RecordDailyOrbitProgressTx(ctx context.Context, tx pgx.Tx, studentID, tz string, now time.Time, correctCount int) (Status, error) {
	return s.recordDailyOrbitProgress(ctx, tx, studentID, tz, now, correctCount)
}

func (s *Service) recordDailyOrbitProgress(ctx context.Context, tx pgx.Tx, studentID, tz string, now time.Time, correctCount int) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var db pgExecutor
	if tx != nil {
		db = tx
	} else if s.pool != nil {
		db = s.pool
	}
	r, err := s.getOrCreate(ctx, db, studentID)
	if err != nil {
		return Status{}, err
	}
	localDate := timewindow.LocalDateString(now, tz)
	r.todayDate = localDate
	r.status.TodayCorrectCount = correctCount

	changedDate := ""
	if correctCount >= DailyStreakMilestone && r.status.LastCompletedDate != localDate {
		r.status.CurrentStreak++
		if r.status.CurrentStreak > r.status.LongestStreak {
			r.status.LongestStreak = r.status.CurrentStreak
		}
		r.status.NovaPetStage = NovaPetStageForStreak(r.status.CurrentStreak)
		r.status.LastCompletedDate = localDate
		r.calendar[localDate] = DayCompleted
		changedDate = localDate
	}
	if tx != nil {
		return r.status, s.writeRecordPG(ctx, tx, studentID, r, changedDate)
	}
	return r.status, s.saveRecordPG(ctx, studentID, r, changedDate)
}

// ReconcileMidnight evaluates yesterday (D-1) in the student's timezone:
// consumes 1 Cryo Shield if missed and shields > 0; otherwise resets streak to 0.
func (s *Service) ReconcileMidnight(studentID, tz string, now time.Time) Status {
	st, _ := s.reconcileMidnightErr(context.Background(), studentID, tz, now)
	return st
}

func (s *Service) reconcileMidnightErr(ctx context.Context, studentID, tz string, now time.Time) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getOrCreate(ctx, s.pool, studentID)
	if err != nil {
		return Status{}, err
	}
	loc := timewindow.LoadLocationSafe(tz)
	localToday := timewindow.LocalDateString(now, tz)
	yesterday := now.In(loc).AddDate(0, 0, -1).Format("2006-01-02")

	resetToday := r.todayDate != localToday
	if resetToday {
		r.status.TodayCorrectCount = 0
		r.todayDate = localToday
	}
	if r.calendar[yesterday] == DayCompleted || r.calendar[yesterday] == DayFrozen || r.calendar[yesterday] == DayMissed {
		if resetToday && s.pool != nil && isUUID(studentID) {
			if err := s.resetTodayPG(ctx, s.pool, studentID, localToday); err != nil {
				return Status{}, err
			}
		}
		return r.status, nil
	}
	if r.status.CurrentStreak > 0 {
		if r.status.CryoShields > 0 {
			r.status.CryoShields--
			r.calendar[yesterday] = DayFrozen
		} else {
			r.status.CurrentStreak = 0
			r.status.NovaPetStage = NovaPetStageForStreak(0)
			r.calendar[yesterday] = DayMissed
		}
	}
	if err := s.saveRecordPG(ctx, studentID, r, yesterday); err != nil {
		return Status{}, err
	}
	return r.status, nil
}

// GrantCryoShield adds shields up to the strict cap of 3.
func (s *Service) GrantCryoShield(studentID string, count int) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getOrCreateLocked(studentID)
	if err != nil {
		return Status{}, err
	}
	r.status.CryoShields += count
	if r.status.CryoShields > MaxCryoShields {
		r.status.CryoShields = MaxCryoShields
	}
	if r.status.CryoShields < 0 {
		r.status.CryoShields = 0
	}
	return r.status, s.saveRecordPG(context.Background(), studentID, r, "")
}

func (s *Service) SetReminder(studentID string, enabled bool, timeLocal string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getOrCreateLocked(studentID)
	if err != nil {
		return Status{}, err
	}
	r.status.ReminderEnabled = enabled
	if timeLocal != "" {
		r.status.ReminderTimeLocal = timeLocal
	}
	return r.status, s.saveRecordPG(context.Background(), studentID, r, "")
}

type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

type AuthHeaders struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
}

func (h *Handler) resolveStudent(ctx context.Context, in AuthHeaders, requireAuth bool) (string, string, error) {
	tok := in.CookieAccess
	if tok == "" && len(in.Authorization) > 7 {
		tok = in.Authorization[7:]
	}
	if h.authSvc != nil {
		if tok != "" {
			p, err := h.authSvc.ResolvePrincipal(ctx, tok)
			if err != nil {
				return "", "", auth.ErrInvalidCredentials
			}
			return p.ID, p.Timezone, nil
		}
		if requireAuth {
			return "", "", auth.ErrInvalidCredentials
		}
	}
	return "demo-pilot1", "Asia/Jakarta", nil
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "streak-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/streak/status",
		Summary:     "Get student's Orbit Streak, Nova Pet stage, and Cryo Shields",
		Tags:        []string{"Streak"},
	}, func(ctx context.Context, in *AuthHeaders) (*struct{ Body Status }, error) {
		studentID, tz, err := h.resolveStudent(ctx, *in, false)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		status, err := h.svc.GetStatus(studentID, tz)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("streak unavailable")
		}
		return &struct{ Body Status }{Body: status}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "streak-calendar",
		Method:      http.MethodGet,
		Path:        "/api/v1/streak/calendar",
		Summary:     "Get monthly Orbit Streak calendar days",
		Tags:        []string{"Streak"},
	}, func(ctx context.Context, in *struct {
		AuthHeaders
		Month string `query:"month" doc:"YYYY-MM filter"`
	}) (*struct {
		Body struct {
			Days []CalendarEntry `json:"days"`
		}
	}, error) {
		studentID, _, err := h.resolveStudent(ctx, in.AuthHeaders, false)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		out := &struct {
			Body struct {
				Days []CalendarEntry `json:"days"`
			}
		}{}
		out.Body.Days, err = h.svc.GetCalendar(studentID, in.Month)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("streak calendar unavailable")
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "streak-reminder",
		Method:      http.MethodPost,
		Path:        "/api/v1/streak/reminder",
		Summary:     "Update Orbit Streak daily push reminder preference",
		Tags:        []string{"Streak"},
		Security:    []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}},
	}, func(ctx context.Context, in *struct {
		AuthHeaders
		Body struct {
			Enabled   bool   `json:"enabled"`
			TimeLocal string `json:"timeLocal,omitempty"`
		}
	}) (*struct{ Body Status }, error) {
		studentID, _, err := h.resolveStudent(ctx, in.AuthHeaders, true)
		if err != nil {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		status, err := h.svc.SetReminder(studentID, in.Body.Enabled, in.Body.TimeLocal)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("streak reminder unavailable")
		}
		return &struct{ Body Status }{Body: status}, nil
	})
}
