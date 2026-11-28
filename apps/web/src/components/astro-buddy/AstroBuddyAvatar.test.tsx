// @vitest-environment jsdom

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AstroBuddyAvatar } from "./AstroBuddyAvatar";

describe("AstroBuddyAvatar", () => {
  it("renders an accessible image when profile customization has not loaded", () => {
    const contents = document.createElement("div");
    contents.innerHTML = renderToStaticMarkup(<AstroBuddyAvatar config={{}} />);

    const avatar = contents.querySelector("svg[role='img']");
    expect(avatar?.getAttribute("aria-label")).toBe("Astro Buddy");
    expect(avatar?.getAttribute("width")).toBe("96");
    expect(avatar?.getAttribute("height")).toBe("96");
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
