# Implementation Plan: Company Portal Login Revamp

## Outcome

The internal Company Portal `/login` will use a reference-inspired dark burgundy split screen: a primary CMS identity/form panel and a local ATM/CIT artwork panel. Existing Indonesian copy, username/password authentication, Zustand state, TanStack Router navigation, and protected-route behavior remain intact.

## Scope

- **In scope:** Company Portal `_auth` layout and `/login`, local artwork packaging, scoped login tokens/CSS, React Hook Form/Zod validation, Zustand state consumption, password visibility, loading/error/rate-limit UI, responsive behavior, accessibility, and frontend tests.
- **Out of scope:** backend auth, `/api/v1/auth/login` contract, token/role/permission changes, database tables/columns/migrations, env vars, Vendor Portal, protected pages, password reset, MFA, SSO, LDAP, recovery, self-registration, trust-device persistence, and new dependencies.

## Evidence Reviewed

- `.opencode/opencode.md`: authoritative React 19/TanStack Router/React Hook Form/Zod/Zustand/Vitest stack; OKLCH and accessibility rules; internal Company Portal vs Vendor Portal split; light mode only; no invented API/table/env/dependency changes.
- `.kiro/specs/revamp-login/requirements.md`: requirements 1-10 covering composition, identity/copy, artwork, form, auth preservation, states, tokens, responsive/motion behavior, accessibility/security, and verification.
- `login_ui_design/crown-login-red.html`: reference `1.08fr 1fr` layout, burgundy wells, compact labels, artwork scrim/skeleton, and responsive stacking. Its remote image/fonts, vendor copy, fake metrics/EOD, trust-device control, reset link, and fake success must not be copied.
- `login_ui_design/CROWN IMG.png`: local artwork containing an ATM, cash-in-transit vehicle, cash containers, banknotes, and route diagram.
- `frontend/CompanyPortal-Vite/src/routes/_auth.tsx`: current `_auth` boundary owns the split shell, CMS identity, copy, and footer; it is the layout entry point for the new shell.
- `frontend/CompanyPortal-Vite/src/routes/_auth/login.tsx`: current RHF/Zod form retains the required Indonesian labels/copy and calls `useAuthStore().login`; it lacks max lengths, whitespace-only enforcement, password visibility, rate-limit rendering, and complete ARIA wiring.
- `frontend/CompanyPortal-Vite/src/styles/tokens.css`: canonical OKLCH red, neutral, semantic, spacing, radius, shadow, and system-font tokens.
- `frontend/CompanyPortal-Vite/src/styles/index.css`: unused legacy light login-card rules; replace the login-specific section with scoped shell/form/artwork rules rather than changing global app styling.
- `frontend/CompanyPortal-Vite/src/routes/_auth/login.test.tsx`: existing field/title/submit, empty validation, 401 alert, success navigation, and alert tests.
- `frontend/CompanyPortal-Vite/src/routes/_auth/login.property.test.ts`: existing whitespace and Retry-After properties duplicate source logic and need to exercise the actual schema/formatter plus length/clamp boundaries.
- `frontend/CompanyPortal-Vite/src/lib/auth/store.ts`: existing `login` sends `POST /api/v1/auth/login`, `{ username, password }`, `X-Portal-Type: company`, and `credentials: include`; it exposes `error` and `rateLimitRetryAfter` and remains the auth source of truth.
- `frontend/CompanyPortal-Vite/src/lib/auth/store.test.ts`: already covers request contract and 401/403/422/429/503/network/Retry-After behavior.
- `frontend/CompanyPortal-Vite/src/routes/login.tsx`: existing TanStack Router route mounts `LoginPage` under `_auth`, with no route-level authenticated guard.
- `frontend/CompanyPortal-Vite/package.json`, `vite.config.ts`: supported commands are `pnpm test`, `pnpm build`, `pnpm lint`, and Vitest via the existing Vite setup. `public/` currently has only favicon/icons.
- Worktree status: unrelated ATM Portal, planner, and config changes are present; they do not overlap this plan and must be preserved.

## Requirements and Acceptance Criteria

- [ ] `/login` stays under the existing TanStack Router route and `_auth` boundary; no second auth shell.
- [ ] Desktop uses an approximately `minmax(0, 1.08fr) minmax(0, 1fr)` split with the identity/form region primary and artwork full height.
- [ ] CMS/Cash Management System and Company Portal identity remain; retain `Selamat datang`, `Masuk untuk melanjutkan ke sistem manajemen kas`, `Username`, `Kata Sandi`, `Masuk`, `Memproses...`, and `© 2026 CIMB Niaga STCC`; add no fabricated stats/status/version/counters.
- [ ] `CROWN IMG.png` is local, cropped with `object-fit: cover`, meaningfully described, and has a coherent burgundy fallback on load failure.
- [ ] Exactly one username and one password field remain, with visible labels, unique IDs, correct type/autocomplete/placeholder, default password masking, accessible eye toggle, and a submit target at least 44px high.
- [ ] Zod rejects empty/whitespace-only values with `Wajib diisi`, enforces username <=128 and password <=72, prevents auth on invalid input, and focuses the first invalid field.
- [ ] Enter and click share one RHF submit handler; only `useAuthStore().login` performs auth; no credential/token storage or query parameters; success navigates to `/` without reload; authenticated `/login` redirects to `/`.
- [ ] Loading prevents duplicates, retains values, exposes busy/status semantics, and shows `Memproses...`; existing localized store errors remain safe and retryable; 429 shows `Terlalu banyak percobaan login` and `M menit S detik` when available.
- [ ] Rate limiting uses one stable deadline, clamps at zero, and re-enables without reload. Scoped login colors use OKLCH tokens, accessible contrast/focus states, no arbitrary color literals, remote fonts, or new dependencies.
- [ ] Desktop/tablet/mobile/short viewport, keyboard, reduced motion, image failure, field/error/status ARIA, and no-secret persistence are verified; `pnpm test` and `pnpm build` pass.

## Assumptions and Open Questions

- **Approval needed:** project context says light mode only, while this request/requirements require a dark burgundy login. Treat this as a narrowly scoped approved exception for `.login-shell` descendants only; do not add global dark mode or a toggle. If not approved, stop and revise the direction.
- **Copy assumption:** retain the current approved operational sentence from `_auth.tsx` (`Kelola operasional kas ATM secara terintegrasi, dari peramalan hingga rekonsiliasi.`) and current Indonesian form copy. Any new headline/artwork caption needs product approval; do not invent live metrics.
- **Artwork alt assumption:** use a meaningful Indonesian description of the ATM, cash-in-transit vehicle, containers, banknotes, and route diagram. If product design declares it decorative, use `alt=""` and `aria-hidden="true"` consistently instead.
- **No backend/data approval is needed:** this is frontend-only and uses the existing auth store/contract.

## Architecture and Data Impact

- **Affected modules:** Company Portal `_auth` layout, login page, login route guard, scoped styles, and tests. `src/lib/auth/store.ts` remains unchanged.
- **API contract:** existing `POST /api/v1/auth/login`; body/header/credentials unchanged. No endpoint/event/API client change.
- **Database and routing:** no database impact; no primary/replica read/write path is added.
- **Security/state:** Zustand remains in-memory auth state; do not persist/log passwords or tokens. No maker-checker/audit change applies to a login screen.
- **Asset strategy:** add `frontend/CompanyPortal-Vite/public/assets/crown-login-artwork.png` as a copy of `login_ui_design/CROWN IMG.png`; use `src="/assets/crown-login-artwork.png"`, not a remote URL or TS image import. This avoids declarations/dependencies and works through Vite/Nginx public serving and offline delivery.

## Implementation Steps

### Phase 1: Contract and RED tests

1. **Confirm the scoped dark exception and copy.**
   - Files: `.opencode/opencode.md`, `.kiro/specs/revamp-login/requirements.md` (review only).
   - Change: record approval for the dark login exception and any new headline/caption; retain current Indonesian copy by default. Do not introduce vendor/reset/trust/fake-status content.
   - Depends on: none.
   - Verification: approval is recorded before application implementation.
   - Risk: High, due to the light-mode policy conflict.

2. **Add failing component interaction tests first (RED).**
   - Files: `frontend/CompanyPortal-Vite/src/routes/_auth/login.test.tsx` (`describe("LoginPage")`).
   - Change: cover exactly one username/password field; labels, IDs, types, autocomplete, placeholder; empty and whitespace-only validation with `Wajib diisi`; 128/72 limits; no fetch and first-invalid focus; Enter submission; loading/disabled/status; invalid credentials and existing 403/503/network messages; success navigation; authenticated redirect; password toggle preserving value/focus and exposing action/state; 429 message/countdown/disabled retry; image source/alt/load error fallback. Use fake timers or controlled `Date.now` for countdown tests.
   - Depends on: step 1.
   - Verification: run the focused file and confirm new assertions fail for missing behavior, not because selectors are decorative.
   - Risk: Medium.

3. **Make property tests exercise source logic (RED).**
   - Files: `frontend/CompanyPortal-Vite/src/routes/_auth/login.property.test.ts`.
   - Change: replace duplicated schema/formatter definitions with imports from the actual exported helpers in `login.tsx` (or an existing-equivalent co-located helper); add arbitrary whitespace, max-length boundary, valid boundary, formatting, zero, and non-negative clamp properties. Correct stale comments claiming trimming is already implemented.
   - Depends on: step 2.
   - Verification: properties expose current missing trim/length behavior before implementation.
   - Risk: Low.

### Phase 2: Asset and layout

4. **Add the local artwork.**
   - Files: add `frontend/CompanyPortal-Vite/public/assets/crown-login-artwork.png`; source `login_ui_design/CROWN IMG.png` is read/copy-only.
   - Change: copy the supplied PNG without changing it. Do not copy the reference remote URL/fonts or embed base64.
   - Depends on: step 1.
   - Verification: built/previewed `/assets/crown-login-artwork.png` loads without external network access.
   - Risk: Medium.

5. **Rebuild the existing `_auth` shell around the Outlet.**
   - Files: `frontend/CompanyPortal-Vite/src/routes/_auth.tsx` (`AuthLayout`).
   - Change: keep the route boundary and `Outlet`, but render a semantic `.login-shell` with identity panel (CMS lockup, current Indonesian operational copy, `Outlet`, authorized/logged footer, attribution) and artwork `<aside>`. The artwork state handles load/ready/error, stable crop, caption/scrim, and burgundy fallback. No fabricated operational metadata or vendor link.
   - Depends on: step 4.
   - Verification: desktop DOM has identity/form and artwork regions; mobile DOM has one form/Outlet and a logical focus order; image `load` and `error` tests retain the panel/caption.
   - Risk: Medium.

### Phase 3: Tokens, CSS, form, and auth behavior

6. **Add scoped OKLCH login tokens and styles.**
   - Files: `frontend/CompanyPortal-Vite/src/styles/tokens.css`, `frontend/CompanyPortal-Vite/src/styles/index.css`.
   - Change: add `--login-*` deep burgundy/chrome/well/hover/border/text/focus/semantic properties. Adapt the reference primary to canonical CIMB `--red-*` hue-29 tokens where possible. Replace unused legacy light login-card rules with shell, panel, form, field, alert, artwork, fallback, responsive, and reduced-motion rules. Use only existing spacing tokens and radii; no global `color-scheme: dark`, arbitrary hex/RGB/HSL, pure black/white, gradient text, remote font, or new visual dependency. Use `object-fit: cover`, safe padding, vertical scrolling, 44px targets, and a visible non-animated reduced-motion loading indicator.
   - Depends on: step 5.
   - Verification: add actual dark token pairs to `src/styles/contrast.property.test.ts` where its static token list requires it; check normal/large text contrast and focus visibility.
   - Risk: High, due to policy/contrast.

7. **Implement the RHF/Zod form boundary.**
   - Files: `frontend/CompanyPortal-Vite/src/routes/_auth/login.tsx` (`loginSchema`, `LoginPage`, `onSubmit`).
   - Change: keep two registered fields and one `handleSubmit` handler. Use a Zod refinement that rejects whitespace-only values without trimming a valid password, plus username `.max(128)` and password `.max(72)`; retain `Wajib diisi` and RHF first-error focus. Add visible labels, unique IDs, `aria-describedby`, `aria-invalid`, autocomplete values, and semantic inline field errors. Select Zustand `login`, `error`, `isAuthenticated`, and `rateLimitRetryAfter`; do not duplicate fetch/token logic. Export only pure schema/formatter helpers needed by the property test if no existing helper location is preferred.
   - Depends on: step 6.
   - Verification: invalid input makes no request; valid input calls the existing store action; current `store.test.ts` remains green without contract changes.
   - Risk: High, because this is the auth boundary.

8. **Add visibility, loading, errors, rate-limit deadline, and navigation.**
   - Files: `frontend/CompanyPortal-Vite/src/routes/_auth/login.tsx`; `frontend/CompanyPortal-Vite/src/routes/login.tsx` (`loginRoute`); `frontend/CompanyPortal-Vite/src/routes/_auth/login.test.tsx`.
   - Change: use existing Lucide eye icons in a practical 44px button with `Show password`/`Hide password`, `aria-pressed` or equivalent, preserved value, and password focus. Use `isSubmitting` to prevent duplicate submit, retain values, expose `aria-busy`/live status, and render one inline `role="alert"`. On `rateLimitRetryAfter`, create one deadline from `Date.now() + seconds*1000`; derive `Math.max(0, ceil((deadline-now)/1000))` on ticks, clean up the interval, and enable at zero. Add a synchronous authenticated `beforeLoad` guard plus component-level Zustand observation for root initialization races; navigate with router `{ to: "/" }` only, without full reload or fake success delay.
   - Depends on: step 7.
   - Verification: tests cover every existing error, unknown Retry-After, exact countdown, no negative time, retry after error, successful navigation, and authenticated `/login`; `_protected.tsx` remains untouched.
   - Risk: High, due to redirect loops and rate-limit bypass risk.

9. **Green responsive/a11y/motion checks.**
   - Files: `_auth.tsx`, `login.tsx`, `index.css`.
   - Change: desktop full-height artwork; below desktop stack/compact artwork above identity; narrow mobile keeps identity, fields, alerts, action, and footer available; short viewports scroll; focus order follows DOM; reduced motion removes artwork fade/skeleton sweep/decorative transitions and spinner animation where possible. Keep status understandable without color alone.
   - Depends on: steps 6-8.
   - Verification: keyboard-only Tab/Enter/Space, desktop/tablet/mobile/short-height manual checks, image failure, and `prefers-reduced-motion: reduce`.
   - Risk: Medium.

### Phase 4: Improve and verification

10. **Refactor after GREEN.**
    - Files: all touched login files and tests only.
    - Change: remove inline focus-ring mutation (`applyFocusRing`/`clearFocusRing`), keep styling in token-backed CSS, remove stale duplicate login CSS, keep helper logic pure and functions small, and preserve the existing store contract.
    - Depends on: steps 7-9.
    - Verification: focused tests pass after each refactor; diff contains no unrelated worktree changes, secrets, endpoint changes, or new dependency.
    - Risk: Low.

11. **Run the full quality gate.**
    - Files: no additional source files.
    - Change: run focused tests, full tests, build, lint, and the repository's production-style TypeScript/Vite check; manually verify offline asset serving and responsive/a11y states.
    - Depends on: step 10.
    - Verification: all commands below pass with no skipped tests.
    - Risk: Medium.

## Exact File Change Set

### Add

- `frontend/CompanyPortal-Vite/public/assets/crown-login-artwork.png`: copy of `login_ui_design/CROWN IMG.png`, referenced as `/assets/crown-login-artwork.png`.

### Change

- `frontend/CompanyPortal-Vite/src/routes/_auth.tsx`: identity/artwork shell, semantic regions, local image states/fallback, copy/footer, responsive structure.
- `frontend/CompanyPortal-Vite/src/routes/_auth/login.tsx`: Zod boundary, RHF ARIA wiring, Zustand selectors, loading/error/rate-limit UI, password toggle, and navigation.
- `frontend/CompanyPortal-Vite/src/routes/login.tsx`: authenticated `beforeLoad` guard, paired with the component guard for initialization races.
- `frontend/CompanyPortal-Vite/src/styles/tokens.css`: scoped `--login-*` OKLCH tokens; global light tokens remain unchanged.
- `frontend/CompanyPortal-Vite/src/styles/index.css`: scoped login shell/form/artwork/responsive/reduced-motion CSS; replace unused legacy login rules.
- `frontend/CompanyPortal-Vite/src/routes/_auth/login.test.tsx`: RED/GREEN component and interaction coverage.
- `frontend/CompanyPortal-Vite/src/routes/_auth/login.property.test.ts`: properties against actual validation/formatting and countdown boundaries.
- `frontend/CompanyPortal-Vite/src/styles/contrast.property.test.ts`: explicit contrast pairs for new login values if required by its static token test.

### Intentionally unchanged

- `frontend/CompanyPortal-Vite/src/lib/auth/store.ts`: existing Zustand action and `POST /api/v1/auth/login` remain the source of truth.
- `frontend/CompanyPortal-Vite/src/lib/auth/store.test.ts`: existing request/error/rate-limit contract tests stay green; extend only for regression, never to change the contract.
- Backend, migrations, env files, Vendor Portal, protected routes, package manifests, lockfiles, and dependencies.

## Requirement Traceability

| Requirement | Task coverage |
|---|---|
| R1 Composition | Steps 5-6 and 9: existing `/login` + `_auth`, 1.08fr/1fr split, primary form, desktop artwork, no generic card/marketing. |
| R2 Identity/copy | Steps 1 and 5: CMS/Company Portal identity, current Indonesian copy, attribution, capability-only caption, no fabricated metrics. |
| R3 Artwork | Steps 4-5: local public PNG, stable crop, meaningful alt, caption treatment, load-error fallback. |
| R4 Form | Step 7-8: exactly two fields, labels/attributes, masking, accessible eye control, full-width >=44px action, no reference-only controls. |
| R5 Validation/auth | Steps 2-3, 7-8: whitespace/max rules, no request on invalid, focus/Enter, existing Zustand contract, no storage, success/authenticated redirects. |
| R6 Loading/errors/rate limits | Steps 2 and 8: duplicate prevention, retained values, alert/status, existing errors, stable Retry-After deadline, zero clamp/retry. |
| R7 Dark system | Approval note and step 6: scoped OKLCH tokens, canonical red/semantic colors, wells/focus/hover/disabled, no arbitrary colors/fonts/deps, contrast tests. |
| R8 Responsive/motion | Step 9: desktop artwork, stacked/compact breakpoints, scroll/padding/targets, usable loading, reduced motion. |
| R9 Accessibility/security | Steps 2, 7-9: labels/IDs/autocomplete, ARIA relationships, keyboard/focus, artwork semantics, text plus icons, no secret persistence. |
| R10 Verification | Steps 2-3 and 10-11: expanded component/property tests, store contract regression, focused/full commands, build/lint/manual checks. |

## Testing and Verification

- **Unit/property:** import the actual schema and formatter in `login.property.test.ts`; test arbitrary whitespace-only strings, username/password max boundaries, valid boundaries, `M*60+S=N`, zero, and non-negative clamping.
- **Component/integration:** `login.test.tsx` covers semantic fields, validation/no-request, first-invalid focus, Enter, password toggle, loading, all existing store errors, alert/status semantics, 429 countdown, success/authenticated navigation, local asset URL/alt, and image `error` fallback.
- **Auth regression:** keep `src/lib/auth/store.test.ts` green for URL, method, body, `X-Portal-Type: company`, `credentials: include`, response mapping, and Retry-After; no fake endpoint or backend fixture.
- **Accessibility/manual:** keyboard-only use, visible focus on dark wells/button states, `aria-invalid`/`aria-describedby`, `role="alert"`, busy/live status, eye-button name/state, non-color-only feedback, desktop/tablet/mobile/short viewport, offline asset, image failure, and reduced motion. Use existing Playwright login coverage only if present; do not invent a backend flow.
- **Commands from `frontend/CompanyPortal-Vite/`:**
  - `pnpm exec vitest run src/routes/_auth/login.test.tsx src/routes/_auth/login.property.test.ts src/styles/contrast.property.test.ts`
  - `pnpm test`
  - `pnpm build`
  - `pnpm lint`
  - `pnpm tsc -b tsconfig.app.json && pnpm vite build`

## Risks and Mitigations

- **Light-mode conflict:** require explicit approval for a `.login-shell`-only dark exception; keep global tokens, protected pages, and the rest of the app light.
- **Auth contract regression:** leave `store.ts` untouched and assert URL, method, headers, credentials, body, error mapping, and success mapping in existing tests.
- **Credential exposure/semantics:** no logging/storage/query params; keep password raw and auth inside Zustand; do not duplicate token logic.
- **Rate-limit bypass/drift:** one deadline plus current-time calculation, interval cleanup, clamp at zero, and fake-timer tests.
- **Redirect loop/auth race:** route-level guard only when auth is resolved, component-level observation for post-initialize state, and no `window.location` navigation.
- **Contrast/accessibility regression:** token-pair tests, explicit labels/ARIA, visible focus, keyboard tests, and text/icons for status rather than color alone.
- **Broken offline artwork:** public asset path, built-asset check, `onError` fallback, and no remote URL/font.
- **Responsive clipping/duplicate controls:** one Outlet/form, safe padding/vertical overflow, DOM-order focus, and manual short-height checks.
- **Unrelated worktree changes:** only the task document is written now; implementation must touch the exact set above and preserve existing ATM Portal/planner/config changes.

## Definition of Done

- [ ] Dark-login exception and any new headline/caption are explicitly approved, or the conflict is resolved before implementation.
- [ ] Local PNG exists at the exact public path and remote reference assets/fonts/mock behaviors are absent.
- [ ] `/login` remains under `_auth`; authenticated users redirect to `/`; protected routes and Vendor Portal are unchanged.
- [ ] Exactly one username/password field has all required attributes, Indonesian copy, validation, focus behavior, visibility toggle, loading, errors, and rate-limit countdown.
- [ ] Zustand remains the only component-to-auth boundary; endpoint, header, body, credentials, and token handling are unchanged.
- [ ] Scoped OKLCH tokens/CSS satisfy contrast, focus, responsive, light-mode-scope, and reduced-motion constraints.
- [ ] Image load/error, meaningful alt/fallback, keyboard operation, alert/status semantics, and no-secret persistence are verified.
- [ ] RED -> GREEN -> IMPROVE is complete; focused tests, `pnpm test`, `pnpm build`, `pnpm lint`, and the TypeScript/Vite check pass with no skipped tests.
- [ ] Implementation changes only planned login files plus the local asset; unrelated worktree changes are preserved.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3"] },
    { "id": 2, "tasks": ["2.1", "2.2"] },
    { "id": 3, "tasks": ["3.1"] },
    { "id": 4, "tasks": ["3.2", "3.3"] },
    { "id": 5, "tasks": ["3.4"] },
    { "id": 6, "tasks": ["4.1"] },
    { "id": 7, "tasks": ["4.2"] }
  ]
}
```

**Smallest next implementation step:** obtain approval for the scoped dark-login exception and any new copy, then add the failing semantic interaction tests in `frontend/CompanyPortal-Vite/src/routes/_auth/login.test.tsx` without touching backend or auth-store code.

**WAITING FOR CONFIRMATION:** do not implement until the user explicitly approves this plan, including the light-mode exception and final new copy.
