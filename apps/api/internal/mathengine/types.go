package mathengine

// InputType enumerates all supported elementary Grades 1-6 input widgets.
type InputType string

const (
	InputSingleNumeric InputType = "single_numeric"
	InputMultiNumeric  InputType = "multi_numeric"
	InputText          InputType = "short_text"
	InputMCQ           InputType = "mcq"
	InputFraction      InputType = "fraction"
)

type LocaleText struct {
	EN string `json:"en"`
	ID string `json:"id"`
}

func (l LocaleText) For(locale string) string {
	if locale == "id" && l.ID != "" {
		return l.ID
	}
	return l.EN
}

type InputPart struct {
	Key    string `json:"key"`
	Label  string `json:"label,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"`
}

type MCQOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type DiagramSpec struct {
	Type   string         `json:"type"` // number_line | bar_model | fraction_bar | rectangle_grid | clock
	Params map[string]any `json:"params"`
}

type SolutionStep struct {
	Text    string       `json:"text"`
	Diagram *DiagramSpec `json:"diagram,omitempty"`
}

// RenderedQuestion is the client-safe DTO sent to React.
// Answer keys and worked solutions are NEVER attached until submission or explicit skip.
type RenderedQuestion struct {
	ID              string       `json:"id"`
	TemplateCode    string       `json:"templateCode"`
	Seed            int64        `json:"seed"`
	Grade           int          `json:"grade"`
	SkillCode       string       `json:"skillCode"`
	SkillName       string       `json:"skillName"`
	CurriculumCode  string       `json:"curriculumCode"`
	Difficulty      int          `json:"difficulty"`
	HeuristicCode   string       `json:"heuristicCode,omitempty"`
	HeuristicName   string       `json:"heuristicName,omitempty"`
	InputType       InputType    `json:"inputType"`
	Locale          string       `json:"locale"`
	Stem            string       `json:"stem"`
	TTSText         string       `json:"ttsText"`
	Hints           []string     `json:"hints"`
	Parts           []InputPart  `json:"parts,omitempty"`
	Options         []MCQOption  `json:"options,omitempty"`
	Diagram         *DiagramSpec `json:"diagram,omitempty"`
	VideoTutorialID string       `json:"videoTutorialId,omitempty"`
}

// WorkedSolution is returned ONLY after grading or an explicit Skip confirmation.
type WorkedSolution struct {
	CorrectAnswers map[string]string `json:"correctAnswers"`
	Steps          []SolutionStep    `json:"steps"`
}

// GeneratedProblem is the server-internal complete artifact (client payload + grading key).
type GeneratedProblem struct {
	Question        RenderedQuestion
	Solution        WorkedSolution
	GradingStrategy string // exact_numeric | fraction_equiv | text_exact | mcq
	Tolerance       float64
	OrderIndep      bool
}

type GradeResult struct {
	IsCorrect      bool              `json:"isCorrect"`
	CorrectAnswers map[string]string `json:"correctAnswers"`
	WorkedSolution WorkedSolution    `json:"workedSolution"`
}
