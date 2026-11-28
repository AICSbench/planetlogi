import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, type Principal } from "@planetlogi/api-client";
import { apiClient, currentSessionGeneration, useAuthStore } from "./authStore";

const demoStudent: Principal = {
  id: "stu-1",
  role: "STUDENT",
  displayName: "Raka",
  grade: 1,
  preferredLocale: "id",
  timezone: "Asia/Jakarta",
};

describe("useAuthStore session resilience", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    useAuthStore.setState({ user: null, hydrated: false });
  });

  it("advances the session generation when the account changes", () => {
    const before = currentSessionGeneration();
    useAuthStore.getState().setUser(demoStudent);
    expect(currentSessionGeneration()).toBeGreaterThan(before);
  });

  it("preserves a known session when me() fails with a transport error", async () => {
    useAuthStore.getState().setUser(demoStudent);
    vi.spyOn(apiClient, "me").mockRejectedValue(new TypeError("Failed to fetch"));

    await useAuthStore.getState().hydrate();

    expect(useAuthStore.getState().user?.id).toBe("stu-1");
    expect(useAuthStore.getState().hydrated).toBe(true);
  });

  it("keeps initial hydration unknown on offline failure, and clears user only on 401/403", async () => {
    vi.spyOn(apiClient, "me").mockRejectedValue(new TypeError("Failed to fetch"));
    await useAuthStore.getState().hydrate();
    expect(useAuthStore.getState().user).toBeNull();
    expect(useAuthStore.getState().hydrated).toBe(false);

    vi.spyOn(apiClient, "me").mockRejectedValue(new ApiError(401, "unauthenticated"));
    vi.spyOn(apiClient, "refresh").mockRejectedValue(new ApiError(401, "expired"));
    await useAuthStore.getState().hydrate();
    expect(useAuthStore.getState().user).toBeNull();
    expect(useAuthStore.getState().hydrated).toBe(true);
  });

  it("does not clear a newer setUser() when an older pending logout settles", async () => {
    let resolveLogout!: () => void;
    vi.spyOn(apiClient, "logout").mockImplementation(
      () =>
        new Promise<{ ok: boolean }>((resolve) => {
          resolveLogout = () => resolve({ ok: true });
        })
    );

    const logoutPromise = useAuthStore.getState().logout();
    useAuthStore.getState().setUser(demoStudent);
    resolveLogout();
    await logoutPromise;

    expect(useAuthStore.getState().user?.id).toBe("stu-1");
  });
});
