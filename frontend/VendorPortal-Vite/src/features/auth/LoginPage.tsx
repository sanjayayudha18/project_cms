import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useLocation, useNavigate } from 'react-router';
import { AlertCircle, Loader2 } from 'lucide-react';
import { useAuth } from '@/features/auth/useAuth';

// ─── Types ────────────────────────────────────────────────────────────────────

interface LoginFormData {
  username: string;
  password: string;
}

// ─── Validation Helpers ───────────────────────────────────────────────────────

function isNonWhitespace(value: string): boolean {
  return value.trim().length > 0;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function LoginPage() {
  const { state, login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [isLoading, setIsLoading] = useState(false);
  const [countdown, setCountdown] = useState<number | null>(null);
  const countdownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Parse redirect URL from query params
  const searchParams = new URLSearchParams(location.search);
  const redirectTo = searchParams.get('redirect') ?? '/dashboard';

  // Already authenticated → redirect to dashboard
  useEffect(() => {
    if (state.isAuthenticated && !state.isAuthLoading) {
      void navigate('/dashboard', { replace: true });
    }
  }, [state.isAuthenticated, state.isAuthLoading, navigate]);

  // Rate limit countdown timer
  useEffect(() => {
    if (state.rateLimitRetryAfter && state.rateLimitRetryAfter > 0) {
      setCountdown(state.rateLimitRetryAfter);

      countdownRef.current = setInterval(() => {
        setCountdown((prev) => {
          if (prev === null || prev <= 1) {
            if (countdownRef.current) {
              clearInterval(countdownRef.current);
              countdownRef.current = null;
            }
            return null;
          }
          return prev - 1;
        });
      }, 1000);

      return () => {
        if (countdownRef.current) {
          clearInterval(countdownRef.current);
          countdownRef.current = null;
        }
      };
    }
  }, [state.rateLimitRetryAfter]);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    defaultValues: { username: '', password: '' },
  });

  const onSubmit = async (data: LoginFormData) => {
    setIsLoading(true);

    try {
      await login(data.username, data.password);
      void navigate(redirectTo, { replace: true });
    } catch {
      // Error state is managed by AuthContext
    } finally {
      setIsLoading(false);
    }
  };

  const isRateLimited = countdown !== null && countdown > 0;
  const isSubmitDisabled = isLoading || isRateLimited;

  // Format countdown for display
  function formatCountdown(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    if (m > 0) {
      return `${m} menit ${s} detik`;
    }
    return `${s} detik`;
  }

  // Don't render login form if already authenticated
  if (state.isAuthenticated && !state.isAuthLoading) {
    return null;
  }

  // Show loading while checking auth state
  if (state.isAuthLoading) {
    return (
      <div
        className="flex min-h-svh items-center justify-center"
        style={{ backgroundColor: 'var(--n-50, oklch(0.975 0.004 29))' }}
      >
        <Loader2 className="h-8 w-8 animate-spin" style={{ color: 'var(--red-500, oklch(0.552 0.205 29))' }} />
      </div>
    );
  }

  return (
    <div
      className="flex min-h-svh items-center justify-center p-4"
      style={{ backgroundColor: 'var(--n-50, oklch(0.975 0.004 29))' }}
    >
      <div
        className="w-full max-w-sm rounded-[10px] p-8"
        style={{
          backgroundColor: 'var(--n-0, oklch(0.992 0.003 29))',
          boxShadow: '0 4px 12px oklch(0.25 0.02 29 / 0.08)',
        }}
      >
        {/* CIMB Niaga Branding */}
        <div className="mb-8 text-center">
          <h1
            className="text-2xl font-bold"
            style={{ color: 'var(--red-500, oklch(0.552 0.205 29))' }}
          >
            CIMB Niaga
          </h1>
          <p
            className="mt-1 text-sm"
            style={{ color: 'var(--n-500, oklch(0.560 0.009 29))' }}
          >
            Vendor Portal
          </p>
        </div>

        {/* Error Display — aria-live for screen readers */}
        <div aria-live="polite" aria-atomic="true">
          {state.error && (
            <div
              className="mb-4 flex items-center gap-3 rounded-[6px] px-4 py-3"
              style={{ backgroundColor: 'var(--danger-bg, oklch(0.955 0.035 12))' }}
              role="alert"
            >
              <AlertCircle
                size={16}
                className="shrink-0"
                style={{ color: 'var(--danger-fg, oklch(0.500 0.195 12))' }}
                aria-hidden="true"
              />
              <p
                className="text-sm"
                style={{ color: 'var(--danger-fg, oklch(0.500 0.195 12))' }}
              >
                {state.error}
              </p>
            </div>
          )}

          {isRateLimited && countdown !== null && (
            <div
              className="mb-4 rounded-[6px] px-4 py-3 text-sm"
              style={{
                backgroundColor: 'var(--warning-bg, oklch(0.960 0.055 78))',
                color: 'var(--warning-fg, oklch(0.520 0.115 78))',
              }}
            >
              Coba lagi dalam {formatCountdown(countdown)}
            </div>
          )}
        </div>

        {/* Login Form */}
        <form onSubmit={handleSubmit(onSubmit)} noValidate>
          {/* Username Field */}
          <div className="mb-4">
            <label
              htmlFor="login-username"
              className="mb-1.5 block text-[13px] font-medium"
              style={{ color: 'var(--n-700, oklch(0.352 0.007 29))' }}
            >
              Username
            </label>
            <input
              id="login-username"
              type="text"
              autoComplete="username"
              placeholder="Username"
              maxLength={128}
              disabled={isSubmitDisabled}
              aria-invalid={errors.username ? 'true' : undefined}
              aria-describedby={errors.username ? 'username-error' : undefined}
              className="h-10 w-full rounded-[6px] border px-3 text-sm outline-none transition-shadow"
              style={{
                borderColor: errors.username
                  ? 'var(--danger-fg, oklch(0.500 0.195 12))'
                  : 'var(--n-300, oklch(0.845 0.007 29))',
                backgroundColor: 'var(--n-0, oklch(0.992 0.003 29))',
                color: 'var(--n-800, oklch(0.258 0.006 29))',
              }}
              onFocus={(e) => {
                if (!errors.username) {
                  e.currentTarget.style.borderColor = 'var(--red-400, oklch(0.640 0.185 29))';
                  e.currentTarget.style.boxShadow = '0 0 0 3px var(--red-100, oklch(0.925 0.045 29))';
                }
              }}
              {...register('username', {
                required: 'Username wajib diisi',
                validate: (v) => isNonWhitespace(v) || 'Username wajib diisi',
                maxLength: { value: 128, message: 'Maksimal 128 karakter' },
                onBlur: (e) => {
                  if (!errors.username) {
                    e.target.style.borderColor = 'var(--n-300, oklch(0.845 0.007 29))';
                    e.target.style.boxShadow = 'none';
                  }
                },
              })}
            />
            {errors.username && (
              <p
                id="username-error"
                className="mt-1 text-xs"
                style={{ color: 'var(--danger-fg, oklch(0.500 0.195 12))' }}
              >
                {errors.username.message}
              </p>
            )}
          </div>

          {/* Password Field */}
          <div className="mb-6">
            <label
              htmlFor="login-password"
              className="mb-1.5 block text-[13px] font-medium"
              style={{ color: 'var(--n-700, oklch(0.352 0.007 29))' }}
            >
              Password
            </label>
            <input
              id="login-password"
              type="password"
              autoComplete="current-password"
              placeholder="Password"
              maxLength={72}
              disabled={isSubmitDisabled}
              aria-invalid={errors.password ? 'true' : undefined}
              aria-describedby={errors.password ? 'password-error' : undefined}
              className="h-10 w-full rounded-[6px] border px-3 text-sm outline-none transition-shadow"
              style={{
                borderColor: errors.password
                  ? 'var(--danger-fg, oklch(0.500 0.195 12))'
                  : 'var(--n-300, oklch(0.845 0.007 29))',
                backgroundColor: 'var(--n-0, oklch(0.992 0.003 29))',
                color: 'var(--n-800, oklch(0.258 0.006 29))',
              }}
              onFocus={(e) => {
                if (!errors.password) {
                  e.currentTarget.style.borderColor = 'var(--red-400, oklch(0.640 0.185 29))';
                  e.currentTarget.style.boxShadow = '0 0 0 3px var(--red-100, oklch(0.925 0.045 29))';
                }
              }}
              {...register('password', {
                required: 'Password wajib diisi',
                validate: (v) => isNonWhitespace(v) || 'Password wajib diisi',
                maxLength: { value: 72, message: 'Maksimal 72 karakter' },
                onBlur: (e) => {
                  if (!errors.password) {
                    e.target.style.borderColor = 'var(--n-300, oklch(0.845 0.007 29))';
                    e.target.style.boxShadow = 'none';
                  }
                },
              })}
            />
            {errors.password && (
              <p
                id="password-error"
                className="mt-1 text-xs"
                style={{ color: 'var(--danger-fg, oklch(0.500 0.195 12))' }}
              >
                {errors.password.message}
              </p>
            )}
          </div>

          {/* Submit Button */}
          <button
            type="submit"
            disabled={isSubmitDisabled}
            className="flex h-11 w-full items-center justify-center gap-2 rounded-[6px] text-sm font-medium transition-colors"
            style={{
              backgroundColor: isSubmitDisabled
                ? 'var(--n-200, oklch(0.908 0.006 29))'
                : 'var(--red-500, oklch(0.552 0.205 29))',
              color: isSubmitDisabled
                ? 'var(--n-400, oklch(0.700 0.008 29))'
                : 'white',
              cursor: isSubmitDisabled ? 'not-allowed' : 'pointer',
            }}
            onMouseEnter={(e) => {
              if (!isSubmitDisabled) {
                e.currentTarget.style.backgroundColor = 'var(--red-600, oklch(0.485 0.193 29))';
              }
            }}
            onMouseLeave={(e) => {
              if (!isSubmitDisabled) {
                e.currentTarget.style.backgroundColor = 'var(--red-500, oklch(0.552 0.205 29))';
              }
            }}
            onFocus={(e) => {
              e.currentTarget.style.boxShadow = '0 0 0 3px var(--red-100, oklch(0.925 0.045 29))';
            }}
            onBlur={(e) => {
              e.currentTarget.style.boxShadow = 'none';
            }}
          >
            {isLoading ? (
              <>
                <Loader2 size={16} className="animate-spin" aria-hidden="true" />
                Memproses...
              </>
            ) : (
              'Masuk'
            )}
          </button>
        </form>
      </div>
    </div>
  );
}
