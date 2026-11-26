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
