package academy

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/mathengine"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/platform/economy"
)

type SelfCheckItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Checked bool   `json:"checked"`
}

type LessonSummary struct {
	Code           string `json:"code"`
	Title          string `json:"title"`
	AgeBand        string `json:"ageBand"`
	CompletionPct  int    `json:"completionPct"`
	ScrollEarned   bool   `json:"scrollEarned"`
	SkillCode      string `json:"skillCode"`
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
	Index           int                         `json:"index"`
	Question        mathengine.RenderedQuestion `json:"question"`
	DraftAnswer     map[string]string           `json:"draftAnswer,omitempty"`
	Solved          bool                        `json:"solved"`
	LastWrong       bool                        `json:"lastWrong"`
	WorkedSolution  *mathengine.WorkedSolution  `json:"workedSolution,omitempty"`
}

type LessonDetail struct {
	Code              string            `json:"code"`
	TopicCode         string            `json:"topicCode"`
	Title             string            `json:"title"`
	AgeBand           string            `json:"ageBand"`
	SkillCode         string            `json:"skillCode"`
	StoryVideoTitle   string            `json:"storyVideoTitle"`
	ConceptVideoTitle string            `json:"conceptVideoTitle"`
	CompletionPct     int               `json:"completionPct"`
	ScrollEarned      bool              `json:"scrollEarned"`
	DoAndLearn        []DoLearnQuestion `json:"doAndLearn"`
	SelfCheck         []SelfCheckItem   `json:"selfCheck"`
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
	selfCheck    []struct {
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
	drafts       map[int]map[string]string
	solved       map[int]bool
	lastWrong    map[int]bool
	selfChecked  map[string]bool
	scrollEarned bool
}

type Service struct {
	mu       sync.Mutex
	ledger   economy.Ledger
	topics   []topicDef
	lessons  []lessonDef
	progress map[string]map[string]*lessonProgress // studentID -> lessonCode -> progress
}

func NewService(ledger economy.Ledger) *Service {
	t, l := buildCatalog()
	return &Service{
		ledger:   ledger,
		topics:   t,
		lessons:  l,
		progress: make(map[string]map[string]*lessonProgress),
	}
}

func (s *Service) getProgLocked(studentID, lessonCode string) *lessonProgress {
	if s.progress[studentID] == nil {
		s.progress[studentID] = make(map[string]*lessonProgress)
	}
	p := s.progress[studentID][lessonCode]
	if p == nil {
		p = &lessonProgress{
			drafts:      make(map[int]map[string]string),
			solved:      make(map[int]bool),
			lastWrong:   make(map[int]bool),
			selfChecked: make(map[string]bool),
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

func (s *Service) ListTopics(studentID string, grade int, locale string) []TopicSummary {
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
	return out
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

func (s *Service) GetLesson(studentID, lessonCode, locale string) (LessonDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.findLesson(lessonCode)
	if !ok {
		return LessonDetail{}, fmt.Errorf("lesson %q not found", lessonCode)
	}
	p := s.getProgLocked(studentID, lessonCode)
	return s.buildLessonDetailLocked(l, p, locale), nil
}

func (s *Service) SaveDraft(studentID, lessonCode, locale string, questionIndex int, answers map[string]string) (LessonDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.findLesson(lessonCode)
	if !ok {
		return LessonDetail{}, fmt.Errorf("lesson %q not found", lessonCode)
	}
	if questionIndex < 1 || questionIndex > 4 {
		return LessonDetail{}, fmt.Errorf("questionIndex must be 1..4")
	}
	p := s.getProgLocked(studentID, lessonCode)
	copied := make(map[string]string, len(answers))
	for k, v := range answers {
		copied[k] = v
	}
	p.drafts[questionIndex] = copied
	return s.buildLessonDetailLocked(l, p, locale), nil
}

// CheckAnswer evaluates a Do & Learn answer; wrong answers allow unlimited Try Again retries
// while solving all 4 questions atomically awards 1 Star Scroll.
func (s *Service) CheckAnswer(ctx context.Context, studentID, lessonCode, locale string, questionIndex int, answers map[string]string) (mathengine.GradeResult, LessonDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.findLesson(lessonCode)
	if !ok {
		return mathengine.GradeResult{}, LessonDetail{}, fmt.Errorf("lesson %q not found", lessonCode)
	}
	if questionIndex < 1 || questionIndex > 4 {
		return mathengine.GradeResult{}, LessonDetail{}, fmt.Errorf("questionIndex must be 1..4")
	}
	p := s.getProgLocked(studentID, lessonCode)
	p.drafts[questionIndex] = answers

	seed := seedForLessonQuestion(l.code, questionIndex)
	prob, _ := mathengine.GenerateByCode(l.templateCode, seed, locale)
	gradeRes := mathengine.Grade(prob, answers)

	if gradeRes.IsCorrect {
		p.solved[questionIndex] = true
		p.lastWrong[questionIndex] = false
	} else {
		p.lastWrong[questionIndex] = true
	}

	// Award 1 Star Scroll atomically once all 4 guided Do & Learn questions are solved
	if len(p.solved) == 4 && !p.scrollEarned {
		p.scrollEarned = true
		if s.ledger != nil {
			_, _ = s.ledger.Apply(ctx, economy.MutationRequest{
				StudentID:      studentID,
				Currency:       economy.CurrencyScroll,
				Delta:          1,
				SourceType:     "SPACE_ACADEMY",
				SourceID:       l.code,
				IdempotencyKey: fmt.Sprintf("academy_scroll:%s:%s", studentID, l.code),
			})
		}
	}

	return gradeRes, s.buildLessonDetailLocked(l, p, locale), nil
}

func (s *Service) ToggleSelfCheck(studentID, lessonCode, itemKey string, checked bool, locale string) (LessonDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.findLesson(lessonCode)
	if !ok {
		return LessonDetail{}, fmt.Errorf("lesson %q not found", lessonCode)
	}
	p := s.getProgLocked(studentID, lessonCode)
	p.selfChecked[itemKey] = checked
	return s.buildLessonDetailLocked(l, p, locale), nil
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
		Code:              l.code,
		TopicCode:         l.topicCode,
		Title:             l.title.For(locale),
		AgeBand:           l.ageBand,
		SkillCode:         l.skillCode,
		StoryVideoTitle:   l.storyVideo.For(locale),
		ConceptVideoTitle: l.conceptVideo.For(locale),
		CompletionPct:     completionPctFor(l, p),
		ScrollEarned:      p.scrollEarned,
		DoAndLearn:        qs,
		SelfCheck:         checks,
	}
}

func buildCatalog() ([]topicDef, []lessonDef) {
	topics := []topicDef{
		{code: "g1-whole-numbers-1", grade: 1, ageBand: "6-8", title: mathengine.LocaleText{EN: "Whole Numbers & Place Value", ID: "Bilangan Cacah & Nilai Tempat"}},
		{code: "g1-sub-patterns", grade: 1, ageBand: "6-8", title: mathengine.LocaleText{EN: "Subtraction & Star Patterns", ID: "Pengurangan & Pola Bintang"}},
		{code: "g2-add-1000", grade: 2, ageBand: "7-9", title: mathengine.LocaleText{EN: "Addition within 1,000", ID: "Penjumlahan sampai 1.000"}},
		{code: "g2-bar-models", grade: 2, ageBand: "7-9", title: mathengine.LocaleText{EN: "Multiplication & Comparison Models", ID: "Perkalian & Model Perbandingan"}},
		{code: "g3-mul-div", grade: 3, ageBand: "8-10", title: mathengine.LocaleText{EN: "Division with Remainders", ID: "Pembagian Bersisa"}},
		{code: "g3-area-perim", grade: 3, ageBand: "8-10", title: mathengine.LocaleText{EN: "Area & Perimeter", ID: "Luas & Keliling"}},
		{code: "g4-factors-multiples", grade: 4, ageBand: "9-11", title: mathengine.LocaleText{EN: "Factors & Multiples (LCM/GCF)", ID: "Faktor & Kelipatan (KPK/FPB)"}},
		{code: "g4-decimals", grade: 4, ageBand: "9-11", title: mathengine.LocaleText{EN: "Decimals & Fractions", ID: "Desimal & Pecahan"}},
		{code: "g5-percentage", grade: 5, ageBand: "10-12", title: mathengine.LocaleText{EN: "Percentage & Discounts", ID: "Persentase & Diskon"}},
		{code: "g5-ratio-volume", grade: 5, ageBand: "10-12", title: mathengine.LocaleText{EN: "Ratio & Volume", ID: "Rasio & Volume"}},
		{code: "g6-algebra-speed", grade: 6, ageBand: "11-13", title: mathengine.LocaleText{EN: "Algebra & Speed", ID: "Aljabar & Kecepatan"}},
		{code: "g6-circles-hots", grade: 6, ageBand: "11-13", title: mathengine.LocaleText{EN: "Circles & HOTS Supposition", ID: "Lingkaran & HOTS Pengandaian"}},
	}

	lessons := []lessonDef{
		{
			code: "g1-wn-l1", topicCode: "g1-whole-numbers-1", grade: 1, ageBand: "6-8",
			templateCode: "g1_add_within_20", skillCode: "num-add-within-20",
			title:        mathengine.LocaleText{EN: "Counting On & Adding within 20", ID: "Hitung Maju & Penjumlahan sampai 20"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Counting Stars on Terra Pebble", ID: "Cerita Pembuka: Menghitung Bintang di Planet Kerikil"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Using a Number Line to Add", ID: "Tonton & Pelajari: Garis Bilangan Penjumlahan"},
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
			title:        mathengine.LocaleText{EN: "Tens & Ones up to 100", ID: "Puluhan & Satuan sampai 100"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Packing Star Rocks in Tens", ID: "Cerita Pembuka: Mengemas Batu Bintang Berkelompok 10"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Place Value Chart", ID: "Tonton & Pelajari: Tabel Nilai Tempat"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "tens_ones", text: mathengine.LocaleText{EN: "I can split 2-digit numbers into tens and ones.", ID: "Saya bisa menguraikan bilangan 2 digit menjadi puluhan dan satuan."}},
			},
		},
		{
			code: "g1-sp-l1", topicCode: "g1-sub-patterns", grade: 1, ageBand: "6-8",
			templateCode: "g1_sub_within_20", skillCode: "num-sub-within-20",
			title:        mathengine.LocaleText{EN: "Taking Away within 20", ID: "Pengurangan sampai 20"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Sharing Space Stickers", ID: "Cerita Pembuka: Berbagi Stiker Antariksa"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Counting Backwards", ID: "Tonton & Pelajari: Menghitung Mundur"},
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
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Moon Rock Delivery", ID: "Cerita Pembuka: Pengiriman Batu Bulan"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Regrouping Ones & Tens", ID: "Tonton & Pelajari: Menyimpan Satuan & Puluhan"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "add_1000", text: mathengine.LocaleText{EN: "I can add 3-digit numbers with regrouping.", ID: "Saya bisa menjumlahkan bilangan 3 angka dengan menyimpan."}},
			},
		},
		{
			code: "g2-bm-l1", topicCode: "g2-bar-models", grade: 2, ageBand: "7-9",
			templateCode: "g2_bar_model_compare", skillCode: "wp-bar-model-compare",
			title:        mathengine.LocaleText{EN: "Comparison Bar Models", ID: "Model Batang Perbandingan"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Cosmic Card Collections", ID: "Cerita Pembuka: Koleksi Kartu Kosmik"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Drawing Two Comparison Bars", ID: "Tonton & Pelajari: Menggambar Dua Batang Perbandingan"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "draw_bar", text: mathengine.LocaleText{EN: "I can draw a bar model for 'more than' word problems.", ID: "Saya bisa menggambar model batang untuk soal cerita perbandingan."}},
			},
		},
		{
			code: "g3-md-l1", topicCode: "g3-mul-div", grade: 3, ageBand: "8-10",
			templateCode: "g3_div_with_remainder", skillCode: "num-div-remainder",
			title:        mathengine.LocaleText{EN: "Quotient & Remainder", ID: "Hasil Bagi & Sisa"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Aquaris Crystal Boxes", ID: "Cerita Pembuka: Kotak Kristal Akuaris"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Long Division with Remainders", ID: "Tonton & Pelajari: Pembagian Bersisa"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "div_rem", text: mathengine.LocaleText{EN: "I can compute quotient and remainder.", ID: "Saya bisa menghitung hasil bagi dan sisa."}},
			},
		},
		{
			code: "g3-ap-l1", topicCode: "g3-area-perim", grade: 3, ageBand: "8-10",
			templateCode: "g3_area_perimeter_rect", skillCode: "geo-area-perimeter-rect",
			title:        mathengine.LocaleText{EN: "Area vs Perimeter of Rectangles", ID: "Luas vs Keliling Persegi Panjang"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Solar Panel Wings", ID: "Cerita Pembuka: Sayap Panel Surya"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Area and Perimeter Formulas", ID: "Tonton & Pelajari: Rumus Luas dan Keliling"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "area_perim", text: mathengine.LocaleText{EN: "I know the difference between area (cm²) and perimeter (cm).", ID: "Saya memahami beda luas (cm²) dan keliling (cm)."}},
			},
		},
		{
			code: "g4-fm-l1", topicCode: "g4-factors-multiples", grade: 4, ageBand: "9-11",
			templateCode: "g4_lcm_gcf", skillCode: "num-factors-multiples",
			title:        mathengine.LocaleText{EN: "Syncing Space Beacons with LCM", ID: "Menyelaraskan Mercusuar dengan KPK"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Crystalia Beacons", ID: "Cerita Pembuka: Mercusuar Kristalia"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Lowest Common Multiple", ID: "Tonton & Pelajari: Kelipatan Persekutuan Terkecil"},
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
			title:        mathengine.LocaleText{EN: "Adding Decimals", ID: "Menjumlahkan Desimal"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Rover Distance Log", ID: "Cerita Pembuka: Catatan Jarak Rover"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Aligning Decimal Points", ID: "Tonton & Pelajari: Menyejajarkan Koma Desimal"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "dec_add", text: mathengine.LocaleText{EN: "I can add decimal numbers accurately.", ID: "Saya bisa menjumlahkan bilangan desimal."}},
			},
		},
		{
			code: "g5-pct-l1", topicCode: "g5-percentage", grade: 5, ageBand: "10-12",
			templateCode: "g5_percentage_discount", skillCode: "pct-discount",
			title:        mathengine.LocaleText{EN: "Galaxy Week Discounts", ID: "Diskon Pekan Galaksi"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Lumina Star Market", ID: "Cerita Pembuka: Pasar Bintang Lumina"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Percentage Discounts", ID: "Tonton & Pelajari: Menghitung Diskon Persen"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "pct_disc", text: mathengine.LocaleText{EN: "I can find final prices after a discount.", ID: "Saya bisa menghitung harga setelah diskon."}},
			},
		},
		{
			code: "g5-rv-l1", topicCode: "g5-ratio-volume", grade: 5, ageBand: "10-12",
			templateCode: "g5_ratio_word_problem", skillCode: "ratio-two-quantities",
			title:        mathengine.LocaleText{EN: "Two-Quantity Ratio Units", ID: "Metode Unit Rasio"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Sorting Lumina Gems", ID: "Cerita Pembuka: Menyortir Permata Lumina"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Ratio Unit Method", ID: "Tonton & Pelajari: Metode Unit Rasio"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "ratio_unit", text: mathengine.LocaleText{EN: "I can solve ratio problems using 1 unit.", ID: "Saya bisa menyelesaikan soal rasio dengan 1 unit."}},
			},
		},
		{
			code: "g6-as-l1", topicCode: "g6-algebra-speed", grade: 6, ageBand: "11-13",
			templateCode: "g6_algebra_linear", skillCode: "alg-linear-eq",
			title:        mathengine.LocaleText{EN: "Solving Simple Algebra", ID: "Menyelesaikan Aljabar Dasar"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Balance Vault of Solaris", ID: "Cerita Pembuka: Brankas Solaris"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Isolating Variable x", ID: "Tonton & Pelajari: Mencari Nilai x"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "alg_x", text: mathengine.LocaleText{EN: "I can solve simple linear equations.", ID: "Saya bisa menyelesaikan persamaan linear sederhana."}},
			},
		},
		{
			code: "g6-ch-l1", topicCode: "g6-circles-hots", grade: 6, ageBand: "11-13",
			templateCode: "g6_circle_circumference", skillCode: "geo-circles",
			title:        mathengine.LocaleText{EN: "Circle Circumference", ID: "Keliling Lingkaran"},
			storyVideo:   mathengine.LocaleText{EN: "Story Launch: Solaris Ring Architects", ID: "Cerita Pembuka: Arsitek Cincin Solaris"},
			conceptVideo: mathengine.LocaleText{EN: "Watch & Learn: Using π = 22/7", ID: "Tonton & Pelajari: Menggunakan π = 22/7"},
			selfCheck: []struct {
				key  string
				text mathengine.LocaleText
			}{
				{key: "circle_c", text: mathengine.LocaleText{EN: "I can calculate circle circumference.", ID: "Saya bisa menghitung keliling lingkaran."}},
			},
		},
	}
	return topics, lessons
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

func (h *Handler) resolveStudent(ctx context.Context, in authIn) (string, string) {
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
				return p.ID, locale
			}
		}
	}
	return "demo-pilot1", locale
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "academy-topics",
		Method:      http.MethodGet,
		Path:        "/api/v1/academy/topics",
		Summary:     "List Space Academy topics with Star Scroll progress",
		Tags:        []string{"SpaceAcademy"},
	}, func(ctx context.Context, in *struct {
		authIn
		Grade int `query:"grade" default:"1"`
	}) (*struct {
		Body struct {
			Topics []TopicSummary `json:"topics"`
		}
	}, error) {
		id, locale := h.resolveStudent(ctx, in.authIn)
		out := &struct {
			Body struct {
				Topics []TopicSummary `json:"topics"`
			}
		}{}
		out.Body.Topics = h.svc.ListTopics(id, in.Grade, locale)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-lesson-detail",
		Method:      http.MethodGet,
		Path:        "/api/v1/academy/lessons/{code}",
		Summary:     "Get a Space Academy lesson timeline (Story, Watch, Do & Learn, Self-Check)",
		Tags:        []string{"SpaceAcademy"},
	}, func(ctx context.Context, in *struct {
		authIn
		Code string `path:"code"`
	}) (*struct{ Body LessonDetail }, error) {
		id, locale := h.resolveStudent(ctx, in.authIn)
		detail, err := h.svc.GetLesson(id, in.Code, locale)
		if err != nil {
			return nil, huma.Error404NotFound(err.Error())
		}
		return &struct{ Body LessonDetail }{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-do-learn-save",
		Method:      http.MethodPost,
		Path:        "/api/v1/academy/do-learn/save",
		Summary:     "Save a draft answer inside Space Academy Do & Learn",
		Tags:        []string{"SpaceAcademy"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			LessonCode    string            `json:"lessonCode"`
			QuestionIndex int               `json:"questionIndex" minimum:"1" maximum:"4"`
			Answers       map[string]string `json:"answers"`
		}
	}) (*struct{ Body LessonDetail }, error) {
		id, locale := h.resolveStudent(ctx, in.authIn)
		detail, err := h.svc.SaveDraft(id, in.Body.LessonCode, locale, in.Body.QuestionIndex, in.Body.Answers)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body LessonDetail }{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "academy-do-learn-check",
		Method:      http.MethodPost,
		Path:        "/api/v1/academy/do-learn/check",
		Summary:     "Check a Do & Learn answer (supports Try Again & awards Star Scroll)",
		Tags:        []string{"SpaceAcademy"},
	}, func(ctx context.Context, in *struct {
		authIn
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
		id, locale := h.resolveStudent(ctx, in.authIn)
		res, detail, err := h.svc.CheckAnswer(ctx, id, in.Body.LessonCode, locale, in.Body.QuestionIndex, in.Body.Answers)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
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
		OperationID: "academy-self-check",
		Method:      http.MethodPatch,
		Path:        "/api/v1/academy/self-check",
		Summary:     "Toggle a Space Academy Mission Self-Check learning objective",
		Tags:        []string{"SpaceAcademy"},
	}, func(ctx context.Context, in *struct {
		authIn
		Body struct {
			LessonCode string `json:"lessonCode"`
			ItemKey    string `json:"itemKey"`
			Checked    bool   `json:"checked"`
		}
	}) (*struct{ Body LessonDetail }, error) {
		id, locale := h.resolveStudent(ctx, in.authIn)
		detail, err := h.svc.ToggleSelfCheck(id, in.Body.LessonCode, in.Body.ItemKey, in.Body.Checked, locale)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body LessonDetail }{Body: detail}, nil
	})
}
