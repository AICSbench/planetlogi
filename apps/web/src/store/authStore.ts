import { create } from "zustand";
import { ApiError, createApiClient, type Principal } from "@planetlogi/api-client";
import { queryClient } from "./queryClient";

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

// Cached queries hold one account's data. Drop them whenever the signed-in identity changes,
// on every path that can change it (login, hydration, refresh, logout).
function clearIfAccountChanged(previous: Principal | null, next: Principal | null) {
  if ((previous?.id ?? null) !== (next?.id ?? null)) {
    queryClient.clear();
  }
}

/** Identity segment for query keys of account-scoped data, so late responses for a previous
 * account can never be read by the next one. */
export function accountScope(user: Principal | null): string {
  return user?.id ?? "guest";
}

export function useAccountScope(): string {
  return useAuthStore((s) => accountScope(s.user));
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  hydrated: false,
  setUser: (user) => {
    sessionGeneration += 1;
    clearIfAccountChanged(get().user, user);
    set({ user, hydrated: true });
  },
  hydrate: async () => {
    const gen = ++sessionGeneration;
    try {
      const res = await apiClient.me();
      if (gen === sessionGeneration) {
        clearIfAccountChanged(get().user, res.user);
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
          clearIfAccountChanged(get().user, refreshed.user);
          set({ user: refreshed.user, hydrated: true });
        }
      } catch (refreshErr) {
        if (gen !== sessionGeneration) return;
        if (isUnauthenticatedError(refreshErr)) {
          clearIfAccountChanged(get().user, null);
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
      queryClient.clear();
      set({ user: null, hydrated: true });
    }
  },
}));
