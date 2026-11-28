package academy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type recordingLedger struct {
	mu       sync.Mutex
	applied  []economy.MutationRequest
	failNext bool
}

func (l *recordingLedger) Apply(_ context.Context, req economy.MutationRequest) (economy.MutationResult, error) {
	return l.apply(req)
}

func (l *recordingLedger) ApplyBatch(_ context.Context, reqs []economy.MutationRequest) ([]economy.MutationResult, error) {
	out := make([]economy.MutationResult, 0, len(reqs))
	for _, req := range reqs {
		res, err := l.apply(req)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (l *recordingLedger) apply(req economy.MutationRequest) (economy.MutationResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failNext {
		l.failNext = false
		return economy.MutationResult{}, errors.New("ledger temporarily unavailable")
	}
	l.applied = append(l.applied, req)
	return economy.MutationResult{BalanceAfter: req.Delta}, nil
}

func (l *recordingLedger) scrolls() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var total int64
	for _, req := range l.applied {
		if req.Currency == economy.CurrencyScroll {
			total += req.Delta
		}
	}
	return total
}

func TestSpaceAcademySaveTryAgainAndStarScroll(t *testing.T) {
	ledger := &recordingLedger{}
	svc := NewService(ledger)

	topics, err := svc.ListTopics(context.Background(), "stu-1", 0, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) < 12 {
		t.Fatalf("expected >=12 topics across G1-G6, got %d", len(topics))
	}

	// Save draft persists across reloads
	if _, err := svc.SaveDraft(context.Background(), "stu-1", "g1-wn-l1", "en", 1, map[string]string{"answer": "9"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := svc.GetLesson(context.Background(), "stu-1", "g1-wn-l1", "en")
	if err != nil || reloaded.DoAndLearn[0].DraftAnswer["answer"] != "9" {
		t.Fatalf("draft did not survive a reload: err=%v draft=%v", err, reloaded.DoAndLearn[0].DraftAnswer)
	}

	// Unknown self-check keys are rejected
	if _, err := svc.ToggleSelfCheck(context.Background(), "stu-1", "g1-wn-l1", "not-a-real-item", true, "en"); err == nil {
		t.Fatal("expected unknown self-check item to be rejected")
	}

	// Wrong check marks LastWrong=true and allows Try Again!
	wrongRes, afterWrong, err := svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", 1, map[string]string{"answer": "-999"})
	if err != nil || wrongRes.IsCorrect || !afterWrong.DoAndLearn[0].LastWrong {
		t.Fatalf("expected wrong check to allow Try Again, got res=%+v", wrongRes)
	}

	// The fourth solved question hits a failing ledger; the answer stays
	// solved and a retry credits exactly one Star Scroll.
	for idx := 1; idx <= 4; idx++ {
		q := afterWrong.DoAndLearn[idx-1].Question
		prob, _ := mathengine.GenerateByCode(q.TemplateCode, q.Seed, "en")
		if idx == 4 {
			ledger.failNext = true
			if _, _, err = svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", idx, prob.Solution.CorrectAnswers); err == nil {
				t.Fatal("expected the failed Star Scroll award to surface as an error")
			}
			if _, afterWrong, err = svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", idx, prob.Solution.CorrectAnswers); err != nil {
				t.Fatalf("retry after ledger failure: %v", err)
			}
			continue
		}
		if _, afterWrong, err = svc.CheckAnswer(context.Background(), "stu-1", "g1-wn-l1", "en", idx, prob.Solution.CorrectAnswers); err != nil {
			t.Fatalf("check q%d failed: %v", idx, err)
		}
	}

	if !afterWrong.ScrollEarned {
		t.Fatalf("expected Star Scroll awarded after solving 4/4 Do & Learn questions")
	}
	if got := ledger.scrolls(); got != 1 {
		t.Fatalf("expected exactly 1 Star Scroll credited, got %d", got)
	}

	// Toggle Self-Check pushes completion to 100%
	_, _ = svc.ToggleSelfCheck(context.Background(), "stu-1", "g1-wn-l1", "count_20", true, "en")
	final, _ := svc.ToggleSelfCheck(context.Background(), "stu-1", "g1-wn-l1", "numline_add", true, "en")
	if final.CompletionPct != 100 {
		t.Fatalf("expected 100%% completion, got %d%%", final.CompletionPct)
	}
}

// The content manifest is the authoring contract for the compiled catalog.
// This test loads it so the two copies cannot drift silently.
func TestCatalogMatchesContentManifest(t *testing.T) {
	raw, err := os.ReadFile("../../../../../content/academy/g1-g6-lessons.json")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("content manifest not present in this checkout")
	}
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Topics []struct {
			Code         string            `json:"code"`
			Grade        int               `json:"grade"`
			AgeBand      string            `json:"ageBand"`
			Translations map[string]string `json:"translations"`
			Lessons      []struct {
				Code         string            `json:"code"`
				AgeBand      string            `json:"ageBand"`
				TemplateCode string            `json:"templateCode"`
				SkillCode    string            `json:"skillCode"`
				Translations map[string]string `json:"translations"`
				StoryVideo   map[string]string `json:"storyVideo"`
				ConceptVideo map[string]string `json:"conceptVideo"`
				SelfCheck    []struct {
					Key string `json:"key"`
					EN  string `json:"en"`
					ID  string `json:"id"`
				} `json:"selfCheck"`
			} `json:"lessons"`
		} `json:"topics"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}

	topics, lessons := buildCatalog()
	if len(topics) != len(manifest.Topics) {
		t.Fatalf("topic count drift: catalog=%d manifest=%d", len(topics), len(manifest.Topics))
	}
	lessonByCode := make(map[string]lessonDef, len(lessons))
	for _, l := range lessons {
		lessonByCode[l.code] = l
	}
	for _, mt := range manifest.Topics {
		var topic *topicDef
		for i := range topics {
			if topics[i].code == mt.Code {
				topic = &topics[i]
				break
			}
		}
		if topic == nil {
			t.Fatalf("topic %s missing from catalog", mt.Code)
		}
		if topic.grade != mt.Grade || topic.ageBand != mt.AgeBand ||
			topic.title.EN != mt.Translations["en"] || topic.title.ID != mt.Translations["id"] {
			t.Fatalf("topic %s metadata drift: %+v vs %+v", mt.Code, topic, mt)
		}
		for _, ml := range mt.Lessons {
			l, ok := lessonByCode[ml.Code]
			if !ok {
				t.Fatalf("lesson %s missing from catalog", ml.Code)
			}
			if l.grade != mt.Grade || l.ageBand != ml.AgeBand ||
				l.templateCode != ml.TemplateCode || l.skillCode != ml.SkillCode ||
				l.title.EN != ml.Translations["en"] || l.title.ID != ml.Translations["id"] ||
				l.storyVideo.EN != ml.StoryVideo["en"] || l.storyVideo.ID != ml.StoryVideo["id"] ||
				l.conceptVideo.EN != ml.ConceptVideo["en"] || l.conceptVideo.ID != ml.ConceptVideo["id"] {
				t.Fatalf("lesson %s metadata drift", ml.Code)
			}
			if len(l.selfCheck) != len(ml.SelfCheck) {
				t.Fatalf("lesson %s self-check count drift", ml.Code)
			}
			for i, mc := range ml.SelfCheck {
				if l.selfCheck[i].key != mc.Key || l.selfCheck[i].text.EN != mc.EN || l.selfCheck[i].text.ID != mc.ID {
					t.Fatalf("lesson %s self-check %d drift", ml.Code, i)
				}
			}
			// Every lesson must expose the guided Do & Learn template.
			if l.templateCode == "" {
				t.Fatalf("lesson %s has no Do & Learn template", ml.Code)
			}
		}
	}
}

type memoryStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemoryStore() *memoryStore {
	return &memoryStore{data: make(map[string][]byte)}
}

func (m *memoryStore) key(studentID, module string) string { return studentID + ":" + module }

func (m *memoryStore) Load(_ context.Context, studentID, module string, dest any) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, ok := m.data[m.key(studentID, module)]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dest)
}

func (m *memoryStore) Save(_ context.Context, studentID, module string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.data[m.key(studentID, module)] = raw
	m.mu.Unlock()
	return nil
}

func (m *memoryStore) SaveMerged(_ context.Context, studentID, module string, merge func([]byte) (any, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.key(studentID, module)
	merged, err := merge(m.data[key])
	if err != nil {
		return err
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	m.data[key] = raw
	return nil
}

func TestAcademyRetryScrollAfterReload(t *testing.T) {
	store := newMemoryStore()
	ledger := &recordingLedger{}
	svc := NewServiceWithStore(store, ledger)

	lesson, err := svc.GetLesson(context.Background(), "11111111-1111-1111-1111-111111111111", "g1-wn-l1", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range lesson.DoAndLearn {
		prob, _ := mathengine.GenerateByCode(item.Question.TemplateCode, item.Question.Seed, "en")
		if item.Index == 4 {
			ledger.failNext = true
		}
		if _, _, err := svc.CheckAnswer(context.Background(), "11111111-1111-1111-1111-111111111111", "g1-wn-l1", "en", item.Index, prob.Solution.CorrectAnswers); err != nil && item.Index != 4 {
			t.Fatalf("check q%d: %v", item.Index, err)
		}
	}

	// A fresh service reloads all four solved answers and retries the award.
	reloaded := NewServiceWithStore(store, ledger)
	before, _ := reloaded.GetLesson(context.Background(), "11111111-1111-1111-1111-111111111111", "g1-wn-l1", "en")
	for _, q := range before.DoAndLearn {
		if !q.Solved {
			t.Fatalf("expected Q%d to stay solved after reload", q.Index)
		}
	}
	detail, err := reloaded.RetryScrollAward(context.Background(), "11111111-1111-1111-1111-111111111111", "g1-wn-l1", "en")
	if err != nil {
		t.Fatalf("retry scroll after reload: %v", err)
	}
	if !detail.ScrollEarned {
		t.Fatal("expected Star Scroll to be earned after retry")
	}
	if got := ledger.scrolls(); got != 1 {
		t.Fatalf("expected exactly one Star Scroll credit, got %d", got)
	}
}

func TestApplyLessonMediaSelectsLocale(t *testing.T) {
	original := lessonMediaJSON
	t.Cleanup(func() { lessonMediaJSON = original })
	lessonMediaJSON = []byte(`{
		"lessons": {
			"media-test": {
				"storyVideoUrls": {"en": "https://cdn.example/story-en.mp4", "id": "https://cdn.example/story-id.mp4"},
				"conceptVideoUrls": {"en": "https://cdn.example/concept-en.mp4", "id": "https://cdn.example/concept-id.mp4"},
				"storyCaptionUrls": {"en": "https://cdn.example/story-en.vtt", "id": "https://cdn.example/story-id.vtt"},
				"conceptCaptionUrls": {"en": "https://cdn.example/concept-en.vtt", "id": "https://cdn.example/concept-id.vtt"}
			}
		}
	}`)
	lessons := applyLessonMedia([]lessonDef{{code: "media-test"}})
	got := lessons[0]
	if got.storyVideoURL.EN != "https://cdn.example/story-en.mp4" || got.storyVideoURL.ID != "https://cdn.example/story-id.mp4" {
		t.Fatalf("story media locale selection failed: %+v", got.storyVideoURL)
	}
	if got.conceptVideoURL.EN == "" || got.conceptCaptionsURL.ID == "" {
		t.Fatalf("concept media/captions missing: %+v", got)
	}

	// Lessons without media keep empty URLs so the UI falls back to the placeholder.
	plain := applyLessonMedia([]lessonDef{{code: "no-media"}})
	if plain[0].storyVideoURL.EN != "" || plain[0].conceptVideoURL.ID != "" {
		t.Fatalf("unexpected media for lesson without an entry: %+v", plain[0])
	}
}
