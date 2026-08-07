---
inclusion: auto
description: Project-specific patterns, preferences, and lessons learned over time (user-editable)
---

# Lessons Learned

This file captures project-specific patterns, coding preferences, common pitfalls, and architectural decisions that emerge during development. It serves as a workaround for continuous learning by allowing you to document patterns manually.

**How to use this file:**
1. The `extract-patterns` hook will suggest patterns after agent sessions
2. Review suggestions and add genuinely useful patterns below
3. Edit this file directly to capture team conventions
4. Keep it focused on project-specific insights, not general best practices

---

## Project-Specific Patterns

*Document patterns unique to this project that the team should follow.*

### Example: API Error Handling
```typescript
// Always use our custom ApiError class for consistent error responses
throw new ApiError(404, 'Resource not found', { resourceId });
```

---

## Code Style Preferences

*Document team preferences that go beyond standard linting rules.*

### Example: Import Organization
```typescript
// Group imports: external, internal, types
import { useState } from 'react';
import { Button } from '@/components/ui';
import type { User } from '@/types';
```

---

## Kiro Hooks

### `install.sh` is additive-only — it won't update existing installations
The installer skips any file that already exists in the target (`if [ ! -f ... ]`). Running it against a folder that already has `.kiro/` will not overwrite or update hooks, agents, or steering files. To push updates to an existing project, manually copy the changed files or remove the target files first before re-running the installer.

### README.md mirrors hook configurations — keep them in sync
The hooks table and Example 5 in README.md document the action type (`runCommand` vs `askAgent`) and behavior of each hook. When changing a hook's `then.type` or behavior, update both the hook file and the corresponding README entries to avoid misleading documentation.

### Prefer `askAgent` over `runCommand` for file-event hooks
`runCommand` hooks on `fileEdited` or `fileCreated` events spawn a new terminal session every time they fire, creating friction. Use `askAgent` instead so the agent handles the task inline. Reserve `runCommand` for `userTriggered` hooks where a manual, isolated terminal run is intentional (e.g., `quality-gate`).

---

## Common Pitfalls

*Document mistakes that have been made and how to avoid them.*

### fast-check property tests: TypeScript array indexing requires guards
Vitest runs tests without full `tsc` type-checking, so `NAV_CONFIG[index]` compiles fine at runtime but fails `tsc -b` with "possibly undefined." When writing property tests that index into typed arrays with `fc.integer({ min: 0, max: arr.length - 1 })`, always add a narrowing guard:
```typescript
const item = NAV_CONFIG[index];
if (!item) return; // satisfies tsc strict mode
```
This pattern applies to all fast-check properties using array indexing in this project.

### Stub API only intercepts `apiClient` — direct `fetch()` hits the network
The stub interceptor (`handleStubRequest`) is wired into the API client's `executeRequest` function via dynamic import when `apiConfig.mode === "stub"`. However, code outside the API client (e.g., auth store's `postLogin` / `postRefreshToken`) must also explicitly check `apiConfig.mode` and call `handleStubRequest` directly — there is no global fetch monkey-patch. When adding new direct-fetch flows:
- Import `apiConfig` and check `mode === "stub"`
- Use `handleStubRequest(url, options)` for stub mode
- Fall through to real `fetch()` for real mode
- In stub mode, `postRefreshToken()` returns `null` immediately (no persisted session) so the login page always shows on first boot

### TanStack Router `beforeLoad` is synchronous — async auth needs component-level fallback
`beforeLoad` runs synchronously when a route is entered. If auth state depends on an async `initialize()` call (e.g., token refresh), `beforeLoad` may run before the auth store is resolved. Always pair `beforeLoad` with a component-level check:
```tsx
// In beforeLoad (catches immediate cases):
if (!isLoading && !isAuthenticated) throw redirect({ to: "/login" });

// In the layout component (catches async resolution):
if (!isLoading && !isAuthenticated) return <Navigate to="/login" />;
```
Also: after changing core modules like `__root.tsx` or auth store, Vite HMR may not fully refresh — always restart the dev server (`Ctrl+C` then `pnpm dev`) and hard-refresh the browser (`Ctrl+Shift+R`).

### Auth `initialize()` must not call `logout()` on failure
`logout()` does a hard `window.location.href = "/login"` redirect. If `initialize()` calls `logout()` when the refresh fails (first app boot, no session), it causes an infinite redirect loop. Instead, `initialize()` should only set state to `{ isAuthenticated: false, isLoading: false }` and let the route guard handle the redirect via TanStack Router.

### Example: Database Transactions
- Always wrap multiple database operations in a transaction
- Remember to handle rollback on errors
- Don't forget to close connections in finally blocks

---

## Architecture Decisions

*Document key architectural decisions and their rationale.*

### Example: State Management
- **Decision**: Use Zustand for global state, React Context for component trees
- **Rationale**: Zustand provides better performance and simpler API than Redux
- **Trade-offs**: Less ecosystem tooling than Redux, but sufficient for our needs

### Forcing API mode in tests: mock `@/lib/api/config`
Because the API client and auth store now branch on `apiConfig.mode`, and the test env resolves `VITE_API_MODE` to `"stub"` by default, any test that mocks `global.fetch` and expects the real network path must force real mode:
```ts
vi.mock("@/lib/api/config", () => ({
  apiConfig: { mode: "real", baseURL: "/api/v1", stubLatency: { min: 200, max: 800 } },
}));
```
Applies to `store.test.ts`, `client.test.ts`, and any future test exercising fetch-based flows.

### Only use defined spacing tokens — no interpolation
The 4pt spacing scale has specific stops: `--space-1` (4px), `--space-2` (8px), `--space-3` (12px), `--space-4` (16px), `--space-6` (24px), `--space-8` (32px), `--space-12` (48px), `--space-18` (72px). There is NO `--space-5`, `--space-7`, `--space-9`, etc. Using a nonexistent token (e.g., `p-[var(--space-5)]`) resolves to 0px in the browser, causing layout overflow. Always reference `src/styles/tokens.css` for the exact list.

### UI redesigns move elements behind interaction — update tests to match
When redesigning components, elements often move behind an interaction (e.g., the Header role badge + logout button moved inside a click-to-open dropdown). Component tests that assumed those elements were always visible must first trigger the interaction (`fireEvent.click(header-user-name)`) before asserting. Keep stable `data-testid` attributes on moved elements so tests can still target them after redesign.

### Spec task execution creates components in isolation — integration wiring may be missing
When tasks are executed by subagents, each task creates its own files independently (e.g., "Implement Sidebar component" creates `Sidebar.tsx`, "Implement AppShell" creates `AppShell.tsx` with placeholders). The parent component may end up with `{/* Placeholder: Component X renders here */}` comments instead of actual imports and usage. After completing a wave of tasks, always verify that child components are actually imported and rendered in their parent — don't assume the integration happened.

### TanStack Table `ColumnDef` type: use `any` for cell accessor generic when using `createColumnHelper`
When defining columns with `createColumnHelper<T>()`, the resulting column array has type `ColumnDef<T, any>[]`. If the DataTable wrapper accepts `ColumnDef<T, unknown>[]`, TypeScript rejects the column helper output. Use `ColumnDef<T, any>[]` in the DataTable props interface to be compatible with both inline column definitions and `createColumnHelper` output. This is the standard pattern in TanStack Table docs.

### Tailwind CSS 4 `@theme` block registers design tokens for utility generation
In Tailwind CSS 4, simply defining CSS custom properties in `:root` is NOT enough to use them as utility classes (e.g., `bg-red-500`). You must ALSO declare them inside a `@theme { }` block with the `--color-*` / `--spacing-*` prefix pattern. Both are needed: `:root` for direct `var()` usage, `@theme` for Tailwind utility class generation. When adding new tokens, add to both sections.

### Tailwind CSS 4 + Vite requires `@tailwindcss/vite` plugin
Tailwind v4 does NOT work with just `@import "tailwindcss"` in your CSS file. You MUST install `@tailwindcss/vite` and register it in `vite.config.ts` as `tailwindcss()` (place it before `react()` in the plugins array). Without this plugin, all utility classes are silently ignored and the app renders completely unstyled — no errors, just raw HTML.

### Mock data JSON: enable `resolveJsonModule` in tsconfig for direct imports
TypeScript won't resolve `.json` imports without `"resolveJsonModule": true` in `tsconfig.app.json`. Since the prototype uses `import data from '@/data/foo.json'` extensively, this must be set in the base config. Without it, all TanStack Query hooks and property tests that import JSON will fail type-checking.

### fast-check `fc.date()` can produce invalid Date objects — use integer-to-ISO instead
`fc.date()` in fast-check v4 can generate date values that produce `"Invalid Date"` when calling `.toISOString()`, causing `RangeError` in property tests. For ISO timestamp fields, use the pattern:
```typescript
fc.integer({ min: 1577836800000, max: 1924905600000 }).map((ts) => new Date(ts).toISOString())
```
This guarantees valid date strings within a reasonable range (2020–2030).

### Vitest config: exclude `e2e/` directory explicitly
The default Vitest file discovery picks up `.spec.ts` files in the `e2e/` folder (Playwright tests), causing "Failed Suites" errors since they import Playwright APIs unavailable in Vitest. Add `exclude: ['e2e/**', 'node_modules/**']` to `vitest.config.ts` to prevent this. This applies whenever Playwright and Vitest coexist in the same frontend directory.

### FilterSelect component: tests must find `<select>` via DOM traversal, not `getByRole`
The project's `FilterSelect` component doesn't use standard `htmlFor`/`id` label-select association. Tests must find selects via DOM traversal from the label element:
```typescript
function getSelectByLabel(labelText: string): HTMLSelectElement {
  const label = screen.getByText(labelText, { selector: 'label' });
  const select = label.parentElement!.querySelector('select');
  return select as HTMLSelectElement;
}
```
This pattern is used in both `ReplenishmentScreen.test.tsx` and `ReconciliationScreen.test.tsx`.

### Page-enter animation: use `key={location.pathname}` for route-change re-trigger
The `.page-enter` CSS animation only plays once on mount. To re-trigger on navigation, add `key={location.pathname}` to the wrapper div in AppShell. This forces React to unmount/remount the wrapper on route change, replaying the animation. Without the key, the animation only fires on initial page load.

### Recharts in Vitest: mock `ResponsiveContainer` and polyfill `ResizeObserver`
Recharts relies on `ResizeObserver` and DOM layout measurements that JSDOM doesn't support. Any test file rendering a Recharts chart must:
1. Mock `recharts` to replace `ResponsiveContainer` with a simple div: `ResponsiveContainer: ({ children }) => <div style={{ width: 800, height: 280 }}>{children}</div>`
2. Polyfill `ResizeObserver`: `(globalThis as Record<string, unknown>).ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }`
Without both, tests either hang or throw. Use `(globalThis as Record<string, unknown>)` instead of `global.ResizeObserver =` to satisfy `verbatimModuleSyntax`.

### OKLCH color contrast verification: inline conversion in property tests
When verifying WCAG contrast ratios for OKLCH colors (no external lib needed), use the inline conversion pipeline: parse OKLCH string → convert to OKLab (polar→cartesian) → OKLab to linear sRGB (via LMS cube) → clamp → WCAG luminance formula. This avoids adding `culori` or similar dependencies for a test-only concern. The conversion code lives in `cash-flow.property.test.tsx` and can be extracted to a shared test utility if more features need it.

### Pattern: Prefer additive infrastructure over router-swap merges. When adding auth/RBAC/design-tokens to an existing working app, implement them as additive layers (Zustand store, route guard wrapper, CSS tokens file) rather than merging from a different codebase with an incompatible router. react-router-dom v7 supports guards via <RequireAuth> wrapper components — you get 90% of TanStack Router's protection benefits without a full rewrite. Migrate routers only as a deliberate, isolated refactor when features are stable.

### Frontend monorepo Docker pattern: `frontend/` is an orchestration root, not an app root
The `frontend/` directory contains a `docker-compose.yml` that orchestrates separate app containers. Each sub-app (`CompanyPortal-Vite/`, `VendorPortal-Vite/`) is fully self-contained with its own `Dockerfile`, `nginx.conf`, `package.json`, and `pnpm-lock.yaml`. They build independently — no shared node_modules or build step. When adding a new frontend app:
1. Create a new folder under `frontend/` with its own Dockerfile + nginx.conf
2. Add a service entry to `frontend/docker-compose.yml` with a unique port and image name
3. Image naming convention: `{appname}-vite-fe` (e.g., `userportal-vite-fe`, `vendorportal-vite-fe`)

### pnpm major upgrade + Docker: lockfile version mismatch breaks `--frozen-lockfile`
When upgrading pnpm across major versions (e.g., 9→11), the lockfile format changes (lockfileVersion `9.0` → `10.0`). pnpm 11 reads v9 lockfiles locally in backward-compat mode, but `--frozen-lockfile` in a fresh Docker container may reject the mismatch. Fix:
1. Run `pnpm install` locally to regenerate `pnpm-lock.yaml` at the new lockfile version
2. Commit the updated lockfile
3. Restore `--frozen-lockfile` in the Dockerfile

Also: `corepack prepare` and `corepack install --global` can both fail in `node:22-alpine` for pnpm 11.x. Use `npm install -g pnpm@<exact-version>` in Dockerfiles for reliable installs.

### pnpm 11 Docker builds: must COPY `pnpm-workspace.yaml` before `pnpm install`
pnpm 11 no longer reads config from `package.json#pnpm`. All settings (including `allowBuilds` for postinstall scripts) moved to `pnpm-workspace.yaml`. In Dockerfiles, the COPY line before `pnpm install` MUST include `pnpm-workspace.yaml`:
```dockerfile
COPY pnpm-lock.yaml package.json pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
```
Without it, pnpm sees no build allowlist and fails with `ERR_PNPM_IGNORED_BUILDS` for packages like `@biomejs/biome` and `esbuild`. The `pnpm.onlyBuiltDependencies` field in `package.json` does nothing in pnpm 11 — use `allowBuilds` in `pnpm-workspace.yaml` instead.

### Docker production build: use `tsc -b tsconfig.app.json`, not `tsc -b`
The root `tsconfig.json` references `tsconfig.app.json`, `tsconfig.node.json`, AND `tsconfig.test.json`. Running `tsc -b` (which the `pnpm build` script does via `tsc -b && vite build`) compiles all three — including test files that may have type errors irrelevant to production. In Docker, override the build command:
```dockerfile
RUN pnpm tsc -b tsconfig.app.json && pnpm vite build
```
This type-checks only production source code. Test type errors should be caught in CI test steps, not during Docker image builds.

---

## Notes

- Keep entries concise and actionable
- Remove patterns that are no longer relevant
- Update patterns as the project evolves
- Focus on what's unique to this project

### DB constraint evolution for dev/prod split: add a new enum value, don't relax existing ones
When a DB schema has strict CHECK constraints designed for production (e.g., `auth_source IN ('ldap', 'local')` with business rules like "internal users must be 'ldap'"), don't change existing rows to a different allowed value or loosen the constraint. Instead, add a new enum value (`'local_dev'`) with its own constraint branch. Benefits:
- Production constraint remains untouched and enforceable
- Migration to production is clear: stop using `local_dev`, switch to `ldap`
- Down migration is straightforward: revert the new value, restore original state
- No risk of accidentally shipping dev-mode data to production

Applied in: `user-login` spec, Requirement 1 — added `auth_source='local_dev'` rather than changing internal users to `auth_source='local'`.

### Spec task dependency graph format: use `{ id, tasks }` objects, not bare arrays
The Kiro task management tools expect the Task Dependency Graph `waves` field to be an array of objects with `{ "id": N, "tasks": ["1.1", "1.2", ...] }` — referencing sub-task IDs as strings. Bare arrays like `[["1"], ["2"]]` or numeric arrays like `[[1], [2]]` cause parse failures (`Cannot read properties of undefined (reading 'map')`). When creating tasks.md, always use the object format:
```json
{ "waves": [{ "id": 0, "tasks": ["1.1", "1.2"] }, { "id": 1, "tasks": ["1.3"] }] }
```
Compare against a working spec's tasks.md (e.g., `cms-app-foundation`) if unsure about format.

### RateLimiter interface in auth package for testability
Define a `RateLimiter` interface in `internal/auth/` (the consumer package) rather than importing the concrete struct from `internal/middleware/`. This decouples the auth service from Redis implementation details and allows unit testing with a stub that returns configurable errors. The concrete `middleware.RateLimiter` satisfies the interface without needing an explicit assertion — Go structural typing handles it.

### Separate docker-compose projects: nginx upstream must use `host.docker.internal`
Frontend and backend run in **separate** docker-compose projects (`frontend/docker-compose.yml` vs `backend/docker-compose.yml`). They have isolated networks even if both define `cms-net` — Docker prefixes the network name with the project name. Nginx `proxy_pass http://backend:8080` will fail with `host not found in upstream` because the `backend` hostname only exists in the backend's network.

Fix: use variable-based proxy_pass with Docker's internal DNS resolver to avoid crash-on-startup:
```nginx
location /api/ {
    resolver 127.0.0.11 valid=30s ipv6=off;
    set $backend_upstream http://host.docker.internal:8080;
    proxy_pass $backend_upstream;
    # ... headers
}
```
Key points:
- `resolver 127.0.0.11` = Docker's embedded DNS
- `set $variable` makes nginx resolve the upstream at request-time, not startup (prevents crash if backend is down)
- `host.docker.internal` routes to the host machine where backend exposes its port
- This applies to ALL frontend nginx.conf files in this project (CompanyPortal + VendorPortal)
