package reports

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/training"
)

type ProficiencySummary struct {
	MasteryCount    int                  `json:"masteryCount"`
	CompetentCount  int                  `json:"competentCount"`
	BeginningCount  int                  `json:"beginningCount"`
	IncompleteCount int                  `json:"incompleteCount"`
	Skills          []training.SkillView `json:"skills"`
}

type HighScoreBuckets struct {
	Tier9To10 []training.SkillView `json:"tier9To10"`
	Tier6To8  []training.SkillView `json:"tier6To8"`
	Tier4To5  []training.SkillView `json:"tier4To5"`
	Tier1To3  []training.SkillView `json:"tier1To3"`
	Unplayed  []training.SkillView `json:"unplayed"`
}

type Handler struct {
	trainingSvc *training.Service
	authSvc     *auth.Service
}

func NewHandler(trainingSvc *training.Service, authSvc *auth.Service) *Handler {
	return &Handler{trainingSvc: trainingSvc, authSvc: authSvc}
}

type reportQuery struct {
	CookieAccess  string `cookie:"pl_access"`
	Authorization string `header:"Authorization"`
	Curriculum    string `query:"curriculum" default:"ALL"`
	Grade         int    `query:"grade" default:"0"`
	Locale        string `query:"locale" default:"en"`
}

func (h *Handler) resolveStudent(ctx context.Context, in reportQuery) (string, string) {
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

func (h *Handler) BuildProficiency(studentID, curriculum string, grade int, locale string) ProficiencySummary {
	skills := h.trainingSvc.ListSkills(studentID, curriculum, grade, locale)
	out := ProficiencySummary{Skills: skills}
	for _, sk := range skills {
		switch sk.ProficiencyTier {
		case training.TierMastery:
			out.MasteryCount++
		case training.TierCompetent:
			out.CompetentCount++
		case training.TierBeginning:
			out.BeginningCount++
		default:
			out.IncompleteCount++
		}
	}
	return out
}

func (h *Handler) BuildHighScores(studentID, curriculum string, grade int, locale string) HighScoreBuckets {
	skills := h.trainingSvc.ListSkills(studentID, curriculum, grade, locale)
	out := HighScoreBuckets{
		Tier9To10: []training.SkillView{},
		Tier6To8:  []training.SkillView{},
		Tier4To5:  []training.SkillView{},
		Tier1To3:  []training.SkillView{},
		Unplayed:  []training.SkillView{},
	}
	for _, sk := range skills {
		switch {
		case sk.BestSessionScore >= 9:
			out.Tier9To10 = append(out.Tier9To10, sk)
		case sk.BestSessionScore >= 6:
			out.Tier6To8 = append(out.Tier6To8, sk)
		case sk.BestSessionScore >= 4:
			out.Tier4To5 = append(out.Tier4To5, sk)
		case sk.BestSessionScore >= 1:
			out.Tier1To3 = append(out.Tier1To3, sk)
		default:
			out.Unplayed = append(out.Unplayed, sk)
		}
	}
	return out
}

func (h *Handler) RegisterRoutes(_ chi.Router, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "reports-proficiency",
		Method:      http.MethodGet,
		Path:        "/api/v1/reports/proficiency",
		Summary:     "Get student Proficiency % report across all 6 curricula and Grades 1-6",
		Tags:        []string{"Reports"},
	}, func(ctx context.Context, in *reportQuery) (*struct{ Body ProficiencySummary }, error) {
		studentID, locale := h.resolveStudent(ctx, *in)
		return &struct{ Body ProficiencySummary }{
			Body: h.BuildProficiency(studentID, in.Curriculum, in.Grade, locale),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reports-high-score",
		Method:      http.MethodGet,
		Path:        "/api/v1/reports/high-score",
		Summary:     "Get student High Score star-tier report (9-10, 6-8, 4-5, 1-3)",
		Tags:        []string{"Reports"},
	}, func(ctx context.Context, in *reportQuery) (*struct{ Body HighScoreBuckets }, error) {
		studentID, locale := h.resolveStudent(ctx, *in)
		return &struct{ Body HighScoreBuckets }{
			Body: h.BuildHighScores(studentID, in.Curriculum, in.Grade, locale),
		}, nil
	})
}
