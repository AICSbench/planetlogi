package academy

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/reqguard"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
)

var (
	ErrLessonNotFound = errors.New("lesson not found")
	ErrInvalidInput   = errors.New("invalid input")
	// ErrRewardUnavailable means the Star Scroll credit failed; it can be retried.
	ErrRewardUnavailable = errors.New("star scroll could not be credited; please retry")
)

type SelfCheckItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Checked bool   `json:"checked"`
}

type LessonSummary struct {
	Code          string `json:"code"`
	Title         string `json:"title"`
	AgeBand       string `json:"ageBand"`
	CompletionPct int    `json:"completionPct"`
	ScrollEarned  bool   `json:"scrollEarned"`
	SkillCode     string `json:"skillCode"`
}

type TopicSummary struct {
	Code          string          `json:"code"`
	Grade         int             `json:"grade"`
	Title         string          `json:"title"`
	AgeBand       string          `json:"ageBand"`
	ScrollsEarned int             `json:"scrollsEarned"`
	TotalScrolls  int             `json:"totalScrolls"`
	Lessons       []LessonSummary `json:"lessons"`
}

type DoLearnQuestion struct {
	Index          int                         `json:"index"`
	Question       mathengine.RenderedQuestion `json:"question"`
	DraftAnswer    map[string]string           `json:"draftAnswer,omitempty"`
	Solved         bool                        `json:"solved"`
	LastWrong      bool                        `json:"lastWrong"`
	WorkedSolution *mathengine.WorkedSolution  `json:"workedSolution,omitempty"`
}

type LessonDetail struct {
	Code               string `json:"code"`
	TopicCode          string `json:"topicCode"`
	Title              string `json:"title"`
	AgeBand            string `json:"ageBand"`
	SkillCode          string `json:"skillCode"`
	StoryVideoTitle    string `json:"storyVideoTitle"`
	ConceptVideoTitle  string `json:"conceptVideoTitle"`
	StoryVideoURL      string `json:"storyVideoUrl,omitempty"`
	ConceptVideoURL    string `json:"conceptVideoUrl,omitempty"`
	StoryCaptionsURL   string `json:"storyCaptionsUrl,omitempty"`
	ConceptCaptionsURL string `json:"conceptCaptionsUrl,omitempty"`
	// YouTube video IDs, played through the privacy-enhanced youtube-nocookie.com embed.
	// When set they take precedence over the self-hosted URLs.
	StoryYouTubeID   string            `json:"storyYoutubeId,omitempty"`
	ConceptYouTubeID string            `json:"conceptYoutubeId,omitempty"`
	CompletionPct    int               `json:"completionPct"`
	ScrollEarned     bool              `json:"scrollEarned"`
	DoAndLearn       []DoLearnQuestion `json:"doAndLearn"`
	SelfCheck        []SelfCheckItem   `json:"selfCheck"`
}

//go:embed media.json
var lessonMediaJSON []byte

type lessonMedia struct {
	StoryVideoURLs     map[string]string `json:"storyVideoUrls"`
	ConceptVideoURLs   map[string]string `json:"conceptVideoUrls"`
	StoryCaptionURLs   map[string]string `json:"storyCaptionUrls"`
	ConceptCaptionURLs map[string]string `json:"conceptCaptionUrls"`
	// Per-locale YouTube video IDs (11 characters), e.g. {"en": "...", "id": "..."}.
	StoryYouTubeIDs   map[string]string `json:"storyYoutubeIds"`
	ConceptYouTubeIDs map[string]string `json:"conceptYoutubeIds"`
}

type lessonMediaFile struct {
	Lessons map[string]lessonMedia `json:"lessons"`
}

func localeTextFromMap(values map[string]string) mathengine.LocaleText {
	return mathengine.LocaleText{EN: values["en"], ID: values["id"]}
}

// applyLessonMedia connects the compiled catalog to the runtime media source.
// Content can add per-lesson URLs without changing the Go catalog.
func applyLessonMedia(lessons []lessonDef) []lessonDef {
	var file lessonMediaFile
	if err := json.Unmarshal(lessonMediaJSON, &file); err != nil {
		return lessons
	}
	for i := range lessons {
		media, ok := file.Lessons[lessons[i].code]
		if !ok {
			continue
		}
		lessons[i].storyVideoURL = localeTextFromMap(media.StoryVideoURLs)
		lessons[i].conceptVideoURL = localeTextFromMap(media.ConceptVideoURLs)
		lessons[i].storyCaptionsURL = localeTextFromMap(media.StoryCaptionURLs)
		lessons[i].conceptCaptionsURL = localeTextFromMap(media.ConceptCaptionURLs)
		lessons[i].storyYouTubeID = localeTextFromMap(media.StoryYouTubeIDs)
		lessons[i].conceptYouTubeID = localeTextFromMap(media.ConceptYouTubeIDs)
	}
	return lessons
}

type lessonDef struct {
	code         string
	topicCode    string
	grade        int
	ageBand      string
	templateCode string
	skillCode    string
	title        mathengine.LocaleText
	storyVideo   mathengine.LocaleText
	conceptVideo mathengine.LocaleText
	// Media URLs are optional per locale; lessons without media keep the
	// illustrated placeholder while the content team supplies assets.
	storyVideoURL      mathengine.LocaleText
	conceptVideoURL    mathengine.LocaleText
	storyCaptionsURL   mathengine.LocaleText
	conceptCaptionsURL mathengine.LocaleText
	storyYouTubeID     mathengine.LocaleText
	conceptYouTubeID   mathengine.LocaleText
	selfCheck          []struct {
		key  string
		text mathengine.LocaleText
	}
}

type topicDef struct {
	code    string
	grade   int
	ageBand string
	title   mathengine.LocaleText
}

type lessonProgress struct {
	drafts             map[int]map[string]string
	draftUpdatedAt     map[int]int64
	solved             map[int]bool
	lastWrong          map[int]bool
	selfChecked        map[string]bool
	selfCheckUpdatedAt map[string]int64
	scrollEarned       bool
}

type Service struct {
	mu       sync.Mutex
	ledger   economy.Ledger
	store    state.Store
	loaded   map[string]bool
	topics   []topicDef
	lessons  []lessonDef
	progress map[string]map[string]*lessonProgress // studentID -> lessonCode -> progress
}

func NewService(ledger economy.Ledger) *Service {
	t, l := buildCatalog()
	return &Service{
		ledger:   ledger,
		loaded:   make(map[string]bool),
		topics:   t,
		lessons:  l,
		progress: make(map[string]map[string]*lessonProgress),
	}
}

func NewServiceWithStore(store state.Store, ledger economy.Ledger) *Service {
	svc := NewService(ledger)
	svc.store = store
	return svc
}

type academyState struct {
	Lessons map[string]lessonProgressDTO `json:"lessons"`
}

type lessonProgressDTO struct {
	Drafts             map[int]map[string]string `json:"drafts"`
	DraftUpdatedAt     map[int]int64             `json:"draftUpdatedAt,omitempty"`
	Solved             map[int]bool              `json:"solved"`
	LastWrong          map[int]bool              `json:"lastWrong"`
	SelfChecked        map[string]bool           `json:"selfChecked"`
	SelfCheckUpdatedAt map[string]int64          `json:"selfCheckUpdatedAt,omitempty"`
	ScrollEarned       bool                      `json:"scrollEarned"`
}

func (p *lessonProgress) toDTO() lessonProgressDTO {
	drafts := make(map[int]map[string]string, len(p.drafts))
	for idx, answers := range p.drafts {
		copied := make(map[string]string, len(answers))
		for key, value := range answers {
			copied[key] = value
		}
		drafts[idx] = copied
	}
	solved := make(map[int]bool, len(p.solved))
	for idx, ok := range p.solved {
		solved[idx] = ok
	}
	lastWrong := make(map[int]bool, len(p.lastWrong))
	for idx, wrong := range p.lastWrong {
		lastWrong[idx] = wrong
	}
	selfChecked := make(map[string]bool, len(p.selfChecked))
	for key, checked := range p.selfChecked {
		selfChecked[key] = checked
	}
	draftUpdatedAt := make(map[int]int64, len(p.draftUpdatedAt))
	for idx, at := range p.draftUpdatedAt {
		draftUpdatedAt[idx] = at
	}
	selfCheckUpdatedAt := make(map[string]int64, len(p.selfCheckUpdatedAt))
	for key, at := range p.selfCheckUpdatedAt {
		selfCheckUpdatedAt[key] = at
	}
	return lessonProgressDTO{
		Drafts: drafts, DraftUpdatedAt: draftUpdatedAt, Solved: solved, LastWrong: lastWrong,
		SelfChecked: selfChecked, SelfCheckUpdatedAt: selfCheckUpdatedAt, ScrollEarned: p.scrollEarned,
	}
}

func (d lessonProgressDTO) toProgress() *lessonProgress {
	p := &lessonProgress{
		drafts: d.Drafts, draftUpdatedAt: d.DraftUpdatedAt, solved: d.Solved, lastWrong: d.LastWrong,
		selfChecked: d.SelfChecked, selfCheckUpdatedAt: d.SelfCheckUpdatedAt, scrollEarned: d.ScrollEarned,
	}
	if p.drafts == nil {
		p.drafts = make(map[int]map[string]string)
	}
	if p.solved == nil {
		p.solved = make(map[int]bool)
	}
	if p.lastWrong == nil {
		p.lastWrong = make(map[int]bool)
	}
	if p.selfChecked == nil {
		p.selfChecked = make(map[string]bool)
	}
	if p.draftUpdatedAt == nil {
		p.draftUpdatedAt = make(map[int]int64)
	}
	if p.selfCheckUpdatedAt == nil {
		p.selfCheckUpdatedAt = make(map[string]int64)
	}
	return p
}

// isUUID reports whether studentID is a real (PostgreSQL-backed) ID.
func isUUID(s string) bool { return reqguard.IsUUID(s) }

func (s *Service) ensureStudentLoaded(ctx context.Context, studentID string) error {
	if s.store == nil || !isUUID(studentID) {
		return nil
	}
	s.mu.Lock()
	if s.loaded[studentID] {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	var st academyState
	if _, err := s.store.Load(ctx, studentID, "academy", &st); err != nil {
		return err
	}
	s.mu.Lock()
	if s.loaded[studentID] {
		s.mu.Unlock()
		return nil
	}
	if s.progress[studentID] == nil {
		s.progress[studentID] = make(map[string]*lessonProgress)
	}
	for code, dto := range st.Lessons {
		s.progress[studentID][code] = dto.toProgress()
	}
	s.loaded[studentID] = true
	s.mu.Unlock()
	return nil
}

func mergeLessonProgress(current, snapshot lessonProgressDTO) lessonProgressDTO {
	out := current
	if out.Drafts == nil {
		out.Drafts = make(map[int]map[string]string)
	}
	if out.Solved == nil {
		out.Solved = make(map[int]bool)
	}
	if out.LastWrong == nil {
		out.LastWrong = make(map[int]bool)
	}
	if out.SelfChecked == nil {
		out.SelfChecked = make(map[string]bool)
	}
	if out.DraftUpdatedAt == nil {
		out.DraftUpdatedAt = make(map[int]int64)
	}
	if out.SelfCheckUpdatedAt == nil {
		out.SelfCheckUpdatedAt = make(map[string]int64)
	}
	for idx, answers := range snapshot.Drafts {
		at, ok := out.DraftUpdatedAt[idx]
		if !ok || snapshot.DraftUpdatedAt[idx] >= at {
			out.Drafts[idx] = answers
			out.DraftUpdatedAt[idx] = snapshot.DraftUpdatedAt[idx]
		}
	}
	for idx, ok := range snapshot.Solved {
		out.Solved[idx] = out.Solved[idx] || ok
	}
	for idx, wrong := range snapshot.LastWrong {
		out.LastWrong[idx] = wrong
	}
	for key, checked := range snapshot.SelfChecked {
		at, ok := out.SelfCheckUpdatedAt[key]
		snapAt := snapshot.SelfCheckUpdatedAt[key]
		switch {
		case !ok:
			if snapAt > 0 {
				// A timestamped explicit uncheck may clear a legacy checkmark
				// that predates per-item timestamps.
				out.SelfChecked[key] = checked
				out.SelfCheckUpdatedAt[key] = snapAt
			} else {
				// Timestamp-free legacy snapshots must not erase a checkmark.
				out.SelfChecked[key] = out.SelfChecked[key] || checked
				out.SelfCheckUpdatedAt[key] = snapAt
			}
		case snapAt > 0 && snapAt >= at:
			out.SelfChecked[key] = checked
			out.SelfCheckUpdatedAt[key] = snapAt
		case snapAt == 0:
			// A timestamp-free snapshot is legacy data. Only OR it into
			// another timestamp-free record; a newer timestamped value wins.
			if at == 0 {
				out.SelfChecked[key] = out.SelfChecked[key] || checked
			}
		}
	}
	out.ScrollEarned = out.ScrollEarned || snapshot.ScrollEarned
	return out
}

func mergeAcademyState(currentRaw []byte, snapshot academyState) (any, error) {
	current := academyState{Lessons: make(map[string]lessonProgressDTO)}
	if len(currentRaw) > 0 {
		if err := json.Unmarshal(currentRaw, &current); err != nil {
			return nil, err
		}
	}
	if current.Lessons == nil {
		current.Lessons = make(map[string]lessonProgressDTO)
	}
	for code, snap := range snapshot.Lessons {
		if existing, ok := current.Lessons[code]; ok {
			current.Lessons[code] = mergeLessonProgress(existing, snap)
		} else {
			current.Lessons[code] = snap
		}
	}
	return current, nil
}

func (s *Service) persistStudent(ctx context.Context, studentID string) error {
	if s.store == nil || !isUUID(studentID) {
		return nil
	}
	s.mu.Lock()
	st := academyState{Lessons: make(map[string]lessonProgressDTO)}
	for code, p := range s.progress[studentID] {
		st.Lessons[code] = p.toDTO()
	}
	s.mu.Unlock()
	if ms, ok := s.store.(state.MergeStore); ok {
		return ms.SaveMerged(ctx, studentID, "academy", func(current []byte) (any, error) {
			return mergeAcademyState(current, st)
		})
	}
	return s.store.Save(ctx, studentID, "academy", st)
}

func (s *Service) getProgLocked(studentID, lessonCode string) *lessonProgress {
	if s.progress[studentID] == nil {
		s.progress[studentID] = make(map[string]*lessonProgress)
	}
	p := s.progress[studentID][lessonCode]
	if p == nil {
		p = &lessonProgress{
			drafts:             make(map[int]map[string]string),
			draftUpdatedAt:     make(map[int]int64),
			solved:             make(map[int]bool),
			lastWrong:          make(map[int]bool),
			selfChecked:        make(map[string]bool),
			selfCheckUpdatedAt: make(map[string]int64),
		}
		s.progress[studentID][lessonCode] = p
	}
	return p
}

func completionPctFor(l lessonDef, p *lessonProgress) int {
	solvedCount := len(p.solved)
	checkedCount := 0
	for _, sc := range l.selfCheck {
		if p.selfChecked[sc.key] {
			checkedCount++
		}
	}
	totalUnits := 4 + len(l.selfCheck)
	if totalUnits == 0 {
		return 0
	}
	pct := ((solvedCount + checkedCount) * 100) / totalUnits
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (s *Service) ListTopics(ctx context.Context, studentID string, grade int, locale string) ([]TopicSummary, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TopicSummary, 0)
	for _, t := range s.topics {
		if grade > 0 && t.grade != grade {
			continue
		}
		lessons := make([]LessonSummary, 0)
		earned := 0
		for _, l := range s.lessons {
			if l.topicCode != t.code {
				continue
			}
			p := s.getProgLocked(studentID, l.code)
			if p.scrollEarned {
				earned++
			}
			lessons = append(lessons, LessonSummary{
				Code:          l.code,
				Title:         l.title.For(locale),
				AgeBand:       l.ageBand,
				CompletionPct: completionPctFor(l, p),
				ScrollEarned:  p.scrollEarned,
				SkillCode:     l.skillCode,
			})
		}
		out = append(out, TopicSummary{
			Code:          t.code,
			Grade:         t.grade,
			Title:         t.title.For(locale),
			AgeBand:       t.ageBand,
			ScrollsEarned: earned,
			TotalScrolls:  len(lessons),
			Lessons:       lessons,
		})
	}
	return out, nil
}

func (s *Service) findLesson(code string) (lessonDef, bool) {
	for _, l := range s.lessons {
		if l.code == code {
			return l, true
		}
	}
	return lessonDef{}, false
}

func seedForLessonQuestion(lessonCode string, idx int) int64 {
	var h int64 = 500
	for _, ch := range lessonCode {
		h = h*31 + int64(ch)
	}
	if h < 0 {
		h = -h
	}
	return h%100000 + int64(idx*17)
}

func (s *Service) GetLesson(ctx context.Context, studentID, lessonCode, locale string) (LessonDetail, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return LessonDetail{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.findLesson(lessonCode)
	if !ok {
		return LessonDetail{}, fmt.Errorf("%w: %q", ErrLessonNotFound, lessonCode)
	}
	p := s.getProgLocked(studentID, lessonCode)
	return s.buildLessonDetailLocked(l, p, locale), nil
}

func (s *Service) SaveDraft(ctx context.Context, studentID, lessonCode, locale string, questionIndex int, answers map[string]string) (LessonDetail, error) {
	if err := reqguard.ValidateAnswers(answers); err != nil {
		return LessonDetail{}, err
	}
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return LessonDetail{}, err
	}
	s.mu.Lock()
	detail, err := func() (LessonDetail, error) {
		l, ok := s.findLesson(lessonCode)
		if !ok {
			return LessonDetail{}, fmt.Errorf("%w: %q", ErrLessonNotFound, lessonCode)
		}
		if questionIndex < 1 || questionIndex > 4 {
			return LessonDetail{}, fmt.Errorf("%w: questionIndex must be 1..4", ErrInvalidInput)
		}
		p := s.getProgLocked(studentID, lessonCode)
		copied := make(map[string]string, len(answers))
		for k, v := range answers {
			copied[k] = v
		}
		p.drafts[questionIndex] = copied
		p.draftUpdatedAt[questionIndex] = time.Now().UnixNano()
		return s.buildLessonDetailLocked(l, p, locale), nil
	}()
	s.mu.Unlock()
	if err != nil {
		return LessonDetail{}, err
	}
	if err := s.persistStudent(ctx, studentID); err != nil {
		return LessonDetail{}, err
	}
	return detail, nil
}

// CheckAnswer evaluates a Do & Learn answer; wrong answers allow unlimited Try Again retries
// while solving all 4 questions atomically awards 1 Star Scroll.
func (s *Service) CheckAnswer(ctx context.Context, studentID, lessonCode, locale string, questionIndex int, answers map[string]string) (mathengine.GradeResult, LessonDetail, error) {
	if err := reqguard.ValidateAnswers(answers); err != nil {
		return mathengine.GradeResult{}, LessonDetail{}, err
	}
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return mathengine.GradeResult{}, LessonDetail{}, err
	}
	s.mu.Lock()
	var shouldAward bool
	res, detail, err := func() (mathengine.GradeResult, LessonDetail, error) {
		l, ok := s.findLesson(lessonCode)
		if !ok {
			return mathengine.GradeResult{}, LessonDetail{}, fmt.Errorf("%w: %q", ErrLessonNotFound, lessonCode)
		}
		if questionIndex < 1 || questionIndex > 4 {
			return mathengine.GradeResult{}, LessonDetail{}, fmt.Errorf("%w: questionIndex must be 1..4", ErrInvalidInput)
		}
		p := s.getProgLocked(studentID, lessonCode)
		copied := make(map[string]string, len(answers))
		for k, v := range answers {
			copied[k] = v
		}
		p.drafts[questionIndex] = copied
		p.draftUpdatedAt[questionIndex] = time.Now().UnixNano()

		seed := seedForLessonQuestion(l.code, questionIndex)
		prob, _ := mathengine.GenerateByCode(l.templateCode, seed, locale)
		gradeRes := mathengine.Grade(prob, answers)

		if gradeRes.IsCorrect {
			p.solved[questionIndex] = true
			p.lastWrong[questionIndex] = false
		} else {
			p.lastWrong[questionIndex] = true
		}

		// Award 1 Star Scroll once all 4 guided Do & Learn questions are solved.
		if len(p.solved) == 4 && !p.scrollEarned {
			if s.ledger == nil {
				p.scrollEarned = true
			} else {
				shouldAward = true
			}
		}
		return gradeRes, s.buildLessonDetailLocked(l, p, locale), nil
	}()
	s.mu.Unlock()
	if err != nil {
		return res, detail, err
	}

	var awardErr error
	if shouldAward {
		// Make the four solved answers durable before crediting: a crash after the credit must
		// never lose the progress that earned it. The scroll-retry path then only re-attempts the
		// (idempotent) credit.
		if err := s.persistStudent(ctx, studentID); err != nil {
			return res, detail, err
		}
		// Keep the ledger outside the service lock; the idempotency key makes
		// a retry after a transient failure safe.
		if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencyScroll,
			Delta:          1,
			SourceType:     "SPACE_ACADEMY",
			SourceID:       lessonCode,
			IdempotencyKey: fmt.Sprintf("academy_scroll:%s:%s", studentID, lessonCode),
		}); err != nil {
			awardErr = fmt.Errorf("%w: %v", ErrRewardUnavailable, err)
		} else {
			s.mu.Lock()
			p := s.getProgLocked(studentID, lessonCode)
			p.scrollEarned = true
			if l, ok := s.findLesson(lessonCode); ok {
				detail = s.buildLessonDetailLocked(l, p, locale)
			}
			s.mu.Unlock()
		}
	}

	// Persist solved answers even if the reward failed, so the retry can use
	// the existing idempotency key without re-answering.
	if err := s.persistStudent(ctx, studentID); err != nil {
		return res, detail, err
	}
	if awardErr != nil {
		return res, detail, awardErr
	}
	return res, detail, nil
}

// RetryScrollAward lets a learner retry a failed Star Scroll credit after a
// reload, without re-answering the four already-solved questions.
func (s *Service) RetryScrollAward(ctx context.Context, studentID, lessonCode, locale string) (LessonDetail, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return LessonDetail{}, err
	}
	s.mu.Lock()
	l, ok := s.findLesson(lessonCode)
	if !ok {
		s.mu.Unlock()
		return LessonDetail{}, fmt.Errorf("%w: %q", ErrLessonNotFound, lessonCode)
	}
	p := s.getProgLocked(studentID, lessonCode)
	if len(p.solved) != 4 {
		s.mu.Unlock()
		return LessonDetail{}, fmt.Errorf("%w: all four Do & Learn questions must be solved first", ErrInvalidInput)
	}
	if p.scrollEarned {
		detail := s.buildLessonDetailLocked(l, p, locale)
		s.mu.Unlock()
		// A prior retry may have credited the ledger but failed to persist;
		// flush before reporting success so a reload cannot lose the scroll.
		if err := s.persistStudent(ctx, studentID); err != nil {
			return detail, err
		}
		return detail, nil
	}
	detail := s.buildLessonDetailLocked(l, p, locale)
	s.mu.Unlock()

	var awardErr error
	if s.ledger == nil {
		s.mu.Lock()
		s.getProgLocked(studentID, lessonCode).scrollEarned = true
		detail = s.buildLessonDetailLocked(l, s.getProgLocked(studentID, lessonCode), locale)
		s.mu.Unlock()
	} else {
		// The four solved answers must be durable before the ledger credit;
		// otherwise a failed save leaves a credited scroll without progress.
		if err := s.persistStudent(ctx, studentID); err != nil {
			return detail, err
		}
		if _, err := s.ledger.Apply(ctx, economy.MutationRequest{
			StudentID:      studentID,
			Currency:       economy.CurrencyScroll,
			Delta:          1,
			SourceType:     "SPACE_ACADEMY",
			SourceID:       lessonCode,
			IdempotencyKey: fmt.Sprintf("academy_scroll:%s:%s", studentID, lessonCode),
		}); err != nil {
			awardErr = fmt.Errorf("%w: %v", ErrRewardUnavailable, err)
		} else {
			s.mu.Lock()
			p = s.getProgLocked(studentID, lessonCode)
			p.scrollEarned = true
			detail = s.buildLessonDetailLocked(l, p, locale)
			s.mu.Unlock()
		}
	}
	if err := s.persistStudent(ctx, studentID); err != nil {
		return detail, err
	}
	if awardErr != nil {
		return detail, awardErr
	}
	return detail, nil
}

func (s *Service) ToggleSelfCheck(ctx context.Context, studentID, lessonCode, itemKey string, checked bool, locale string) (LessonDetail, error) {
	if err := s.ensureStudentLoaded(ctx, studentID); err != nil {
		return LessonDetail{}, err
	}
	s.mu.Lock()
	detail, err := func() (LessonDetail, error) {
		l, ok := s.findLesson(lessonCode)
		if !ok {
			return LessonDetail{}, fmt.Errorf("%w: %q", ErrLessonNotFound, lessonCode)
		}
		valid := false
		for _, item := range l.selfCheck {
			if item.key == itemKey {
				valid = true
				break
			}
		}
		if !valid {
			return LessonDetail{}, fmt.Errorf("%w: self-check item %q not found", ErrInvalidInput, itemKey)
		}
		p := s.getProgLocked(studentID, lessonCode)
		p.selfChecked[itemKey] = checked
		p.selfCheckUpdatedAt[itemKey] = time.Now().UnixNano()
		return s.buildLessonDetailLocked(l, p, locale), nil
	}()
	s.mu.Unlock()
	if err != nil {
		return LessonDetail{}, err
	}
	if err := s.persistStudent(ctx, studentID); err != nil {
		return LessonDetail{}, err
	}
	return detail, nil
}

func (s *Service) buildLessonDetailLocked(l lessonDef, p *lessonProgress, locale string) LessonDetail {
	qs := make([]DoLearnQuestion, 4)
	for i := 0; i < 4; i++ {
		idx := i + 1
		seed := seedForLessonQuestion(l.code, idx)
		prob, _ := mathengine.GenerateByCode(l.templateCode, seed, locale)
		item := DoLearnQuestion{
			Index:       idx,
			Question:    prob.Question,
			DraftAnswer: p.drafts[idx],
			Solved:      p.solved[idx],
			LastWrong:   p.lastWrong[idx],
		}
		if p.solved[idx] {
			sol := prob.Solution
			item.WorkedSolution = &sol
		}
		qs[i] = item
	}

	checks := make([]SelfCheckItem, len(l.selfCheck))
	for i, sc := range l.selfCheck {
		checks[i] = SelfCheckItem{
			Key:     sc.key,
			Label:   sc.text.For(locale),
			Checked: p.selfChecked[sc.key],
		}
	}

	return LessonDetail{
		Code:               l.code,
		TopicCode:          l.topicCode,
		Title:              l.title.For(locale),
		AgeBand:            l.ageBand,
		SkillCode:          l.skillCode,
		StoryVideoTitle:    l.storyVideo.For(locale),
		ConceptVideoTitle:  l.conceptVideo.For(locale),
		StoryVideoURL:      l.storyVideoURL.For(locale),
		ConceptVideoURL:    l.conceptVideoURL.For(locale),
		StoryCaptionsURL:   captionURLFor(l.storyCaptionsURL, locale),
		ConceptCaptionsURL: captionURLFor(l.conceptCaptionsURL, locale),
		StoryYouTubeID:     l.storyYouTubeID.For(locale),
		ConceptYouTubeID:   l.conceptYouTubeID.For(locale),
		CompletionPct:      completionPctFor(l, p),
		ScrollEarned:       p.scrollEarned,
		DoAndLearn:         qs,
		SelfCheck:          checks,
	}
}

// captionURLFor never falls back across languages, because the page labels
// the track with the requested locale.
func captionURLFor(text mathengine.LocaleText, locale string) string {
	if locale == "id" {
		return text.ID
	}
	return text.EN
}

func buildCatalog() ([]topicDef, []lessonDef) {
	topics := []topicDef{
		{code: "g1-whole-numbers-1", grade: 1, ageBand: "6-8", title: mathengine.LocaleText{EN: "Whole Numbers & Place Value", ID: "Bilangan Cacah & Nilai Tempat"}},
		{code: "g1-sub-patterns", grade: 1, ageBand: "6-8", title: mathengine.LocaleText{EN: "Subtraction & Star Patterns", ID: "Pengurangan & Pola Bintang"}},
		{code: "g2-add-1000", grade: 2, ageBand: "7-9", title: mathengine.LocaleText{EN: "Addition within 1,000", ID: "Penjumlahan sampai 1.000"}},
		{code: "g2-bar-models", grade: 2, ageBand: "7-9", title: mathengine.LocaleText{EN: "Multiplication & Comparison Models", ID: "Perkalian & Model Perbandingan"}},
		{code: "g3-mul-div", grade: 3, ageBand: "8-10", title: mathengine.LocaleText{EN: "Multiplication & Division with Remainders", ID: "Perkalian & Pembagian Bersisa"}},
		{code: "g3-area-perim", grade: 3, ageBand: "8-10", title: mathengine.LocaleText{EN: "Area & Perimeter", ID: "Luas & Keliling"}},
		{code: "g4-factors-multiples", grade: 4, ageBand: "9-11", title: mathengine.LocaleText{EN: "Factors & Multiples (LCM/GCF)", ID: "Faktor & Kelipatan (KPK/FPB)"}},
		{code: "g4-decimals", grade: 4, ageBand: "9-11", title: mathengine.LocaleText{EN: "Decimals & Fractions of a Set", ID: "Desimal & Pecahan Himpunan"}},
		{code: "g5-percentage", grade: 5, ageBand: "10-12", title: mathengine.LocaleText{EN: "Percentage & Discounts", ID: "Persentase & Diskon"}},
		{code: "g5-ratio-volume", grade: 5, ageBand: "10-12", title: mathengine.LocaleText{EN: "Ratio & Cuboid Volume", ID: "Rasio & Volume Balok"}},
		{code: "g6-algebra-speed", grade: 6, ageBand: "11-13", title: mathengine.LocaleText{EN: "Algebra & Speed", ID: "Aljabar & Kecepatan"}},
		{code: "g6-circles-hots", grade: 6, ageBand: "11-13", title: mathengine.LocaleText{EN: "Circles & Supposition HOTS", ID: "Lingkaran & HOTS Pengandaian"}},
	}

	lessons := []lessonDef{
		{
			code: "g1-wn-l1", topicCode: "g1-whole-numbers-1", grade: 1, ageBand: "6-8",
			templateCode: "g1_add_within_20", skillCode: "num-add-within-20",
			title:        mathengine.LocaleText{EN: "Counting On & Adding within 20", ID: "Hitung Maju & Penjumlahan sampai 20"},
			storyVideo:   mathengine.LocaleText{EN: "Counting Blue & Gold Stars on Terra Pebble", ID: "Menghitung Bintang Biru & Emas di Planet Kerikil"},
			conceptVideo: mathengine.LocaleText{EN: "Using a Number Line to Add", ID: "Menggunakan Garis Bilangan untuk Penjumlahan"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "count_20", text: mathengine.LocaleText{EN: "I can count on from the larger number within 20.", ID: "Saya bisa menghitung maju dari bilangan lebih besar sampai 20."}},
				{key: "numline_add", text: mathengine.LocaleText{EN: "I can add two numbers using a number line.", ID: "Saya bisa menjumlahkan dua bilangan memakai garis bilangan."}},
			},
		},
		{
			code: "g1-wn-l2", topicCode: "g1-whole-numbers-1", grade: 1, ageBand: "6-8",
			templateCode: "g1_place_value_tens_ones", skillCode: "num-place-value-100",
			title:        mathengine.LocaleText{EN: "Tens and Ones up to 100", ID: "Puluhan dan Satuan sampai 100"},
			storyVideo:   mathengine.LocaleText{EN: "Packing Star Rocks in Bundles of 10", ID: "Mengemas Batu Bintang dalam Ikat 10"},
			conceptVideo: mathengine.LocaleText{EN: "Breaking Numbers into Tens and Ones", ID: "Menguraikan Bilangan Menjadi Puluhan & Satuan"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "tens_ones", text: mathengine.LocaleText{EN: "I can split any 2-digit number into tens and ones.", ID: "Saya bisa menguraikan bilangan 2 angka menjadi puluhan dan satuan."}},
			},
		},
		{
			code: "g1-sp-l1", topicCode: "g1-sub-patterns", grade: 1, ageBand: "6-8",
			templateCode: "g1_sub_within_20", skillCode: "num-sub-within-20",
			title:        mathengine.LocaleText{EN: "Taking Away within 20", ID: "Pengurangan sampai 20"},
			storyVideo:   mathengine.LocaleText{EN: "Sharing Space Stickers", ID: "Berbagi Stiker Antariksa"},
			conceptVideo: mathengine.LocaleText{EN: "Counting Backwards to Subtract", ID: "Menghitung Mundur untuk Pengurangan"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "sub_20", text: mathengine.LocaleText{EN: "I can subtract numbers within 20 accurately.", ID: "Saya bisa mengurangkan bilangan sampai 20 dengan tepat."}},
			},
		},
		{
			code: "g2-a1k-l1", topicCode: "g2-add-1000", grade: 2, ageBand: "7-9",
			templateCode: "g2_add_within_1000", skillCode: "num-add-1000",
			title:        mathengine.LocaleText{EN: "Vertical Addition with Regrouping", ID: "Penjumlahan Bersusun dengan Menyimpan"},
			storyVideo:   mathengine.LocaleText{EN: "Moon Rock Delivery on Planet Tropica", ID: "Pengiriman Batu Bulan di Planet Tropika"},
			conceptVideo: mathengine.LocaleText{EN: "Adding Ones, Tens, and Hundreds", ID: "Menjumlahkan Satuan, Puluhan, dan Ratusan"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "add_1000", text: mathengine.LocaleText{EN: "I can regroup ones and tens when adding 3-digit numbers.", ID: "Saya bisa menyimpan satuan & puluhan saat menjumlahkan 3 angka."}},
			},
		},
		{
			code: "g2-bm-l1", topicCode: "g2-bar-models", grade: 2, ageBand: "7-9",
			templateCode: "g2_bar_model_compare", skillCode: "wp-bar-model-compare",
			title:        mathengine.LocaleText{EN: "Solving 'More Than' with Bar Models", ID: "Menyelesaikan Soal 'Lebih Banyak' dengan Model Batang"},
			storyVideo:   mathengine.LocaleText{EN: "Comparing Cosmic Card Collections", ID: "Membandingkan Koleksi Kartu Kosmik"},
			conceptVideo: mathengine.LocaleText{EN: "Drawing Two Bars to Compare Quantities", ID: "Menggambar Dua Batang untuk Membandingkan"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "draw_bar", text: mathengine.LocaleText{EN: "I can draw a comparison bar model for word problems.", ID: "Saya bisa menggambar model batang perbandingan untuk soal cerita."}},
			},
		},
		{
			code: "g3-md-l1", topicCode: "g3-mul-div", grade: 3, ageBand: "8-10",
			templateCode: "g3_div_with_remainder", skillCode: "num-div-remainder",
			title:        mathengine.LocaleText{EN: "Packing Star Crystals with Remainders", ID: "Mengemas Kristal Bintang dengan Sisa"},
			storyVideo:   mathengine.LocaleText{EN: "Aquaris Cargo Boxes", ID: "Kotak Kargo Planet Akuaris"},
			conceptVideo: mathengine.LocaleText{EN: "Quotient and Remainder Explained", ID: "Penjelasan Hasil Bagi dan Sisa"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "div_rem", text: mathengine.LocaleText{EN: "I can find both quotient and remainder in division.", ID: "Saya bisa mencari hasil bagi dan sisa pembagian."}},
			},
		},
		{
			code: "g3-ap-l1", topicCode: "g3-area-perim", grade: 3, ageBand: "8-10",
			templateCode: "g3_area_perimeter_rect", skillCode: "geo-area-perimeter-rect",
			title:        mathengine.LocaleText{EN: "Solar Panel Area vs Perimeter", ID: "Luas vs Keliling Panel Surya"},
			storyVideo:   mathengine.LocaleText{EN: "Building Solar Wings on Aquaris", ID: "Membangun Sayap Surya di Akuaris"},
			conceptVideo: mathengine.LocaleText{EN: "Length × Width vs Walking Around the Edge", ID: "Panjang × Lebar vs Mengelilingi Sisi"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "area_perim", text: mathengine.LocaleText{EN: "I can tell the difference between area (cm²) and perimeter (cm).", ID: "Saya bisa membedakan luas (cm²) dan keliling (cm)."}},
			},
		},
		{
			code: "g4-fm-l1", topicCode: "g4-factors-multiples", grade: 4, ageBand: "9-11",
			templateCode: "g4_lcm_gcf", skillCode: "num-factors-multiples",
			title:        mathengine.LocaleText{EN: "Syncing Space Beacons with LCM", ID: "Menyelaraskan Mercusuar Antariksa dengan KPK"},
			storyVideo:   mathengine.LocaleText{EN: "Flashing Crystals of Crystalia", ID: "Kristal Berkedip di Planet Kristalia"},
			conceptVideo: mathengine.LocaleText{EN: "Finding the Lowest Common Multiple", ID: "Menentukan Kelipatan Persekutuan Terkecil"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "lcm", text: mathengine.LocaleText{EN: "I can find the LCM of two whole numbers.", ID: "Saya bisa menentukan KPK dari dua bilangan."}},
			},
		},
		{
			code: "g4-dec-l1", topicCode: "g4-decimals", grade: 4, ageBand: "9-11",
			templateCode: "g4_decimal_add_sub", skillCode: "dec-add-sub",
			title:        mathengine.LocaleText{EN: "Adding Tenths & Hundredths", ID: "Menjumlahkan Persepuluhan & Perseratusan"},
			storyVideo:   mathengine.LocaleText{EN: "Rover Distance Log", ID: "Catatan Jarak Rover"},
			conceptVideo: mathengine.LocaleText{EN: "Aligning Decimal Points Correctly", ID: "Menyejajarkan Tanda Desimal dengan Benar"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "dec_add", text: mathengine.LocaleText{EN: "I can add decimals accurately.", ID: "Saya bisa menjumlahkan bilangan desimal dengan tepat."}},
			},
		},
		{
			code: "g5-pct-l1", topicCode: "g5-percentage", grade: 5, ageBand: "10-12",
			templateCode: "g5_percentage_discount", skillCode: "pct-discount",
			title:        mathengine.LocaleText{EN: "Calculating Galaxy Week Discounts", ID: "Menghitung Diskon Pekan Galaksi"},
			storyVideo:   mathengine.LocaleText{EN: "Shopping in Lumina Star Market", ID: "Belanja di Pasar Bintang Lumina"},
			conceptVideo: mathengine.LocaleText{EN: "Finding Part of 100%", ID: "Menghitung Bagian dari 100%"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "pct_disc", text: mathengine.LocaleText{EN: "I can compute sale prices after a percentage discount.", ID: "Saya bisa menghitung harga akhir setelah diskon persen."}},
			},
		},
		{
			code: "g5-rv-l1", topicCode: "g5-ratio-volume", grade: 5, ageBand: "10-12",
			templateCode: "g5_ratio_word_problem", skillCode: "ratio-two-quantities",
			title:        mathengine.LocaleText{EN: "Solving Ratio Word Problems with Units", ID: "Menyelesaikan Soal Rasio dengan Unit"},
			storyVideo:   mathengine.LocaleText{EN: "Sorting Lumina Gems", ID: "Menyortir Permata Lumina"},
			conceptVideo: mathengine.LocaleText{EN: "Unit Method for Two-Quantity Ratios", ID: "Metode Unit untuk Rasio Dua Kuantitas"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "ratio_unit", text: mathengine.LocaleText{EN: "I can find 1 unit from a ratio and total.", ID: "Saya bisa mencari nilai 1 unit dari rasio dan total."}},
			},
		},
		{
			code: "g6-as-l1", topicCode: "g6-algebra-speed", grade: 6, ageBand: "11-13",
			templateCode: "g6_algebra_linear", skillCode: "alg-linear-eq",
			title:        mathengine.LocaleText{EN: "Balancing Simple Equations", ID: "Menyeimbangkan Persamaan Sederhana"},
			storyVideo:   mathengine.LocaleText{EN: "The Balance Vault of Solaris", ID: "Brankas Keseimbangan Planet Solaris"},
			conceptVideo: mathengine.LocaleText{EN: "Isolating the Unknown Variable x", ID: "Mencari Nilai Variabel x"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "alg_x", text: mathengine.LocaleText{EN: "I can solve simple linear equations for x.", ID: "Saya bisa menyelesaikan persamaan linear sederhana untuk x."}},
			},
		},
		{
			code: "g6-ch-l1", topicCode: "g6-circles-hots", grade: 6, ageBand: "11-13",
			templateCode: "g6_circle_circumference", skillCode: "geo-circles",
			title:        mathengine.LocaleText{EN: "Measuring Orbit Ring Circumference", ID: "Mengukur Keliling Cincin Orbit"},
			storyVideo:   mathengine.LocaleText{EN: "Solaris Ring Architects", ID: "Arsitek Cincin Solaris"},
			conceptVideo: mathengine.LocaleText{EN: "Using π = 22/7 to Find Circumference", ID: "Menggunakan π = 22/7 untuk Keliling Lingkaran"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "circle_c", text: mathengine.LocaleText{EN: "I can compute circumference from radius or diameter.", ID: "Saya bisa menghitung keliling dari jari-jari atau diameter."}},
			},
		},
	}
	return topics, applyLessonMedia(lessons)
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

var authSecurity = []map[string][]string{{"accessCookie": {}}, {"bearerAuth": {}}}

// resolveStudent returns the caller's ID and locale. Progress-changing calls require a student
// account: parents/teachers browsing the Academy must not create progress or trigger Star
// Scroll credits under their own IDs.
func (h *Handler) resolveStudent(ctx context.Context, in AuthIn, mutating bool) (string, string, error) {
	locale := in.Locale
	if locale != "id" {
		locale = "en"
	}
	if h.authSvc == nil {
		return "demo-pilot1", locale, nil
	}
	tok := reqguard.Token(in.CookieAccess, in.Authorization)
	if tok == "" {
		return "", locale, auth.ErrInvalidCredentials
	}
	p, err := h.authSvc.ResolvePrincipal(ctx, tok)
	if err != nil {
		return "", locale, err
	}
	if mutating && p.Role != auth.RoleStudent {
		return "", locale, auth.ErrForbidden
	}
	return p.ID, locale, nil
}

// toHTTPError maps service errors to responses without echoing internal (e.g. database) errors.
func toHTTPError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return huma.Error401Unauthorized("authentication required")
	case errors.Is(err, auth.ErrForbidden):
		return huma.Error403Forbidden("student account required")
	case errors.Is(err, ErrLessonNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrInvalidInput), errors.Is(err, reqguard.ErrInvalidAnswers):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, ErrRewardUnavailable):
		slog.WarnContext(ctx, "academy reward credit failed", "err", err)
		return huma.Error503ServiceUnavailable(ErrRewardUnavailable.Error())
	default:
		slog.ErrorContext(ctx, "academy request failed", "err", err)
		return huma.Error500InternalServerError("internal error")
	}
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "academy-topics",
		Method:      http.MethodGet,
		Path:        "/api/v1/academy/topics",
		Summary:     "List Space Academy topics with Star Scroll progress",
		Tags:        []string{"SpaceAcademy"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Grade int `query:"grade" default:"1"`
	}) (*struct {
		Body struct {
			Topics []TopicSummary `json:"topics"`
		}
	}, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn, false)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		out := &struct {
			Body struct {
				Topics []TopicSummary `json:"topics"`
			}
		}{}
		topics, err := h.svc.ListTopics(ctx, id, in.Grade, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		out.Body.Topics = topics
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-lesson-detail",
		Method:      http.MethodGet,
		Path:        "/api/v1/academy/lessons/{code}",
		Summary:     "Get a Space Academy lesson timeline (Story, Watch, Do & Learn, Self-Check)",
		Tags:        []string{"SpaceAcademy"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Code string `path:"code"`
	}) (*struct{ Body LessonDetail }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn, false)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		detail, err := h.svc.GetLesson(ctx, id, in.Code, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body LessonDetail }{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-do-learn-save",
		Method:      http.MethodPost,
		Path:        "/api/v1/academy/do-learn/save",
		Summary:     "Save a draft answer inside Space Academy Do & Learn",
		Tags:        []string{"SpaceAcademy"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Body struct {
			LessonCode    string            `json:"lessonCode"`
			QuestionIndex int               `json:"questionIndex" minimum:"1" maximum:"4"`
			Answers       map[string]string `json:"answers"`
		}
	}) (*struct{ Body LessonDetail }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn, true)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		detail, err := h.svc.SaveDraft(ctx, id, in.Body.LessonCode, locale, in.Body.QuestionIndex, in.Body.Answers)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body LessonDetail }{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-do-learn-check",
		Method:      http.MethodPost,
		Path:        "/api/v1/academy/do-learn/check",
		Summary:     "Check a Do & Learn answer (supports Try Again & awards Star Scroll)",
		Tags:        []string{"SpaceAcademy"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Body struct {
			LessonCode    string            `json:"lessonCode"`
			QuestionIndex int               `json:"questionIndex" minimum:"1" maximum:"4"`
			Answers       map[string]string `json:"answers"`
		}
	}) (*struct {
		Body struct {
			Result mathengine.GradeResult `json:"result"`
			Lesson LessonDetail           `json:"lesson"`
		}
	}, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn, true)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		res, detail, err := h.svc.CheckAnswer(ctx, id, in.Body.LessonCode, locale, in.Body.QuestionIndex, in.Body.Answers)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		out := &struct {
			Body struct {
				Result mathengine.GradeResult `json:"result"`
				Lesson LessonDetail           `json:"lesson"`
			}
		}{}
		out.Body.Result = res
		out.Body.Lesson = detail
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-scroll-retry",
		Method:      http.MethodPost,
		Path:        "/api/v1/academy/scroll/retry",
		Summary:     "Retry a failed Star Scroll award for an already-solved lesson",
		Tags:        []string{"SpaceAcademy"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Body struct {
			LessonCode string `json:"lessonCode"`
		}
	}) (*struct{ Body LessonDetail }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn, true)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		detail, err := h.svc.RetryScrollAward(ctx, id, in.Body.LessonCode, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body LessonDetail }{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-self-check",
		Method:      http.MethodPatch,
		Path:        "/api/v1/academy/self-check",
		Summary:     "Toggle a Space Academy Mission Self-Check learning objective",
		Tags:        []string{"SpaceAcademy"},
		Security:    authSecurity,
	}, func(ctx context.Context, in *struct {
		AuthIn
		Body struct {
			LessonCode string `json:"lessonCode"`
			ItemKey    string `json:"itemKey"`
			Checked    bool   `json:"checked"`
		}
	}) (*struct{ Body LessonDetail }, error) {
		id, locale, err := h.resolveStudent(ctx, in.AuthIn, true)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		detail, err := h.svc.ToggleSelfCheck(ctx, id, in.Body.LessonCode, in.Body.ItemKey, in.Body.Checked, locale)
		if err != nil {
			return nil, toHTTPError(ctx, err)
		}
		return &struct{ Body LessonDetail }{Body: detail}, nil
	})
}
