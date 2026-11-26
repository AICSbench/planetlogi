import { create } from "zustand";
import { createApiClient, type Principal } from "@planetlogi/api-client";

const API_BASE =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "http://localhost:8080";

export const apiClient = createApiClient(API_BASE);

interface AuthState {
  user: Principal | null;
  hydrated: boolean;
  setUser: (user: Principal | null) => void;
  hydrate: () => Promise<void>;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  hydrated: false,
  setUser: (user) => set({ user, hydrated: true }),
  hydrate: async () => {
    try {
      const res = await apiClient.me();
      set({ user: res.user, hydrated: true });
    } catch {
      set({ user: null, hydrated: true });
    }
  },
  logout: async () => {
    try {
      await apiClient.logout();
    } catch {
      // Clear local session even if offline
    }
    set({ user: null, hydrated: true });
  },
}));
