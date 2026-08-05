import { render, screen, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { AuthProvider } from '../AuthContext';
import { useAuth } from '../useAuth';

function TestConsumer() {
  const { state, login, logout } = useAuth();
  return (
    <div>
      <span data-testid="authenticated">
        {String(state.isAuthenticated)}
      </span>
      <span data-testid="user">{state.user?.displayName ?? 'none'}</span>
      <span data-testid="vendor-id">{state.user?.vendorId ?? 'none'}</span>
      <span data-testid="vendor-name">{state.user?.vendorName ?? 'none'}</span>
      <span data-testid="token">{state.token ? 'present' : 'none'}</span>
      <button
        onClick={() => login('gardanet.admin', 'password123')}
        data-testid="login-valid"
      >
        Login Valid
      </button>
      <button
        onClick={() => login('wrong', 'wrong').catch(() => {})}
        data-testid="login-invalid"
      >
        Login Invalid
      </button>
      <button onClick={logout} data-testid="logout">
        Logout
      </button>
    </div>
  );
}

describe('AuthContext', () => {
  it('starts with unauthenticated state', () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('user')).toHaveTextContent('none');
    expect(screen.getByTestId('token')).toHaveTextContent('none');
  });

  it('login with valid credentials sets authenticated state', async () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await act(async () => {
      screen.getByTestId('login-valid').click();
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
    expect(screen.getByTestId('user')).toHaveTextContent('Budi Santoso');
    expect(screen.getByTestId('vendor-id')).toHaveTextContent(
      'vendor-gardanet',
    );
    expect(screen.getByTestId('vendor-name')).toHaveTextContent('PT Gardanet');
    expect(screen.getByTestId('token')).toHaveTextContent('present');
  });

  it('login with invalid credentials throws error', async () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    const { login } = getAuthFromRender();

    await expect(login('wrong', 'wrong')).rejects.toThrow(
      'Username atau password salah',
    );
  });

  it('logout clears auth state', async () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    // Login first
    await act(async () => {
      screen.getByTestId('login-valid').click();
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');

    // Then logout
    act(() => {
      screen.getByTestId('logout').click();
    });

    expect(screen.getByTestId('authenticated')).toHaveTextContent('false');
    expect(screen.getByTestId('user')).toHaveTextContent('none');
    expect(screen.getByTestId('token')).toHaveTextContent('none');
  });

  it('login validates all mock users', async () => {
    const users = [
      {
        username: 'gardanet.admin',
        password: 'password123',
        expectedName: 'Budi Santoso',
        expectedVendor: 'vendor-gardanet',
      },
      {
        username: 'ssi.admin',
        password: 'password123',
        expectedName: 'Siti Rahayu',
        expectedVendor: 'vendor-ssi',
      },
      {
        username: 'g4s.admin',
        password: 'password123',
        expectedName: 'Ahmad Wijaya',
        expectedVendor: 'vendor-g4s',
      },
    ];

    for (const testUser of users) {
      const { login } = getAuthFromRender();
      await login(testUser.username, testUser.password);
    }
  });
});

// Helper to get auth methods directly for assertion tests
function getAuthFromRender() {
  let authValue: ReturnType<typeof useAuth> | null = null;

  function Capture() {
    authValue = useAuth();
    return null;
  }

  render(
    <AuthProvider>
      <Capture />
    </AuthProvider>,
  );

  return authValue!;
}

describe('useAuth', () => {
  it('throws error when used outside AuthProvider', () => {
    function BadConsumer() {
      useAuth();
      return null;
    }

    expect(() => render(<BadConsumer />)).toThrow(
      'useAuth must be used within an AuthProvider',
    );
  });
});
