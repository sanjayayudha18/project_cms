import { render, screen, act, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AuthProvider } from '../AuthContext';
import { useAuth } from '../useAuth';

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

// ─── Test Consumer ────────────────────────────────────────────────────────────

function TestConsumer() {
  const { state, login, logout } = useAuth();
  return (
    <div>
      <span data-testid="authenticated">{String(state.isAuthenticated)}</span>
      <span data-testid="loading">{String(state.isAuthLoading)}</span>
      <span data-testid="user">{state.user?.fullName ?? 'none'}</span>
      <span data-testid="error">{state.error ?? 'none'}</span>
      <span data-testid="retry-after">
        {state.rateLimitRetryAfter !== null ? String(state.rateLimitRetryAfter) : 'none'}
      </span>
      <button
        onClick={() => login('vendor.user', 'password123').catch(() => {})}
        data-testid="login-valid"
      >
        Login
      </button>
      <button
        onClick={() => login('wrong', 'wrong').catch(() => {})}
        data-testid="login-invalid"
      >
        Login Invalid
      </button>
      <button
        onClick={() => { void logout(); }}
        data-testid="logout"
      >
        Logout
      </button>
    </div>
  );
}

// ─── Helper ───────────────────────────────────────────────────────────────────

function mockLoginSuccess() {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes('/refresh')) {
      return new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 });
    }
    if (url.includes('/login')) {
      return new Response(
        JSON.stringify({
          access_token: 'mock-access-token-abc',
          user: {
            id: 1,
            username: 'vendor.user',
            full_name: 'Budi Santoso',
            email: 'budi@vendor.com',
            role: 'VENDOR-USER',
            is_karyawan: false,
            vendor_id: 42,
          },
        }),
        { status: 200 },
      );
    }
    return new Response(null, { status: 404 });
  });
}

function mockLoginUnauthorized() {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes('/refresh')) {
      return new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 });
    }
    if (url.includes('/login')) {
      return new Response(
        JSON.stringify({ error: 'invalid_credentials', message: 'Invalid credentials' }),
        { status: 401 },
      );
    }
    return new Response(null, { status: 404 });
  });
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('AuthContext', () => {
  it('starts with loading state then resolves to unauthenticated', async () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    // Wait for initialization to finish
    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('user')).toHaveTextContent('none');
  });

  it('login with valid credentials sets authenticated state', async () => {
    mockLoginSuccess();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    // Wait for init
    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    await act(async () => {
      screen.getByTestId('login-valid').click();
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('user')).toHaveTextContent('Budi Santoso');
    expect(screen.getByTestId('error')).toHaveTextContent('none');
  });

  it('login with invalid credentials sets error', async () => {
    mockLoginUnauthorized();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    await act(async () => {
      screen.getByTestId('login-invalid').click();
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('error')).toHaveTextContent('Username atau password salah');
  });

  it('login sends X-Portal-Type: vendor header', async () => {
    mockLoginSuccess();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    await act(async () => {
      screen.getByTestId('login-valid').click();
    });

    // Find the login call
    const loginCall = mockFetch.mock.calls.find(
      (call) => typeof call[0] === 'string' && call[0].includes('/login'),
    );
    expect(loginCall).toBeDefined();
    expect(loginCall![1].headers['X-Portal-Type']).toBe('vendor');
  });

  it('logout clears auth state', async () => {
    mockLoginSuccess();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    // Login first
    await act(async () => {
      screen.getByTestId('login-valid').click();
    });
    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');

    // Logout
    await act(async () => {
      screen.getByTestId('logout').click();
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('user')).toHaveTextContent('none');
  });

  it('handles rate limit (429) with Retry-After', async () => {
    mockFetch.mockImplementation(async (url: string) => {
      if (url.includes('/refresh')) {
        return new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 });
      }
      if (url.includes('/login')) {
        return new Response(
          JSON.stringify({ error: 'rate_limited', message: 'Too many attempts' }),
          {
            status: 429,
            headers: { 'Retry-After': '120' },
          },
        );
      }
      return new Response(null, { status: 404 });
    });

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    await act(async () => {
      screen.getByTestId('login-valid').click();
    });

    expect(screen.getByTestId('error')).toHaveTextContent('Terlalu banyak percobaan login');
    expect(screen.getByTestId('retry-after')).toHaveTextContent('120');
  });

  it('handles portal mismatch (403)', async () => {
    mockFetch.mockImplementation(async (url: string) => {
      if (url.includes('/refresh')) {
        return new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 });
      }
      if (url.includes('/login')) {
        return new Response(
          JSON.stringify({ error: 'portal_mismatch', message: 'Portal mismatch' }),
          { status: 403 },
        );
      }
      return new Response(null, { status: 404 });
    });

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    await act(async () => {
      screen.getByTestId('login-valid').click();
    });

    expect(screen.getByTestId('error')).toHaveTextContent('Akun tidak memiliki akses ke portal ini');
  });

  it('initializes as authenticated when refresh succeeds', async () => {
    mockFetch.mockResolvedValue(
      new Response(
        JSON.stringify({
          access_token: 'refreshed-token',
          user: {
            id: 1,
            username: 'vendor.user',
            full_name: 'Budi Santoso',
            email: 'budi@vendor.com',
            role: 'VENDOR-USER',
            is_karyawan: false,
            vendor_id: 42,
          },
        }),
        { status: 200 },
      ),
    );

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('loading')).toHaveTextContent('false');
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('user')).toHaveTextContent('Budi Santoso');
  });
});

describe('useAuth', () => {
  it('throws error when used outside AuthProvider', () => {
    function BadConsumer() {
      useAuth();
      return null;
    }

    // Suppress React error boundary console error
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => render(<BadConsumer />)).toThrow(
      'useAuth must be used within an AuthProvider',
    );
    consoleSpy.mockRestore();
  });
});
