import { afterEach, describe, expect, it, vi } from "vitest";
import { createApiClient } from "./index";

const baseUrl = "https://planetlogi.example";

afterEach(() => vi.unstubAllGlobals());

describe("Phase 3 API client", () => {
  it("sends a story bookmark as a credentialed PATCH with the chapter code in one path segment", async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ ok: true }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient(baseUrl + "/");
    await expect(client.bookmarkStoryPage("terra/pebble?draft", 20)).resolves.toEqual({ ok: true });

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      baseUrl + "/api/v1/stories/chapters/terra%2Fpebble%3Fdraft/bookmark",
      {
        credentials: "include",
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ page: 20 }),
      },
    );
  });

  it("does not send a client-selected opponent for random matchmaking", async () => {
    const response = { duel: { id: "duel-test", status: "IN_PROGRESS" }, energyLeft: 0 };
    const fetchMock = vi.fn().mockResolvedValue(Response.json(response));
    vi.stubGlobal("fetch", fetchMock);

    await expect(createApiClient(baseUrl).startDuel("RANDOM", undefined, "id")).resolves.toEqual(response);
    expect(fetchMock).toHaveBeenCalledWith(
      baseUrl + "/api/v1/duels/start?locale=id",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: JSON.stringify({ mode: "RANDOM" }),
      }),
    );
  });

  it("preserves the server's retryable reward error on Super Warp submission", async () => {
    const detail = "reward temporarily unavailable";
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ detail }, { status: 503 }));
    vi.stubGlobal("fetch", fetchMock);

    const answers = { 1: { answer: "12" }, 3: { numerator: "2", denominator: "3" } };
    await expect(createApiClient(baseUrl).finishSuperWarp("warp-test", answers, "id")).rejects.toMatchObject({
      status: 503,
      detail,
    });
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      baseUrl + "/api/v1/super-nova/super-warp/finish?locale=id",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: JSON.stringify({ sessionId: "warp-test", answers }),
      }),
    );
  });
});
