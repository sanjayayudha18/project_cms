import vendorHero from "@/assets/cit-vendor-illustration.png";
import { useAuth } from "@/features/auth/useAuth";
import { loginRoute } from "@/routes/login";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { AlertCircle, CheckCircle2, Eye, EyeOff } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";

// ─── Types ────────────────────────────────────────────────────────────────────

interface LoginFormData {
  username: string;
  password: string;
}

type LoginStatus = "idle" | "loading" | "done" | "error" | "locked";

// ─── Validation Helpers ───────────────────────────────────────────────────────

function isNonWhitespace(value: string): boolean {
  return value.trim().length > 0;
}

function formatCountdown(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return m > 0 ? `${m} menit ${s} detik` : `${s} detik`;
}

// ─── Small presentational bits ─────────────────────────────────────────────────

function WarningIcon() {
  return <AlertCircle size={15} className="shrink-0" aria-hidden="true" />;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function LoginPage() {
  const { state, login } = useAuth();
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = useState(false);
  const [countdown, setCountdown] = useState<number | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [isCapsLockOn, setIsCapsLockOn] = useState(false);
  const countdownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Redirect target from the typed + sanitized `redirect` search param.
  // Unsafe/missing values are transformed to undefined by the schema, so we
  // fall back to the default destination /orders (Requirement 3.3, 3.4).
  const { redirect } = useSearch({ from: loginRoute.id });
  const redirectTo = redirect ?? "/orders";

  // Already authenticated (or just logged in) → go to the preserved redirect target
  // (default /orders). Uses href so dynamic paths resolve. The guest guard performs
  // the same navigation; both agree on the target, so there is no redirect conflict.
  useEffect(() => {
    if (state.isAuthenticated && !state.isAuthLoading) {
      void navigate({ href: redirectTo, replace: true });
    }
  }, [state.isAuthenticated, state.isAuthLoading, navigate, redirectTo]);

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
    defaultValues: { username: "", password: "" },
  });

  const onSubmit = async (data: LoginFormData) => {
    setIsLoading(true);

    try {
      await login(data.username, data.password);
      // Use href so an arbitrary preserved path (possibly with dynamic segments,
      // e.g. /orders/123/evidence) resolves correctly.
      void navigate({ href: redirectTo, replace: true });
    } catch {
      // Error state is managed by AuthContext
    } finally {
      setIsLoading(false);
    }
  };

  const isRateLimited = countdown !== null && countdown > 0;
  const isSubmitDisabled = isLoading || isRateLimited;

  const status: LoginStatus = isRateLimited
    ? "locked"
    : isLoading
      ? "loading"
      : state.isAuthenticated
        ? "done"
        : state.error
          ? "error"
          : "idle";

  // Don't render login form if already authenticated
  if (state.isAuthenticated && !state.isAuthLoading) {
    return null;
  }

  // Show loading while checking auth state
  if (state.isAuthLoading) {
    return (
      <div
        className="flex min-h-dvh items-center justify-center"
        style={{ backgroundColor: "var(--bg)" }}
      >
        <span
          className="animate-crown-spin size-8 rounded-full border-2 border-[var(--border-c)] border-t-[var(--primary)]"
          role="status"
          aria-label="Memuat"
        />
      </div>
    );
  }

  return (
    <div
      className="grid min-h-dvh grid-cols-1 lg:grid-cols-[44fr_56fr]"
      style={{ backgroundColor: "var(--bg)", fontFamily: "var(--font-sans)" }}
    >
      {/* ─── Left brand panel ──────────────────────────────────────────────── */}
      <section
        aria-label="CROWN — CIMB Niaga Portal Vendor"
        className="relative hidden flex-col justify-between overflow-hidden px-12 py-14 lg:flex"
        style={{ backgroundColor: "var(--maroon-deep)", color: "var(--chrome-fg)" }}
      >
        <div className="animate-crown-rise relative flex items-center gap-2.5">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M3 8l4 3 5-6 5 6 4-3-2 10H5L3 8z"
              stroke="var(--primary-fg)"
              strokeWidth="1.6"
              strokeLinejoin="round"
            />
          </svg>
          <div>
            <p className="m-0 text-sm font-bold tracking-[0.02em]">CROWN</p>
            <p className="m-0 text-xs" style={{ color: "var(--vp-sidebar-text-muted, inherit)" }}>
              CIMB Niaga · Portal Vendor
            </p>
          </div>
        </div>

        <div
          className="animate-crown-rise relative flex max-w-[420px] flex-col gap-4"
          style={{ animationDelay: "0.1s" }}
        >
          <h1 className="m-0 text-[32px] font-bold leading-[1.15] tracking-[-0.02em]">
            Kelola replenishment ATM dari satu portal.
          </h1>
          <p
            className="m-0 text-sm leading-relaxed"
            style={{ color: "var(--vp-sidebar-text-muted, inherit)" }}
          >
            Unggah DSR, pantau jadwal CIT, dan kelola invoice dalam satu alur kerja yang
            terintegrasi dengan Cash Management System CIMB Niaga.
          </p>
        </div>

        <div
          className="animate-crown-rise relative -my-2 h-[220px] w-full max-w-[420px] self-center overflow-hidden"
          style={{ animationDelay: "0.15s" }}
        >
          <img
            src={vendorHero}
            alt=""
            aria-hidden="true"
            className="size-full object-cover object-[35%_62%] opacity-90"
          />
        </div>

        <dl
          className="animate-crown-rise relative m-0 flex gap-10 border-t pt-6"
          style={{ borderColor: "rgba(255,255,255,0.12)", animationDelay: "0.2s" }}
        >
          <div className="flex flex-col gap-1">
            <dt
              className="m-0 text-xs uppercase tracking-[0.08em]"
              style={{ color: "var(--vp-sidebar-text-muted, inherit)" }}
            >
              Batas unggah DSR
            </dt>
            <dd
              className="m-0 text-sm font-medium tabular-nums"
              style={{ fontFamily: "var(--font-mono)" }}
            >
              09.00 WIB
            </dd>
          </div>
          <div className="flex flex-col gap-1">
            <dt
              className="m-0 text-xs uppercase tracking-[0.08em]"
              style={{ color: "var(--vp-sidebar-text-muted, inherit)" }}
            >
              Bantuan operasional
            </dt>
            <dd
              className="m-0 text-sm font-medium tabular-nums"
              style={{ fontFamily: "var(--font-mono)" }}
            >
              021 1500 800
            </dd>
          </div>
        </dl>
      </section>

      {/* ─── Right form panel ──────────────────────────────────────────────── */}
      <section
        className="flex flex-col px-6 py-10 sm:px-12 lg:px-16"
        style={{ backgroundColor: "var(--surface)" }}
      >
        <div className="flex justify-end">
          <span
            className="flex items-center gap-[7px] text-[11px] font-semibold uppercase tracking-[0.11em]"
            style={{ color: "var(--text-secondary)" }}
          >
            <span
              className="size-1.5 shrink-0 rounded-full"
              style={{ backgroundColor: "var(--success)" }}
            />
            Layanan normal
          </span>
        </div>

        <div className="mx-auto my-auto w-full max-w-[392px]">
          <h2
            className="m-0 text-[28px] font-bold leading-[1.15] tracking-[-0.028em]"
            style={{ color: "var(--text)" }}
          >
            Masuk
          </h2>
          <p className="mt-2 text-[15px] leading-normal" style={{ color: "var(--text-secondary)" }}>
            Gunakan kredensial vendor yang diterbitkan CIMB Niaga.
          </p>

          <form
            onSubmit={handleSubmit(onSubmit)}
            noValidate
            className="mt-8 flex flex-col gap-5 lg:mt-10"
          >
            {/* Username */}
            <div className="flex flex-col gap-[7px]">
              <label
                htmlFor="user"
                className="text-[13px] font-medium"
                style={{ color: "var(--text)" }}
              >
                Nama pengguna
              </label>
              <input
                id="user"
                type="text"
                autoComplete="username"
                placeholder="gardanet.ops"
                maxLength={128}
                disabled={isSubmitDisabled}
                aria-invalid={errors.username ? "true" : undefined}
                aria-describedby={errors.username ? "user-err" : undefined}
                className="h-11 w-full rounded-[4px] border px-3.5 text-sm outline-none transition-[border-color,box-shadow] duration-150 focus-visible:shadow-[0_0_0_3px_var(--primary-tint)]"
                style={{
                  borderColor: errors.username ? "var(--danger)" : "var(--border-strong)",
                  backgroundColor: "var(--surface)",
                  color: "var(--text)",
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = "var(--primary)";
                }}
                onBlurCapture={(e) => {
                  if (!errors.username) {
                    e.currentTarget.style.borderColor = "var(--border-strong)";
                  }
                }}
                {...register("username", {
                  required: "Username wajib diisi",
                  validate: (v) => isNonWhitespace(v) || "Username wajib diisi",
                  maxLength: { value: 128, message: "Maksimal 128 karakter" },
                })}
              />
              {errors.username && (
                <p
                  id="user-err"
                  className="m-0 flex items-start gap-1.5 text-xs font-medium"
                  style={{ color: "var(--danger)" }}
                >
                  <WarningIcon />
                  {errors.username.message}
                </p>
              )}
            </div>

            {/* Password */}
            <div className="flex flex-col gap-[7px]">
              <label
                htmlFor="pass"
                className="text-[13px] font-medium"
                style={{ color: "var(--text)" }}
              >
                Kata sandi
              </label>
              <div className="relative flex items-center">
                <input
                  id="pass"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  placeholder="••••••••"
                  maxLength={72}
                  disabled={isSubmitDisabled}
                  aria-invalid={errors.password ? "true" : undefined}
                  aria-describedby={
                    errors.password ? "pass-err" : isCapsLockOn ? "pass-capslock" : undefined
                  }
                  className="h-11 w-full rounded-[4px] border px-3.5 pr-10 text-sm outline-none transition-[border-color,box-shadow] duration-150 focus-visible:shadow-[0_0_0_3px_var(--primary-tint)]"
                  style={{
                    borderColor: errors.password ? "var(--danger)" : "var(--border-strong)",
                    backgroundColor: "var(--surface)",
                    color: "var(--text)",
                  }}
                  onFocus={(e) => {
                    e.currentTarget.style.borderColor = "var(--primary)";
                  }}
                  onBlurCapture={(e) => {
                    if (!errors.password) {
                      e.currentTarget.style.borderColor = "var(--border-strong)";
                    }
                  }}
                  onKeyUp={(e) => setIsCapsLockOn(e.getModifierState("CapsLock"))}
                  onKeyDown={(e) => setIsCapsLockOn(e.getModifierState("CapsLock"))}
                  {...register("password", {
                    required: "Password wajib diisi",
                    validate: (v) => isNonWhitespace(v) || "Password wajib diisi",
                    maxLength: { value: 72, message: "Maksimal 72 karakter" },
                  })}
                />
                <button
                  type="button"
                  disabled={isSubmitDisabled}
                  onClick={() => setShowPassword((v) => !v)}
                  aria-label={showPassword ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"}
                  className="absolute right-2.5 flex size-6 items-center justify-center rounded-sm disabled:opacity-50"
                  style={{ color: "var(--text-muted)" }}
                >
                  {showPassword ? (
                    <EyeOff size={16} aria-hidden="true" />
                  ) : (
                    <Eye size={16} aria-hidden="true" />
                  )}
                </button>
              </div>
              {errors.password && (
                <p
                  id="pass-err"
                  className="m-0 flex items-start gap-1.5 text-xs font-medium"
                  style={{ color: "var(--danger)" }}
                >
                  <WarningIcon />
                  {errors.password.message}
                </p>
              )}
              {isCapsLockOn && !errors.password && (
                <p
                  id="pass-capslock"
                  className="m-0 flex items-start gap-1.5 text-xs font-medium"
                  style={{ color: "var(--warning)" }}
                >
                  <WarningIcon />
                  Caps Lock aktif.
                </p>
              )}
            </div>

            {/* Remember + forgot */}
            <div className="-mt-1 flex items-center justify-between gap-4">
              <label
                className="flex cursor-pointer items-center gap-2.5 text-sm"
                style={{ color: "var(--text-secondary)" }}
              >
                <input
                  type="checkbox"
                  disabled={isSubmitDisabled}
                  className="m-0 size-4 cursor-pointer"
                />
                Ingat perangkat ini
              </label>
              <a
                href="#reset"
                className="rounded-sm text-sm font-medium no-underline hover:underline hover:underline-offset-[3px]"
                style={{ color: "var(--primary-text)" }}
              >
                Lupa kata sandi?
              </a>
            </div>

            {/* Status region: error / locked / done */}
            <div aria-live="polite" aria-atomic="true">
              {status === "error" && (
                <p
                  role="alert"
                  className="m-0 flex gap-2.5 rounded-[4px] px-3.5 py-3 text-sm leading-[1.45]"
                  style={{ backgroundColor: "var(--danger-tint)", color: "var(--danger)" }}
                >
                  <WarningIcon />
                  <span>{state.error}</span>
                </p>
              )}

              {status === "locked" && countdown !== null && (
                <p
                  role="alert"
                  className="m-0 flex gap-2.5 rounded-[4px] px-3.5 py-3 text-sm leading-[1.45]"
                  style={{ backgroundColor: "var(--danger-tint)", color: "var(--danger)" }}
                >
                  <WarningIcon />
                  <span>
                    Terlalu banyak percobaan. Coba lagi dalam {formatCountdown(countdown)}. Hubungi
                    Bantuan Operasional di{" "}
                    <span className="font-medium" style={{ fontFamily: "var(--font-mono)" }}>
                      021 1500 800
                    </span>{" "}
                    untuk membuka lebih awal.
                  </span>
                </p>
              )}

              {status === "done" && (
                <output
                  className="m-0 flex gap-2.5 rounded-[4px] px-3.5 py-3 text-sm leading-[1.45]"
                  style={{ backgroundColor: "var(--success-tint)", color: "var(--success)" }}
                >
                  <CheckCircle2 size={16} className="shrink-0" aria-hidden="true" />
                  <span>Berhasil masuk. Mengalihkan ke dasbor vendor.</span>
                </output>
              )}
            </div>

            <button
              type="submit"
              disabled={isSubmitDisabled}
              className="flex h-11 items-center justify-center gap-2.5 rounded-[4px] text-[15px] font-semibold outline-none transition-colors focus-visible:shadow-[0_0_0_3px_var(--primary-tint)]"
              style={{
                backgroundColor: isSubmitDisabled ? "var(--border-c)" : "var(--primary)",
                color: isSubmitDisabled ? "var(--text-muted)" : "var(--primary-fg)",
                cursor: isSubmitDisabled ? "not-allowed" : "pointer",
              }}
            >
              {status === "loading" && (
                <span
                  className="animate-crown-spin size-4 rounded-full border-2 border-[rgba(255,252,251,0.35)] border-t-current"
                  aria-hidden="true"
                />
              )}
              {status === "done" && <CheckCircle2 size={16} aria-hidden="true" />}
              {status === "loading"
                ? "Memverifikasi"
                : status === "done"
                  ? "Berhasil masuk"
                  : "Masuk"}
            </button>
          </form>

          <p
            className="mt-7 max-w-[52ch] border-t pt-5 text-xs leading-normal"
            style={{ borderColor: "var(--border-c)", color: "var(--text-muted)" }}
          >
            Masukkan nama pengguna vendor, format nama.perusahaan.
          </p>
        </div>
      </section>
    </div>
  );
}
