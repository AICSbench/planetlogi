import { create } from "zustand";
import { ApiError, createApiClient, type Principal } from "@planetlogi/api-client";
import { queryClient } from "../lib/queryClient";

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

// Cached queries (profile bundle, parent overview, attempts, ...) are not keyed by account, so
// they must be dropped whenever the signed-in account changes to avoid showing one account's
// data to the next person on a shared device.
function clearCacheIfAccountChanged(prev: Principal | null, next: Principal | null) {
  if (prev?.id !== next?.id) {
    queryClient.clear();
  }
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  hydrated: false,
  setUser: (user) => {
    sessionGeneration += 1;
    clearCacheIfAccountChanged(get().user, user);
    set({ user, hydrated: true });
  },
  hydrate: async () => {
    const gen = ++sessionGeneration;
    try {
      const res = await apiClient.me();
      if (gen === sessionGeneration) {
        clearCacheIfAccountChanged(get().user, res.user);
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
          clearCacheIfAccountChanged(get().user, refreshed.user);
          set({ user: refreshed.user, hydrated: true });
        }
      } catch (refreshErr) {
        if (gen !== sessionGeneration) return;
        if (isUnauthenticatedError(refreshErr)) {
          clearCacheIfAccountChanged(get().user, null);
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
    queryClient.clear();
    if (gen === sessionGeneration) {
      set({ user: null, hydrated: true });
    }
  },
}));
