# Implementation Plan: CMS App Foundation

## Overview

Build the foundational UI shell for the CMS application using React 19 + TypeScript + Vite 6 with TanStack Router, TanStack Query, Zustand, and Tailwind CSS 4. Implementation follows an outside-in approach: design tokens → layout shell → auth → protected routes → dashboard → module landings → DSR upload flow → error/loading states → accessibility.

## Tasks

- [x] 1. Project scaffolding and design token system
  - [x] 1.1 Initialize Vite project with React 19, TypeScript 5, Tailwind CSS 4, and install core dependencies
    - Initialize `frontend/` with Vite React-TS template
    - Install: `@tanstack/react-router`, `@tanstack/react-query`, `zustand`, `tailwindcss@4`, `lucide-react`, `zod`, `react-hook-form`, `xlsx` (SheetJS)
    - Install dev: `vitest`, `@testing-library/react`, `fast-check`, `@playwright/test`, `@biomejs/biome`
    - Configure `vite.config.ts`, `tsconfig.json`, `biome.json`
    - Set up `pnpm test` script with Vitest config
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [x] 1.2 Create OKLCH design token CSS custom properties and base styles
    - Create `src/styles/tokens.css` with full OKLCH color scale (brand red hue 29, neutrals hue 29, semantic colors)
    - Create `src/styles/index.css` with Tailwind directives, base styles, `tabular-nums` utility, system font stack
    - Define spacing tokens (`--space-1` through `--space-18`), radius tokens, shadow tokens
    - Set `html[lang="id"]` in `index.html`
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 10.5_

  - [x] 1.3 Create utility functions: `formatIDR`, `formatDate`, `cn` class merger
    - Implement `src/lib/utils/format.ts` with `formatIDR(value: number)` returning `Rp` + dot-separated thousands
    - Implement `src/lib/utils/cn.ts` using `clsx` + `tailwind-merge` pattern
    - Implement `src/lib/utils/date.ts` for Indonesian locale date formatting
    - _Requirements: 7.6, 4.3_

  - [x] 1.4 Write property test for IDR currency formatting (Property 6)
    - **Property 6: IDR Currency Formatting**
    - **Validates: Requirements 4.3, 7.6**

- [x] 2. Core layout components (App Shell)
  - [x] 2.1 Implement AppShell layout component with CSS Grid
    - Create `src/components/layout/AppShell.tsx` with fixed sidebar + header + scrollable main
    - Use CSS Grid: `grid-template-columns: auto 1fr`, `grid-template-rows: auto 1fr`
    - Manage sidebar collapsed/expanded state via local state
    - Responsive: collapse sidebar to icon-only at `< 1024px`
    - _Requirements: 1.1, 1.6_

  - [x] 2.2 Implement Sidebar component with role-based navigation filtering
    - Create `src/components/layout/Sidebar.tsx` with `NavItem` rendering
    - Create `src/lib/config/navigation.ts` with full `NAV_CONFIG` array (all nav items, groups, roles, disabled states)
    - Implement `filterNavByRoles(items: NavItem[], roles: Role[])` utility
    - Highlight active route with brand accent `--red-500`
    - Collapse animation: 200ms ease-out, icon-only with tooltip on hover
    - Keyboard navigation: arrow keys traverse, Enter/Space activate
    - Group items by section with uppercase `--n-500` headers
    - All labels in Bahasa Indonesia
    - _Requirements: 1.2, 1.3, 1.5, 1.6, 1.8, 3.1, 3.2, 3.3, 3.4, 3.5, 3.7, 10.2_

  - [x] 2.3 Implement Header component with user info and logout
    - Create `src/components/layout/Header.tsx` displaying user full name, role badge, logout button
    - Mobile: hamburger menu button for sidebar toggle
    - All labels in Bahasa Indonesia
    - _Requirements: 1.7, 1.8_

  - [x] 2.4 Write property tests for navigation filtering and active route (Properties 1, 2)
    - **Property 1: RBAC Navigation Filtering**
    - **Property 2: Active Route Highlighting**
    - **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.7, 1.5**

  - [x] 2.5 Write property test for Header user info display (Property 8)
    - **Property 8: Header User Info Display**
    - **Validates: Requirements 1.7**

- [x] 3. Authentication and session management
  - [x] 3.1 Implement Zustand auth store with login, logout, refresh, and initialize actions
    - Create `src/lib/auth/store.ts` with `AuthState` + `AuthActions`
    - Store access token in memory (not localStorage) for XSS resistance
    - Implement `login()`: POST credentials → store JWT + user
    - Implement `logout()`: clear state, redirect to `/login`
    - Implement `refreshToken()`: use httpOnly cookie refresh endpoint
    - Implement `initialize()`: check existing session on app boot
    - Define `Role` type and `ROLE_NAV_PERMISSIONS` map
    - _Requirements: 2.1, 2.3, 2.5, 2.6, 2.7_

  - [x] 3.2 Implement Login page with form validation
    - Create `src/routes/_auth/login.tsx` with email + password form
    - Use React Hook Form + Zod for validation
    - Display generic inline error on failure (no field-specific hints)
    - Redirect to dashboard on success
    - _Requirements: 2.2, 2.4_

  - [x] 3.3 Write property test for login error message safety (Property 5)
    - **Property 5: Login Error Message Safety**
    - **Validates: Requirements 2.4**

- [x] 4. Routing and route protection
  - [x] 4.1 Set up TanStack Router file-based route tree
    - Create `src/routes/__root.tsx` with QueryClientProvider, ToastProvider
    - Create `src/routes/_auth.tsx` as auth layout (centered card)
    - Create `src/routes/_protected.tsx` as protected layout wrapping AppShell + route guard
    - Create `src/main.tsx` entry point with `createRouter` and render
    - Implement `beforeLoad` hook for auth check + RBAC validation
    - Create unauthorized page ("Akses Tidak Diizinkan" + "Kembali ke Beranda" link)
    - _Requirements: 1.3, 1.4, 2.1, 3.6_

  - [x] 4.2 Write property tests for route protection (Properties 3, 4)
    - **Property 3: Unauthenticated Route Protection**
    - **Property 4: Unauthorized Route Blocking**
    - **Validates: Requirements 2.1, 3.6**

- [x] 5. Checkpoint - Ensure shell and auth work
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. API client with stub layer
  - [x] 6.1 Implement API client with auth header injection and stub/real mode switching
    - Create `src/lib/api/config.ts` resolving `VITE_API_MODE` env var (`stub` | `real`)
    - Create `src/lib/api/client.ts` as fetch wrapper with request/response interceptors
    - Implement request interceptor: inject `Authorization: Bearer {token}` header
    - Implement response interceptor: 401 → attempt refresh → retry once → clear session on second 401
    - _Requirements: 8.1, 8.3, 8.4, 8.5, 8.6_

  - [x] 6.2 Implement stub API interceptor with mock data and simulated latency
    - Create `src/lib/api/stubs/` directory with mock data factories per endpoint
    - Implement stub interceptor returning mock data with random delay (200–800ms)
    - Create stubs for: `/dashboard/metrics`, `/dashboard/activity`, `/forecasting/dsr/uploads`, `/auth/login`, `/auth/refresh`
    - _Requirements: 8.2_

  - [x] 6.3 Write property tests for stub latency range and auth header injection (Properties 17, 18)
    - **Property 17: Stub API Latency Range**
    - **Property 18: Auth Header Injection**
    - **Validates: Requirements 8.2, 8.4**

- [x] 7. Error handling and loading states
  - [x] 7.1 Implement Toast system with auto-dismiss and persistence rules
    - Create `src/components/ui/Toast.tsx` and `src/lib/hooks/useToast.ts`
    - Success toasts auto-dismiss after 5s; error toasts persist until dismissed
    - Toast container with `aria-live="polite"` for screen reader announcements
    - Position: top-right, stack vertically, max 5 visible
    - _Requirements: 9.4, 9.5, 10.6_

  - [x] 7.2 Implement ErrorBoundary and loading skeleton components
    - Create `src/components/feedback/ErrorBoundary.tsx` with fallback UI ("Terjadi Kesalahan" + reload button)
    - Create `src/components/feedback/Skeleton.tsx` for content-shaped loading states
    - Create `src/components/feedback/NetworkError.tsx` for connection errors + retry
    - Implement server error handling: generic message, no technical details exposed
    - _Requirements: 9.1, 9.2, 9.3, 9.6_

  - [x] 7.3 Write property test for server error message sanitization (Property 19)
    - **Property 19: Server Error Message Sanitization**
    - **Validates: Requirements 9.3**

- [x] 8. Dashboard landing page
  - [x] 8.1 Implement Dashboard page with metric cards and activity feed
    - Create `src/routes/_protected/index.tsx` (Dashboard)
    - Create `src/features/dashboard/components/MetricCard.tsx` with tabular-nums, right-aligned IDR
    - Create `src/features/dashboard/components/ActivityFeed.tsx` showing last 10 events
    - Use TanStack Query hooks to fetch from stub API
    - On API failure: hide metrics section, show retry button + error message
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_

  - [x] 8.2 Write property test for dashboard metrics completeness (Property 7)
    - **Property 7: Dashboard Metrics Completeness**
    - **Validates: Requirements 4.2**

- [x] 9. Module landing pages
  - [x] 9.1 Implement Forecasting, Invoice, and Cash Count module landing pages
    - Create `src/routes/_protected/forecasting/index.tsx` with 6 navigation cards (DSR Receipt, Fill Instruction, Fill Validation, Cash Supply, H+2 Projection, Holiday Calendar)
    - Create `src/routes/_protected/invoice/index.tsx` with 4 navigation cards (Invoice Upload, Reconciliation, Charge Calculation, Document Generation)
    - Create `src/routes/_protected/cash-count/index.tsx` with 6 navigation cards (Scheduling, Balance Tier Analysis, Execution, Checklists, Reconciliation, Recapitulation)
    - Create shared `src/components/ui/ModuleCard.tsx` component
    - Disabled cards: reduced opacity, no click handler, "Segera Hadir" badge overlay
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

  - [x] 9.2 Write property tests for module card rendering and disabled state (Properties 9, 10)
    - **Property 9: Module Landing Card Rendering**
    - **Property 10: Disabled Card "Segera Hadir" Label**
    - **Validates: Requirements 5.2, 5.6**

- [x] 10. Checkpoint - Ensure dashboard and module pages work
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. DSR Upload flow (end-to-end)
  - [x] 11.1 Implement DSR file upload interface with validation
    - Create `src/routes/_protected/forecasting/dsr-upload.tsx` route page
    - Create `src/features/forecasting/components/DSRUploadForm.tsx` with file input accepting `.xlsx`, `.xls`
    - Implement file size validation (max 10MB) with descriptive error
    - Implement client-side Excel parsing using SheetJS
    - Implement column validation against `REQUIRED_DSR_COLUMNS`
    - On invalid columns: reject at preview step with message identifying missing/unexpected columns
    - _Requirements: 6.1, 6.2, 6.8_

  - [x] 11.2 Implement DSR preview table and submission flow
    - Create `src/features/forecasting/components/DSRPreviewTable.tsx` showing first 20 rows with column headers
    - Display vault balance, vendor fill plan, reconciliation result, shortage claim columns
    - On confirm: POST parsed data to `/api/v1/forecasting/dsr` via API client
    - On success: display success toast with upload timestamp + record count
    - On validation errors: display error summary listing row number + field + message
    - Implement state machine: idle → preview → submitting → success/error
    - _Requirements: 6.3, 6.4, 6.5, 6.6, 6.7_

  - [x] 11.3 Implement DSR upload history table
    - Create `src/features/forecasting/components/UploadHistory.tsx`
    - Display most recent 30 uploads with date, filename, row count, status
    - Use TanStack Table for sortable columns
    - Render below the upload form
    - _Requirements: 6.9_

  - [x] 11.4 Write property tests for DSR validation and display (Properties 11, 12, 13, 14, 15, 16)
    - **Property 11: DSR File Size Validation**
    - **Property 12: DSR Column Validation**
    - **Property 13: DSR Preview Row Limit**
    - **Property 14: DSR Upload Success Toast Content**
    - **Property 15: DSR Validation Error Display Completeness**
    - **Property 16: DSR Upload History Limit**
    - **Validates: Requirements 6.2, 6.3, 6.6, 6.7, 6.8, 6.9**

- [x] 12. Accessibility foundation
  - [x] 12.1 Implement keyboard navigation, focus management, and ARIA attributes
    - Add focus management on route change (move focus to main content)
    - Ensure all icon-only buttons have `aria-label` attributes
    - Verify `document.documentElement.lang = "id"` is set
    - Add `aria-current="page"` to active sidebar nav item
    - Verify contrast ratios in design tokens meet WCAG 4.5:1 (normal) / 3:1 (large)
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

  - [x] 12.2 Write property test for icon button accessibility labels (Property 20)
    - **Property 20: Icon Button Accessibility Labels**
    - **Validates: Requirements 10.3**

  - [x] 12.3 Write property test for design system contrast compliance (Property 21)
    - **Property 21: Design System Contrast Compliance**
    - **Validates: Requirements 10.4**

- [x] 13. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design doc using fast-check
- Unit tests validate specific examples and edge cases
- All UI labels and error messages are in Bahasa Indonesia
- The stub API layer is controlled by `VITE_API_MODE` env var — defaults to `stub`
- TypeScript is the implementation language (as specified in the design document)

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3"] },
    { "id": 2, "tasks": ["1.4", "2.1", "3.1"] },
    { "id": 3, "tasks": ["2.2", "2.3", "3.2", "4.1"] },
    { "id": 4, "tasks": ["2.4", "2.5", "3.3", "4.2"] },
    { "id": 5, "tasks": ["6.1", "6.2", "7.1", "7.2"] },
    { "id": 6, "tasks": ["6.3", "7.3", "8.1", "9.1"] },
    { "id": 7, "tasks": ["8.2", "9.2"] },
    { "id": 8, "tasks": ["11.1"] },
    { "id": 9, "tasks": ["11.2", "11.3"] },
    { "id": 10, "tasks": ["11.4", "12.1"] },
    { "id": 11, "tasks": ["12.2", "12.3"] }
  ]
}
```
