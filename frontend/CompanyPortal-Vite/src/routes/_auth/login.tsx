import { useAuthStore } from "@/lib/auth";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "@tanstack/react-router";
import { AlertCircle, ArrowRight, Loader2 } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

// ─── Schema ───────────────────────────────────────────────────────────────────

const loginSchema = z.object({
  username: z.string().min(1, "Wajib diisi"),
  password: z.string().min(1, "Wajib diisi"),
});

type LoginFormData = z.infer<typeof loginSchema>;

// ─── Component ────────────────────────────────────────────────────────────────

export function LoginPage() {
  const login = useAuthStore((s) => s.login);
  const router = useRouter();
  const [loginError, setLoginError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: { username: "", password: "" },
  });

  const usernameField = register("username");
  const passwordField = register("password");

  const onSubmit = async (data: LoginFormData) => {
    setLoginError(null);
    await login(data.username, data.password);

    // Check store for errors after login attempt
    const state = useAuthStore.getState();
    if (state.error) {
      setLoginError(state.error);
    } else if (state.isAuthenticated) {
      router.navigate({ to: "/" });
    }
  };

  const applyFocusRing = (el: HTMLInputElement, hasError: boolean) => {
    if (!hasError) {
      el.style.borderColor = "var(--red-400)";
      el.style.boxShadow = "0 0 0 3px var(--red-100)";
    }
  };

  const clearFocusRing = (el: HTMLInputElement, hasError: boolean) => {
    if (!hasError) {
      el.style.borderColor = "var(--n-300)";
      el.style.boxShadow = "none";
    }
  };

  return (
    <div className="flex flex-col gap-[var(--space-8)]">
      {/* Header */}
      <div className="flex flex-col gap-[var(--space-2)]">
        <h1 className="text-2xl font-semibold tracking-tight" style={{ color: "var(--n-900)" }}>
          Selamat datang
        </h1>
        <p className="text-sm leading-relaxed" style={{ color: "var(--n-500)" }}>
          Masuk untuk melanjutkan ke sistem manajemen kas
        </p>
      </div>

      {/* Error */}
      {loginError && (
        <div
          className="flex items-center gap-[var(--space-3)] rounded-[var(--radius-md)] px-[var(--space-4)] py-[var(--space-3)]"
          style={{ backgroundColor: "var(--danger-bg)" }}
          role="alert"
          data-testid="login-error"
        >
          <AlertCircle
            size={16}
            className="shrink-0"
            style={{ color: "var(--danger-fg)" }}
            aria-hidden="true"
          />
          <p className="text-sm" style={{ color: "var(--danger-fg)" }}>
            {loginError}
          </p>
        </div>
      )}

      {/* Form */}
      <form
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex flex-col gap-[var(--space-6)]"
      >
        <div className="flex flex-col gap-[var(--space-4)]">
          {/* Username */}
          <fieldset className="flex flex-col gap-[6px]">
            <label
              htmlFor="username"
              className="text-[13px] font-medium"
              style={{ color: "var(--n-700)" }}
            >
              Username
            </label>
            <input
              id="username"
              type="text"
              autoComplete="username"
              placeholder="Username"
              className="h-10 w-full rounded-[var(--radius-md)] border px-[var(--space-3)] text-sm outline-none"
              style={{
                borderColor: errors.username ? "var(--danger-500)" : "var(--n-300)",
                backgroundColor: "var(--n-0)",
                color: "var(--n-800)",
              }}
              {...usernameField}
              onFocus={(e) => applyFocusRing(e.currentTarget, Boolean(errors.username))}
              onBlur={(e) => {
                usernameField.onBlur(e);
                clearFocusRing(e.currentTarget, Boolean(errors.username));
              }}
            />
            {errors.username && (
              <p className="text-xs" style={{ color: "var(--danger-fg)" }}>
                {errors.username.message}
              </p>
            )}
          </fieldset>

          {/* Password */}
          <fieldset className="flex flex-col gap-[6px]">
            <label
              htmlFor="password"
              className="text-[13px] font-medium"
              style={{ color: "var(--n-700)" }}
            >
              Kata Sandi
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              placeholder="Masukkan kata sandi"
              className="h-10 w-full rounded-[var(--radius-md)] border px-[var(--space-3)] text-sm outline-none"
              style={{
                borderColor: errors.password ? "var(--danger-500)" : "var(--n-300)",
                backgroundColor: "var(--n-0)",
                color: "var(--n-800)",
              }}
              {...passwordField}
              onFocus={(e) => applyFocusRing(e.currentTarget, Boolean(errors.password))}
              onBlur={(e) => {
                passwordField.onBlur(e);
                clearFocusRing(e.currentTarget, Boolean(errors.password));
              }}
            />
            {errors.password && (
              <p className="text-xs" style={{ color: "var(--danger-fg)" }}>
                {errors.password.message}
              </p>
            )}
          </fieldset>
        </div>

        {/* Submit */}
        <button
          type="submit"
          disabled={isSubmitting}
          className="flex h-11 w-full items-center justify-center gap-[var(--space-2)] rounded-[var(--radius-md)] text-sm font-medium disabled:cursor-not-allowed"
          style={{
            backgroundColor: isSubmitting ? "var(--n-200)" : "var(--red-500)",
            color: isSubmitting ? "var(--n-400)" : "white",
          }}
          onMouseEnter={(e) => {
            if (!isSubmitting) e.currentTarget.style.backgroundColor = "var(--red-600)";
          }}
          onMouseLeave={(e) => {
            if (!isSubmitting) e.currentTarget.style.backgroundColor = "var(--red-500)";
          }}
        >
          {isSubmitting ? (
            <>
              <Loader2 size={16} className="animate-spin" aria-hidden="true" />
              Memproses...
            </>
          ) : (
            <>
              Masuk
              <ArrowRight size={16} aria-hidden="true" />
            </>
          )}
        </button>
      </form>
    </div>
  );
}

export default LoginPage;
