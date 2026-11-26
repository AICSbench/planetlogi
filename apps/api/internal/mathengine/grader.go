package mathengine

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// Grade evaluates student answers authoritatively on the server.
// Handles English (3.5) and Indonesian (3,5) decimal separators, whitespace,
// equivalent fractions (e.g., 2/4 == 1/2), and multi-part boxes cleanly.
func Grade(problem GeneratedProblem, submitted map[string]string) GradeResult {
	expected := problem.Solution.CorrectAnswers
	if len(expected) == 0 {
		return GradeResult{
			IsCorrect:      false,
			CorrectAnswers: expected,
			WorkedSolution: problem.Solution,
		}
	}

	if problem.OrderIndep && len(expected) > 1 {
		expVals := make([]string, 0, len(expected))
		subVals := make([]string, 0, len(expected))
		for k, v := range expected {
			expVals = append(expVals, normalizeToken(v))
			subVals = append(subVals, normalizeToken(submitted[k]))
		}
		sort.Strings(expVals)
		sort.Strings(subVals)
		ok := true
		for i := range expVals {
			if !valuesEqual(problem.GradingStrategy, expVals[i], subVals[i], problem.Tolerance) {
				ok = false
				break
			}
		}
		return GradeResult{
			IsCorrect:      ok,
			CorrectAnswers: expected,
			WorkedSolution: problem.Solution,
		}
	}

	allCorrect := true
	for key, expVal := range expected {
		subVal := submitted[key]
		if !valuesEqual(problem.GradingStrategy, expVal, subVal, problem.Tolerance) {
			allCorrect = false
			break
		}
	}

	return GradeResult{
		IsCorrect:      allCorrect,
		CorrectAnswers: expected,
		WorkedSolution: problem.Solution,
	}
}

func valuesEqual(strategy, expected, submitted string, tolerance float64) bool {
	expClean := normalizeToken(expected)
	subClean := normalizeToken(submitted)
	if subClean == "" {
		return false
	}

	switch strategy {
	case "fraction_equiv":
		return fractionsEqual(expClean, subClean)
	case "text_exact", "mcq":
		return strings.EqualFold(expClean, subClean)
	default: // exact_numeric
		expNum, err1 := parseLocaleNumber(expClean)
		subNum, err2 := parseLocaleNumber(subClean)
		if err1 != nil || err2 != nil {
			return strings.EqualFold(expClean, subClean)
		}
		tol := tolerance
		if tol <= 0 {
			tol = 1e-9
		}
		return math.Abs(expNum-subNum) <= tol
	}
}

func normalizeToken(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
}

// parseLocaleNumber accepts both dot (3.5) and comma (3,5) decimals safely.
func parseLocaleNumber(raw string) (float64, error) {
	s := normalizeToken(raw)
	if s == "" {
		return 0, fmt.Errorf("empty numeric input")
	}
	hasDot := strings.Contains(s, ".")
	hasComma := strings.Contains(s, ",")
	if hasDot && hasComma {
		return 0, fmt.Errorf("ambiguous separators")
	}
	if hasComma {
		s = strings.ReplaceAll(s, ",", ".")
	}
	return strconv.ParseFloat(s, 64)
}

func fractionsEqual(expected, submitted string) bool {
	r1, ok1 := parseRat(expected)
	r2, ok2 := parseRat(submitted)
	if !ok1 || !ok2 {
		return false
	}
	return r1.Cmp(r2) == 0
}

// parseRat parses integers ("3"), simple fractions ("2/4"), decimals ("0.5" / "0,5"),
// or mixed numbers ("1_1/2" or "1+1/2").
func parseRat(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	if strings.Contains(s, "_") {
		parts := strings.SplitN(s, "_", 2)
		whole, okW := new(big.Rat).SetString(parts[0])
		frac, okF := new(big.Rat).SetString(parts[1])
		if !okW || !okF {
			return nil, false
		}
		return new(big.Rat).Add(whole, frac), true
	}
	if !strings.Contains(s, "/") && strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}
