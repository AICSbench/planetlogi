// @vitest-environment jsdom

import { renderToStaticMarkup } from "react-dom/server";
import { resources } from "@planetlogi/i18n";
import { createInstance } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { describe, expect, it } from "vitest";
import { AstroBuddyAvatar } from "./AstroBuddyAvatar";

describe("AstroBuddyAvatar", () => {
  it.each(["en", "id"] as const)("localizes missing names and preserves custom names in %s", async (locale) => {
    const i18n = createInstance();
    await i18n.use(initReactI18next).init({
      resources: structuredClone(resources),
      lng: locale,
      fallbackLng: "en",
      ns: ["stories"],
      defaultNS: "stories",
    });
    const renderName = (fullName?: string) => {
      const contents = document.createElement("div");
      contents.innerHTML = renderToStaticMarkup(
        <I18nextProvider i18n={i18n}>
          <AstroBuddyAvatar config={{ fullName }} />
        </I18nextProvider>
      );
      return contents.querySelector("svg[role='img']")?.getAttribute("aria-label");
    };

    expect(renderName()).toBe("Astro Buddy");
    expect(renderName("")).toBe("Astro Buddy");
    expect(renderName("  ")).toBe("Astro Buddy");
    expect(renderName("Starry Pilot")).toBe("Starry Pilot");

    // Both locales currently use the same brand; prove the value comes from i18n.
    i18n.addResource(locale, "stories", "customization.buddyFallbackName", "Translated fallback");
    expect(renderName()).toBe("Translated fallback");
    expect(renderName("Starry Pilot")).toBe("Starry Pilot");
  });

  it("renders the saved body and color at the size requested by its caller", () => {
    const contents = document.createElement("div");
    contents.innerHTML = renderToStaticMarkup(
      <AstroBuddyAvatar
        config={{ bodyShape: "cube", colorHex: "#22D3EE", fullName: "Starry Pilot" }}
        size={48}
      />
    );

    const avatar = contents.querySelector("svg[aria-label='Starry Pilot']");
    expect(avatar?.getAttribute("width")).toBe("48");
    expect(avatar?.getAttribute("height")).toBe("48");
    const gradient = avatar?.querySelector("radialGradient");
    expect(gradient?.querySelector("stop[offset='100%']")?.getAttribute("stop-color")).toBe("#22D3EE");
    expect(avatar?.querySelector("rect")?.getAttribute("fill")).toBe(`url(#${gradient?.id})`);
  });
});
