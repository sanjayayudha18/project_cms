import { ToastContainer } from "@/components/ui/Toast";
import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Outlet, createRootRoute, useRouter } from "@tanstack/react-router";
import { useEffect } from "react";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // 5 minutes
      retry: 1,
    },
  },
});

export const rootRoute = createRootRoute({
  component: RootComponent,
});

function RootComponent() {
  const router = useRouter();
  const initialize = useAuthStore((s) => s.initialize);
  const isAuthLoading = useAuthStore((s) => s.isAuthLoading);

  useEffect(() => {
    // Guards ran with a null user during the initial load and skipped themselves; re-run them
    // now that the session is restored so role checks and the requested path are honoured.
    initialize().then(() => router.invalidate());
  }, [initialize, router]);

  // Show a loading indicator while checking auth state
  if (isAuthLoading) {
    return (
      <div
        className="flex min-h-screen items-center justify-center"
        style={{ backgroundColor: "var(--n-50)" }}
      >
        <div
          className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
          style={{ borderColor: "var(--n-200)", borderTopColor: "var(--red-500)" }}
          aria-label="Memuat..."
        />
      </div>
    );
  }

  return (
    <QueryClientProvider client={queryClient}>
      <Outlet />
      <ToastContainer />
    </QueryClientProvider>
  );
}
