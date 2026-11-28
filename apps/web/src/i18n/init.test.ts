import { describe, expect, it } from "vitest";
import i18n, { setAppLocale } from "./init";

describe("web i18n initialization", () => {
  it("loads both en and id namespaces and switches cleanly", async () => {
    await setAppLocale("en");
    expect(i18n.language).toBe("en");
    expect(i18n.t("common:brand")).toBe("PlanetLogi");
    expect(i18n.t("auth:submit")).toContain("PlanetLogi");

    await setAppLocale("id");
    expect(i18n.language).toBe("id");
    expect(i18n.t("common:nav.home")).toBe("Beranda");
    expect(i18n.t("dashboard:pillars.challenges")).toBe("Tantangan Bintang");
  });
});

describe("Phase 3 translations", () => {
  it.each([
    ["en", "Found 3 / 5", "Duel Question 2 of 5"],
    ["id", "Ditemukan 3 / 5", "Soal Duel 2 dari 5"],
  ] as const)("loads and interpolates both new namespaces in %s", async (locale, sight, duel) => {
    await setAppLocale(locale);
    expect(i18n.t("stories:superNova.sightFound", { found: 3 })).toBe(sight);
    expect(i18n.t("social:duels.questionProgress", { current: 2, total: 5 })).toBe(duel);
  });
});
