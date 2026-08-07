import { useCallback, useContext } from 'react';
import { AuthContext, type AuthContextValue } from './AuthContext';

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);

  if (context === null) {
    throw new Error(
      'useAuth must be used within an AuthProvider. ' +
        'Wrap your component tree with <AuthProvider>.',
    );
  }

  return context;
}

/**
 * Hook that provides a function to handle 401 responses from API calls.
 * Attempts one token refresh; on failure, clears state and redirects to login
 * preserving the current URL for post-login redirect.
 */
export function useAuthRefresh(): {
  handleUnauthorized: () => Promise<boolean>;
} {
  const { refreshToken, logout } = useAuth();

  const handleUnauthorized = useCallback(async (): Promise<boolean> => {
    const refreshed = await refreshToken();

    if (!refreshed) {
      const currentPath = window.location.pathname + window.location.search;
      await logout();
      window.location.href = `/login?redirect=${encodeURIComponent(currentPath)}`;
      return false;
    }

    return true;
  }, [refreshToken, logout]);

  return { handleUnauthorized };
}
