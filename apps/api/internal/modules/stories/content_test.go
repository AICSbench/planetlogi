package stories

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoryPackHasBilingualOpeningForEveryGrade(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "content", "stories", "g1-g6-planet-stories.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pack struct {
		Planets []struct {
			Grade    int
			Code     string
			Chapters []struct {
				Index, UnlockCostAc int
				Pages               []struct {
					Page       int
					SceneTheme string
					Text       map[string]string
				}
			}
		}
	}
	if err := json.Unmarshal(raw, &pack); err != nil {
		t.Fatal(err)
	}
	if len(pack.Planets) != 6 {
		t.Fatalf("found %d planets; want 6", len(pack.Planets))
	}
	grades := make(map[int]bool)
	for _, planet := range pack.Planets {
		t.Run(planet.Code, func(t *testing.T) {
			if planet.Grade < 1 || planet.Grade > 6 || grades[planet.Grade] {
				t.Errorf("invalid or duplicate grade %d", planet.Grade)
			}
			grades[planet.Grade] = true
			openings := 0
			for _, chapter := range planet.Chapters {
				if chapter.Index == 0 {
					openings++
					if chapter.UnlockCostAc != 0 || len(chapter.Pages) != 20 {
						t.Errorf("opening costs %d AC and has %d pages; want 0 AC and 20 pages", chapter.UnlockCostAc, len(chapter.Pages))
					}
				} else if chapter.UnlockCostAc != 20 {
					t.Errorf("chapter %d costs %d AC; want 20", chapter.Index, chapter.UnlockCostAc)
				}
				for i, page := range chapter.Pages {
					if page.Page != i+1 || strings.TrimSpace(page.SceneTheme) == "" {
						t.Errorf("chapter %d page %d has invalid page number or missing scene", chapter.Index, i+1)
					}
					for _, locale := range []string{"en", "id"} {
						if strings.TrimSpace(page.Text[locale]) == "" {
							t.Errorf("chapter %d page %d is missing %s text", chapter.Index, i+1, locale)
						}
					}
				}
			}
			if openings != 1 {
				t.Errorf("found %d opening chapters; want 1", openings)
			}
		})
	}
}
