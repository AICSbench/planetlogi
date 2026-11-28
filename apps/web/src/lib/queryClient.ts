import { QueryClient } from "@tanstack/react-query";

// Shared so the auth store can drop every account-scoped cache entry when the signed-in
// account changes (logout, or a different account signing in on the same device).
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});
