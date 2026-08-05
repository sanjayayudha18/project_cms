import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useLocation, useNavigate } from 'react-router';
import { Loader2 } from 'lucide-react';
import { useAuth } from '@/features/auth/useAuth';

interface LoginFormData {
  username: string;
  password: string;
}

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [serverError, setServerError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const from = (location.state as { from?: string } | null)?.from ?? '/orders';

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    defaultValues: { username: '', password: '' },
  });

  const onSubmit = async (data: LoginFormData) => {
    setServerError(null);
    setIsLoading(true);

    try {
      await login(data.username, data.password);
      void navigate(from, { replace: true });
    } catch (error: unknown) {
      if (error instanceof Error) {
        if (error.message === 'Username atau password salah') {
          setServerError('Username atau password salah');
        } else {
          setServerError('Gagal terhubung ke server. Silakan coba lagi.');
        }
      } else {
        setServerError('Gagal terhubung ke server. Silakan coba lagi.');
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-svh items-center justify-center bg-surface p-4">
      <div className="w-full max-w-sm rounded-lg bg-white p-8 shadow-md">
        {/* Logo area */}
        <div className="mb-6 text-center">
          <h1 className="text-2xl font-bold text-topbar">CIMB Niaga</h1>
          <p className="mt-1 text-sm text-surface-text/70">Vendor Portal</p>
        </div>

        {/* Error display */}
        {serverError && (
          <div
            role="alert"
            id="login-error"
            className="mb-4 rounded-md bg-danger-bg px-4 py-3 text-sm text-danger-fg"
          >
            {serverError}
          </div>
        )}

        {/* Login form */}
        <form onSubmit={handleSubmit(onSubmit)} noValidate>
          {/* Username field */}
          <div className="mb-4">
            <label
              htmlFor="username"
              className="mb-1 block text-sm font-medium text-surface-text"
            >
              Username
            </label>
            <input
              id="username"
              type="text"
              autoComplete="username"
              aria-describedby={errors.username ? 'username-error' : undefined}
              className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm text-surface-text placeholder:text-neutral-400 focus:border-sidebar-active focus:ring-2 focus:ring-sidebar-active/20 focus:outline-none disabled:bg-neutral-100 disabled:text-neutral-400"
              disabled={isLoading}
              {...register('username', {
                required: 'Username wajib diisi',
              })}
            />
            {errors.username && (
              <p id="username-error" className="mt-1 text-xs text-danger-fg">
                {errors.username.message}
              </p>
            )}
          </div>

          {/* Password field */}
          <div className="mb-6">
            <label
              htmlFor="password"
              className="mb-1 block text-sm font-medium text-surface-text"
            >
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              aria-describedby={errors.password ? 'password-error' : (serverError ? 'login-error' : undefined)}
              className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm text-surface-text placeholder:text-neutral-400 focus:border-sidebar-active focus:ring-2 focus:ring-sidebar-active/20 focus:outline-none disabled:bg-neutral-100 disabled:text-neutral-400"
              disabled={isLoading}
              {...register('password', {
                required: 'Password wajib diisi',
                minLength: {
                  value: 8,
                  message: 'Password minimal 8 karakter',
                },
              })}
            />
            {errors.password && (
              <p id="password-error" className="mt-1 text-xs text-danger-fg">
                {errors.password.message}
              </p>
            )}
          </div>

          {/* Submit button */}
          <button
            type="submit"
            disabled={isLoading}
            className="flex w-full items-center justify-center gap-2 rounded-md bg-sidebar-active px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sidebar-active/90 focus:ring-2 focus:ring-sidebar-active/30 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isLoading && <Loader2 className="h-4 w-4 animate-spin" />}
            {isLoading ? 'Memproses...' : 'Masuk'}
          </button>
        </form>
      </div>
    </div>
  );
}
