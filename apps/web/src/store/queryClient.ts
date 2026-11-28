import { QueryClient } from "@tanstack/react-query";

// Shared so session changes can drop account-scoped cached data.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});
