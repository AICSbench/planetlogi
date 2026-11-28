package mathengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAllProceduralTemplatesDeterministicBilingual(t *testing.T) {
	reg := Registry()
	if len(reg) < 60 {
		t.Fatalf("expected at least 60 procedural templates across G1-G6, got %d", len(reg))
	}

	gradeCounts := map[int]int{}
	for _, meta := range reg {
		gradeCounts[meta.Grade]++
		for _, locale := range []string{"en", "id"} {
			p1, ok1 := GenerateByCode(meta.Code, 42, locale)
			p2, ok2 := GenerateByCode(meta.Code, 42, locale)
			if !ok1 || !ok2 {
				t.Fatalf("failed to generate template %s", meta.Code)
			}
			if p1.Question.SkillCode != meta.SkillCode || p1.Question.SkillName == "" {
				t.Fatalf("template %s (%s) has inconsistent skill metadata", meta.Code, locale)
			}
			if p1.Question.Stem == "" || p1.Question.TTSText == "" {
				t.Fatalf("empty stem or ttsText for %s (%s)", meta.Code, locale)
			}
			if p1.Question.Stem != p2.Question.Stem {
				t.Fatalf("non-deterministic stem for %s", meta.Code)
			}
			if len(p1.Solution.CorrectAnswers) == 0 || len(p1.Solution.Steps) == 0 {
				t.Fatalf("missing answer or worked solution for %s", meta.Code)
			}

			// Verify serialized RenderedQuestion NEVER leaks CorrectAnswers
			rawJSON, err := json.Marshal(p1.Question)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(rawJSON), "correctAnswers") {
				t.Fatalf("RenderedQuestion leaked correctAnswers JSON field!")
			}

			// Submitting exact expected answers MUST grade correct
			res := Grade(p1, p1.Solution.CorrectAnswers)
			if !res.IsCorrect {
				t.Fatalf("expected correct grading for %s (%s) with %v", meta.Code, locale, p1.Solution.CorrectAnswers)
			}
		}
	}

	for g := 1; g <= 6; g++ {
		if gradeCounts[g] < 10 {
			t.Fatalf("expected at least 10 templates for Grade %d, got %d", g, gradeCounts[g])
		}
	}
}

func TestGraderLocaleDecimalsAndFractions(t *testing.T) {
	pDec, ok := GenerateByCode("g4_decimal_add_sub", 101, "id")
	if !ok {
		t.Fatal("missing decimal template")
	}
	ansDot := pDec.Solution.CorrectAnswers["answer"]
	ansComma := strings.ReplaceAll(ansDot, ".", ",")
	if !Grade(pDec, map[string]string{"answer": ansComma}).IsCorrect {
		t.Fatalf("expected Indonesian comma decimal %q to match %q", ansComma, ansDot)
	}
	if !Grade(pDec, map[string]string{"answer": " " + ansDot + " "}).IsCorrect {
		t.Fatalf("expected padded dot decimal to pass")
	}

	// Fraction equivalence: 2/4 == 1/2
	fracProb := GeneratedProblem{
		Solution:        WorkedSolution{CorrectAnswers: map[string]string{"answer": "1/2"}},
		GradingStrategy: "fraction_equiv",
	}
	if !Grade(fracProb, map[string]string{"answer": "2/4"}).IsCorrect {
		t.Fatalf("expected 2/4 to be equivalent to 1/2")
	}
	if Grade(fracProb, map[string]string{"answer": "3/4"}).IsCorrect {
		t.Fatalf("expected 3/4 to fail against 1/2")
	}
}

func TestFractionFieldsGradeAsOneRational(t *testing.T) {
	problem := buildFraction("fraction-test", 1, 4, "fraction-test", LocaleText{}, "", 1, "en", "", 5, 2, nil, nil)
	for _, tc := range []struct {
		num, den string
		correct  bool
	}{
		{"10", "4", true},
		{"10", "2", false},
		{"10", "0", false},
		{"10", "", false},
		{"2.5", "1", false},
		{"2500000000000000001", "1000000000000000000", false},
	} {
		if got := Grade(problem, map[string]string{"numerator": tc.num, "denominator": tc.den}).IsCorrect; got != tc.correct {
			t.Errorf("grading %s/%s against 5/2: got %t, want %t", tc.num, tc.den, got, tc.correct)
		}
	}
}

func TestGeneratedClockAndTriangleQuestions(t *testing.T) {
	clock := regexp.MustCompile(`\d\d:\d\d`)
	for _, locale := range []string{"en", "id"} {
		rolledOver := false
		for seed := int64(1); seed <= 256; seed++ {
			p, _ := GenerateByCode("g2_clock_minutes", seed, locale)
			times := clock.FindAllString(p.Question.Stem, -1)
			if len(times) != 2 {
				t.Fatalf("seed %d (%s) did not generate two clock times: %q", seed, locale, p.Question.Stem)
			}
			start, startErr := time.Parse("15:04", times[0])
			end, endErr := time.Parse("15:04", times[1])
			if startErr != nil || endErr != nil || end.Before(start) {
				t.Fatalf("invalid clock times at seed %d (%s): %v", seed, locale, times)
			}
			rolledOver = rolledOver || start.Hour() != end.Hour()
			if got, want := p.Solution.CorrectAnswers["answer"], strconv.Itoa(int(end.Sub(start).Minutes())); got != want {
				t.Fatalf("seed %d (%s): duration %s, want %s", seed, locale, got, want)
			}

			p, _ = GenerateByCode("g4_angles_triangle_mcq", seed, locale)
			seen := make(map[string]bool)
			for _, opt := range p.Question.Options {
				if seen[opt.Label] {
					t.Fatalf("seed %d (%s) repeats angle option %s", seed, locale, opt.Label)
				}
				seen[opt.Label] = true
			}
		}
		if !rolledOver {
			t.Fatalf("no clock fixture crossed an hour (%s)", locale)
		}
	}
}

func TestTTSVerbalizer(t *testing.T) {
	en := VerbalizeMath("3/4 + 2 × 5 = 10", "en")
	if !strings.Contains(en, "3 over 4") || !strings.Contains(en, "times") {
		t.Fatalf("unexpected en verbalization: %q", en)
	}
	id := VerbalizeMath("3/4 + 2 × 5 = 10", "id")
	if !strings.Contains(id, "3 per 4") || !strings.Contains(id, "dikali") {
		t.Fatalf("unexpected id verbalization: %q", id)
	}
}

func TestContentPacksBilingualSchemaValidation(t *testing.T) {
	currPath := filepath.Join("..", "..", "..", "..", "content", "curricula", "g1-g6-curricula.json")
	currBytes, err := os.ReadFile(currPath)
	if err != nil {
		t.Fatalf("failed reading curriculum taxonomy file: %v", err)
	}
	var currDoc struct {
		Curricula []struct {
			Code         string     `json:"code"`
			Translations LocaleText `json:"translations"`
		} `json:"curricula"`
		Skills []struct {
			Code         string     `json:"code"`
			Grade        int        `json:"grade"`
			TemplateCode string     `json:"templateCode"`
			Translations LocaleText `json:"translations"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(currBytes, &currDoc); err != nil {
		t.Fatalf("invalid curricula json: %v", err)
	}
	if len(currDoc.Curricula) != 6 {
		t.Fatalf("expected 6 curricula, got %d", len(currDoc.Curricula))
	}
	for _, s := range currDoc.Skills {
		if s.Translations.EN == "" || s.Translations.ID == "" {
			t.Fatalf("skill %s missing en or id translation", s.Code)
		}
		if s.Grade < 1 || s.Grade > 6 {
			t.Fatalf("skill %s has out-of-range grade %d", s.Code, s.Grade)
		}
		if s.TemplateCode != "" {
			if _, ok := GenerateByCode(s.TemplateCode, 1, "en"); !ok {
				t.Fatalf("skill %s references unknown template %s", s.Code, s.TemplateCode)
			}
		}
	}

	qPath := filepath.Join("..", "..", "..", "..", "content", "questions", "g1-g6-curated-pack.json")
	qBytes, err := os.ReadFile(qPath)
	if err != nil {
		t.Fatalf("failed reading curated questions file: %v", err)
	}
	var qDoc struct {
		Questions []struct {
			Code         string     `json:"code"`
			Grade        int        `json:"grade"`
			SkillCode    string     `json:"skillCode"`
			Stem         LocaleText `json:"stem"`
			SolutionText LocaleText `json:"solutionText"`
			Answer       string     `json:"answer"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(qBytes, &qDoc); err != nil {
		t.Fatalf("invalid curated questions json: %v", err)
	}
	if len(qDoc.Questions) < 300 {
		t.Fatalf("expected at least 300 curated bilingual questions across G1-G6, got %d", len(qDoc.Questions))
	}
	for _, q := range qDoc.Questions {
		if q.Stem.EN == "" || q.Stem.ID == "" || q.SolutionText.EN == "" || q.SolutionText.ID == "" || q.Answer == "" {
			t.Fatalf("curated question %s incomplete bilingual fields", q.Code)
		}
	}
}
