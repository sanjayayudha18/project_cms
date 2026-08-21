import { useAuthStore } from "@/lib/auth";
import { zodResolver } from "@hookform/resolvers/zod";
import { Navigate, useRouter } from "@tanstack/react-router";
import { AlertCircle, ArrowRight, Eye, EyeOff, Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

// ─── Schema & pure helpers (exported for property tests) ──────────────────────

function credentialField(maxLen: number, maxMessage: string) {
  return z
    .string()
    .min(1, "Wajib diisi")
    .max(maxLen, maxMessage)
    .refine((value) => value.trim().length > 0, { message: "Wajib diisi" });
}

export const loginSchema = z.object({
  username: credentialField(128, "Maksimal 128 karakter"),
  password: credentialField(72, "Maksimal 72 karakter"),
});

export type LoginFormData = z.infer<typeof loginSchema>;

/** Formats Retry-After seconds as "M menit S detik". */
export function formatRetryAfter(seconds: number): string {
  const safe = Math.max(0, Math.floor(seconds));
  const minutes = Math.floor(safe / 60);
  const secs = safe % 60;
  return `${minutes} menit ${secs} detik`;
}

/** Remaining whole seconds until deadline; never negative. */
export function remainingRateLimitSeconds(deadlineMs: number, nowMs: number = Date.now()): number {
  return Math.max(0, Math.ceil((deadlineMs - nowMs) / 1000));
}

// ─── Component ────────────────────────────────────────────────────────────────

export function LoginPage() {
  const login = useAuthStore((s) => s.login);
  const storeError = useAuthStore((s) => s.error);
  const rateLimitRetryAfter = useAuthStore((s) => s.rateLimitRetryAfter);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const isAuthLoading = useAuthStore((s) => s.isAuthLoading);
  const router = useRouter();

  const [showPassword, setShowPassword] = useState(false);
  const [rateLimitDeadline, setRateLimitDeadline] = useState<number | null>(null);
  const [rateLimitRemaining, setRateLimitRemaining] = useState(0);
  const passwordInputRef = useRef<HTMLInputElement | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    setFocus,
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: { username: "", password: "" },
  });

  const usernameField = register("username");
  const passwordField = register("password");

  // Stable deadline from store Retry-After seconds
  useEffect(() => {
    if (rateLimitRetryAfter != null && rateLimitRetryAfter > 0) {
      setRateLimitDeadline(Date.now() + rateLimitRetryAfter * 1000);
    }
  }, [rateLimitRetryAfter]);

  // Tick countdown from deadline
  useEffect(() => {
    if (rateLimitDeadline == null) {
      setRateLimitRemaining(0);
      return;
    }

    const tick = () => {
      const remaining = remainingRateLimitSeconds(rateLimitDeadline);
      setRateLimitRemaining(remaining);
      if (remaining <= 0) {
        setRateLimitDeadline(null);
      }
    };

    tick();
    const id = window.setInterval(tick, 250);
    return () => window.clearInterval(id);
  }, [rateLimitDeadline]);

  // Focus first invalid field after validation failure
  useEffect(() => {
    if (errors.username) {
      setFocus("username");
    } else if (errors.password) {
      setFocus("password");
    }
  }, [errors.username, errors.password, setFocus]);

  if (!isAuthLoading && isAuthenticated) {
    return <Navigate to="/" />;
  }

  const onSubmit = async (data: LoginFormData) => {
    await login(data.username, data.password);

    const state = useAuthStore.getState();
    if (state.isAuthenticated) {
      router.navigate({ to: "/" });
    }
  };

  const isRateLimited = rateLimitRemaining > 0;
  const isBusy = isSubmitting || isRateLimited;
  const alertMessage = storeError
    ? isRateLimited
      ? `${storeError}. Coba lagi dalam ${formatRetryAfter(rateLimitRemaining)}`
      : storeError
    : null;

  const togglePassword = () => {
    setShowPassword((prev) => !prev);
    requestAnimationFrame(() => {
      passwordInputRef.current?.focus();
    });
  };

  return (
    <div className="login-form">
      <div className="login-form__header">
        <h1 className="login-form__title">Selamat datang</h1>
        <p className="login-form__subtitle">Masuk untuk melanjutkan ke sistem manajemen kas</p>
      </div>

      {alertMessage && (
        <div className="login-form__alert" role="alert" data-testid="login-error">
          <AlertCircle size={16} className="login-form__alert-icon" aria-hidden="true" />
          <p className="login-form__alert-text">{alertMessage}</p>
        </div>
      )}

      <form
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="login-form__fields"
        aria-busy={isSubmitting ? "true" : undefined}
      >
        <div className="login-form__field-stack">
          <div className="login-form__field">
            <label htmlFor="username" className="login-form__label">
              Username
            </label>
            <input
              id="username"
              type="text"
              autoComplete="username"
              placeholder="Username"
              className={`login-form__input${errors.username ? " login-form__input--error" : ""}`}
              aria-invalid={errors.username ? "true" : undefined}
              aria-describedby={errors.username ? "username-error" : undefined}
              disabled={isSubmitting}
              {...usernameField}
            />
            {errors.username && (
              <p id="username-error" className="login-form__field-error">
                {errors.username.message}
              </p>
            )}
          </div>

          <div className="login-form__field">
            <label htmlFor="password" className="login-form__label">
              Kata Sandi
            </label>
            <div className="login-form__password-wrap">
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                autoComplete="current-password"
                placeholder="Masukkan kata sandi"
                className={`login-form__input login-form__input--password${errors.password ? " login-form__input--error" : ""}`}
                aria-invalid={errors.password ? "true" : undefined}
                aria-describedby={errors.password ? "password-error" : undefined}
                disabled={isSubmitting}
                {...passwordField}
                ref={(el) => {
                  passwordField.ref(el);
                  passwordInputRef.current = el;
                }}
              />
              <button
                type="button"
                className="login-form__toggle"
                onClick={togglePassword}
                aria-label={showPassword ? "Hide password" : "Show password"}
                aria-pressed={showPassword}
                disabled={isSubmitting}
              >
                {showPassword ? (
                  <EyeOff size={18} aria-hidden="true" />
                ) : (
                  <Eye size={18} aria-hidden="true" />
                )}
              </button>
            </div>
            {errors.password && (
              <p id="password-error" className="login-form__field-error">
                {errors.password.message}
              </p>
            )}
          </div>
        </div>

        <button type="submit" disabled={isBusy} className="login-form__submit login-submit">
          {isSubmitting ? (
            <>
              <Loader2 size={16} className="login-form__spinner" aria-hidden="true" />
              <span>Memproses...</span>
            </>
          ) : (
            <>
              <span>Masuk</span>
              <ArrowRight size={16} aria-hidden="true" />
            </>
          )}
        </button>

        {isSubmitting && (
          <output className="login-form__status" aria-live="polite">
            Memproses...
          </output>
        )}
      </form>
    </div>
  );
}

export default LoginPage;
