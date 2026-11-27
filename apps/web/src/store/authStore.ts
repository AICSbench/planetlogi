import { create } from "zustand";
import { ApiError, createApiClient, type Principal } from "@planetlogi/api-client";

const API_BASE =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "http://localhost:8080";

export const apiClient = createApiClient(API_BASE);

export function isUnauthenticatedError(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 401 || err.status === 403);
}

interface AuthState {
  user: Principal | null;
  hydrated: boolean;
  setUser: (user: Principal | null) => void;
  hydrate: () => Promise<void>;
  logout: () => Promise<void>;
}

let sessionGeneration = 0;

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  hydrated: false,
  setUser: (user) => {
    sessionGeneration += 1;
    set({ user, hydrated: true });
  },
  hydrate: async () => {
    const gen = ++sessionGeneration;
    try {
      const res = await apiClient.me();
      if (gen === sessionGeneration) {
        set({ user: res.user, hydrated: true });
      }
    } catch (meErr) {
      if (!isUnauthenticatedError(meErr)) {
        if (gen === sessionGeneration && get().user !== null) {
          set({ hydrated: true });
        }
        return;
      }
      try {
        const refreshed = await apiClient.refresh();
        if (gen === sessionGeneration) {
          set({ user: refreshed.user, hydrated: true });
        }
      } catch (refreshErr) {
        if (gen !== sessionGeneration) return;
        if (isUnauthenticatedError(refreshErr)) {
          set({ user: null, hydrated: true });
        } else if (get().user !== null) {
          set({ hydrated: true });
        }
      }
    }
  },
  logout: async () => {
    const gen = ++sessionGeneration;
    try {
      await apiClient.logout();
    } catch {
      // Clear local session even if offline
    }
    if (gen === sessionGeneration) {
      set({ user: null, hydrated: true });
    }
  },
}));
