import { describe, expect, it } from "vitest";
import { GRADE_PLANETS, ProficiencyPill, getPlanetForGrade } from "./index";

describe("@planetlogi/ui galaxy themes", () => {
  it("defines 6 elementary grade planets with bilingual names", () => {
    expect(GRADE_PLANETS).toHaveLength(6);
    for (const planet of GRADE_PLANETS) {
      expect(planet.name.en.length).toBeGreaterThan(0);
      expect(planet.name.id.length).toBeGreaterThan(0);
    }
  });

  it("clamps grade lookups safely to Grades 1-6", () => {
    expect(getPlanetForGrade(1).code).toBe("terra_pebble");
    expect(getPlanetForGrade(6).code).toBe("solaris");
    expect(getPlanetForGrade(99).code).toBe("solaris");
    expect(getPlanetForGrade(0).code).toBe("terra_pebble");
  });

  it("renders INCOMPLETE proficiency attempt progress when provided", () => {
    const element = ProficiencyPill({
      tier: "INCOMPLETE",
      label: "Incomplete",
      attemptedCount: 4,
    });
    const serialized = JSON.stringify(element);
    expect(serialized).toContain("Incomplete");
    expect(serialized).toContain("4");
    expect(serialized).toContain("10");
  });
});
