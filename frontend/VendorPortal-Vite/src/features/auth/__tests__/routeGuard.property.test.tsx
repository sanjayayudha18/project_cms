/**
 * Property-based test: Authentication Route Guard Round-Trip
 *
 * Feature: vendor-portal, Property 12: Authentication Route Guard Round-Trip
 *
 * For any protected route path, when an unauthenticated user attempts to access it:
 * the system should redirect to /login?redirect={path} AND preserve the original path.
 * After successful authentication, the system should redirect to that preserved path
 * (not the default /dashboard). The round-trip should restore the user's intended destination.
 *
 * Validates: Requirements 10, 12
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import * as fc from 'fast-check';
import { render, screen, act } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation, useSearchParams } from 'react-router';
import { AuthProvider } from '../AuthContext';
import { useAuth } from '../useAuth';
import { ProtectedRoute, GuestRoute } from '../ProtectedRoute';

// ─── Mock fetch ───────────────────────────────────────────────────────────────

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch);
  // Default: refresh on mount returns 401 (not authenticated)
  mockFetch.mockResolvedValue(
    new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 }),
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

/**
 * Helper component that captures and displays the current location
 * so we can assert on redirects.
 */
function LocationDisplay() {
  const location = useLocation();
  const [searchParams] = useSearchParams();
  return (
    <div>
      <span data-testid="pathname">{location.pathname}</span>
      <span data-testid="search">{location.search}</span>
      <span data-testid="redirect-param">{searchParams.get('redirect') ?? 'none'}</span>
    </div>
  );
}

/**
 * A simulated login page that reads the `redirect` query param.
 */
function MockLoginPage() {
  const { login } = useAuth();
  const [searchParams] = useSearchParams();
  const redirectTo = searchParams.get('redirect') ?? '/dashboard';

  return (
    <div>
      <span data-testid="login-page">login</span>
      <span data-testid="redirect-target">{redirectTo}</span>
      <button
        data-testid="do-login"
        onClick={async () => {
          try {
            await login('vendor.user', 'password123');
          } catch {
            // ignore
          }
        }}
      >
        Login
      </button>
    </div>
  );
}

/**
 * Protected page placeholder.
 */
function ProtectedPage() {
  return <span data-testid="protected-page">protected</span>;
}

/**
 * Generator for valid protected paths in the vendor portal.
 */
const protectedPathArb = fc.oneof(
  fc.constant('/orders'),
  fc.constant('/invoices'),
  fc.constant('/schedule'),
  fc.constant('/dsr'),
  fc.constant('/notifications'),
  // Dynamic route: /orders/:id/evidence
  fc.array(fc.constantFrom('a', 'b', 'c', '1', '2', '3', '-'), { minLength: 1, maxLength: 8 })
    .map((chars) => `/orders/${chars.join('')}/evidence`),
);

describe('Feature: vendor-portal, Property 12: Authentication Route Guard Round-Trip', () => {
  it('unauthenticated access to any protected path redirects to /login?redirect={path}', async () => {
    await fc.assert(
      fc.asyncProperty(protectedPathArb, async (path) => {
        const { unmount } = render(
          <MemoryRouter initialEntries={[path]}>
            <AuthProvider>
              <Routes>
                <Route element={<GuestRoute />}>
                  <Route path="/login" element={<><MockLoginPage /><LocationDisplay /></>} />
                </Route>
                <Route element={<ProtectedRoute />}>
                  <Route path="*" element={<ProtectedPage />} />
                </Route>
              </Routes>
            </AuthProvider>
          </MemoryRouter>,
        );

        // Wait for auth initialization to complete
        await act(async () => {
          await new Promise((r) => setTimeout(r, 10));
        });

        // Should be redirected to /login
        const pathnames = screen.getAllByTestId('pathname');
        const loginPathname = pathnames.find((el) => el.textContent === '/login');
        expect(loginPathname).toBeDefined();

        // Should preserve the original path in redirect query param
        const redirectParams = screen.getAllByTestId('redirect-param');
        const matchingParam = redirectParams.find((el) => el.textContent === path);
        expect(matchingParam).toBeDefined();

        unmount();
      }),
      { numRuns: 50 },
    );
  });

  it('post-login redirect target is the preserved path (not /dashboard default)', async () => {
    await fc.assert(
      fc.asyncProperty(
        protectedPathArb.filter((p) => p !== '/dashboard'),
        async (path) => {
          const { unmount } = render(
            <MemoryRouter initialEntries={[path]}>
              <AuthProvider>
                <Routes>
                  <Route element={<GuestRoute />}>
                    <Route path="/login" element={<MockLoginPage />} />
                  </Route>
                  <Route element={<ProtectedRoute />}>
                    <Route path="*" element={<ProtectedPage />} />
                  </Route>
                </Routes>
              </AuthProvider>
            </MemoryRouter>,
          );

          // Wait for auth initialization
          await act(async () => {
            await new Promise((r) => setTimeout(r, 10));
          });

          // After redirect to login, the redirect target should contain the original path
          expect(screen.getByTestId('login-page')).toBeInTheDocument();
          const redirectTarget = screen.getByTestId('redirect-target');
          expect(redirectTarget.textContent).toBe(path);
          expect(redirectTarget.textContent).not.toBe('/dashboard');

          unmount();
        },
      ),
      { numRuns: 50 },
    );
  });
});
