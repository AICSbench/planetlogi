import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, createApiClient } from "./index";

const base = "https://planetlogi.example.test";
const client = createApiClient(`${base}/`);

afterEach(() => vi.unstubAllGlobals());

describe("Phase 3 API client", () => {
  it.each([
    {
      name: "bookmark a chapter whose code needs escaping",
      call: () => client.bookmarkStoryPage("planet/chapter #1", 20),
      path: "/api/v1/stories/chapters/planet%2Fchapter%20%231/bookmark",
      method: "PATCH",
      body: { page: 20 },
    },
    {
      name: "submit normalized Super Sight coordinates, including zero",
      call: () => client.tapSuperSight("sight-1", 0, 0.75, "id"),
      path: "/api/v1/super-nova/super-sight/tap?locale=id",
      method: "POST",
      body: { sessionId: "sight-1", x: 0, y: 0.75 },
    },
    {
      name: "submit indexed duel answers without changing their values",
      call: () => client.submitDuel("duel/01", { 0: { answer: "0" }, 4: { answer: "1,5" } }, "id"),
      path: "/api/v1/duels/duel%2F01/submit?locale=id",
      method: "POST",
      body: { answers: { 0: { answer: "0" }, 4: { answer: "1,5" } } },
    },
    {
      name: "recover a badge by calendar date",
      call: () => client.recoverDailyBadge("2026-11-30", "id"),
      path: "/api/v1/badges/recover?locale=id",
      method: "POST",
      body: { date: "2026-11-30" },
    },
  ])("$name", async ({ call, path, method, body }) => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(Response.json({}));
    vi.stubGlobal("fetch", fetchMock);

    await call();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, request] = fetchMock.mock.calls[0]!;
    expect(url).toBe(`${base}${path}`);
    expect(request).toMatchObject({ method, credentials: "include" });
    expect(JSON.parse(String(request?.body))).toEqual(body);
  });

  it("propagates insufficient funds when a story unlock is denied", async () => {
    const detail = "insufficient Astro Coins (20 AC required)";
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(Response.json({ detail }, { status: 400 }));
    vi.stubGlobal("fetch", fetchMock);

    const unlock = client.unlockStoryChapter("terra_pebble-ch1", "en");
    await expect(unlock).rejects.toBeInstanceOf(ApiError);
    await expect(unlock).rejects.toMatchObject({ status: 400, detail });
    expect(fetchMock).toHaveBeenCalledWith(
      `${base}/api/v1/stories/chapters/terra_pebble-ch1/unlock?locale=en`,
      expect.objectContaining({ method: "POST", credentials: "include" })
    );
  });
});
