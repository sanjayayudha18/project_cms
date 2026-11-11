# Requirements Document: Company Portal Login Revamp

## Introduction

Revamp the login experience for the internal Company Portal at
`frontend/CompanyPortal-Vite/` using the visual direction in
`login_ui_design/crown-login-red.html` and `login_ui_design/CROWN IMG.png`.

The reference establishes the visual direction: a dark burgundy operations
interface, a split composition with identity and login on one side, and a
full-height ATM/cash-in-transit artwork panel on the other. The implementation
must adapt that direction to the Company Portal. It must retain the current
Company Portal identity, Indonesian login copy, username/password contract,
Zustand auth flow, and TanStack Router behavior. It must not turn the page into
the reference's separate vendor portal or copy its mock authentication logic.

## Source Adaptation Decisions

- Use the reference's dark burgundy palette, split layout, editorial headline,
  compact uppercase labels, input wells, subtle borders, and operational footer
  metadata.
- Use `CROWN IMG.png` as the local artwork source, with an accessible description
  of the ATM, cash-in-transit vehicle, cash containers, banknotes, and route
  diagram.
- Replace reference-only `CROWN`, `Vendor sign in`, `Corporate ID`, `Vendor
  portal`, and fake EOD values with Company Portal content.
- Keep the current fields `Username` and `Kata Sandi`; the backend contract
  accepts `username` and `password`, not a new corporate-ID field.
- Do not add reference-only trust-workstation, reset-password, or fake success
  behavior unless a separate product/API requirement is approved.
- Do not load the reference's remote ClickUp image or remote Google Fonts at
  runtime. Assets and typography must work when the Company Portal is served
  without external network access.

## Scope

### In Scope

- `/login` and the existing `_auth` layout in CompanyPortal-Vite.
- Local artwork integration and the reference-inspired dark visual system.
- Form interaction, validation, loading, errors, rate-limit feedback, and
  password visibility.
- Responsive layout, keyboard behavior, accessibility, and automated tests.

### Out of Scope

- Changes to backend authentication, token format, roles, or permissions.
- Password reset, MFA, SSO, LDAP, account recovery, or self-registration.
- Trust-device persistence or a new session-duration option.
- Changes to protected application pages or the Vendor Portal.

## Glossary

- **Company_Portal**: The internal React + Vite application at
  `frontend/CompanyPortal-Vite/`.
- **Login_Page**: The `/login` route rendered by
  `src/routes/_auth/login.tsx`.
- **Auth_Store**: The Zustand store at `src/lib/auth/store.ts` that submits
  credentials and owns in-memory authentication state.
- **Identity_Panel**: The left reference-inspired region containing the brand,
  product message, login form, and footer metadata.
- **Artwork_Panel**: The right region containing the local ATM/cash-management
  artwork and contextual caption.
- **Design_Tokens**: CSS custom properties in `src/styles/tokens.css` and the
  shared stylesheet.
- **Rate_Limit**: A rejected login attempt represented by HTTP 429 and an
  optional `Retry-After` value in seconds.

## Requirements

### Requirement 1: Reference-Inspired Composition

**User Story:** As a Company Portal user, I want the login screen to feel like
an intentional cash-operations product, so that I immediately understand the
system I am entering.

#### Acceptance Criteria

1. THE Login_Page SHALL render through the existing TanStack Router `/login`
   route and SHALL use the existing `_auth` layout boundary.
2. ON desktop-width viewports, THE page SHALL use a two-region composition with
   an Identity_Panel and an Artwork_Panel, with the Identity_Panel remaining
   the primary actionable region.
3. THE layout SHALL use a visual proportion close to the reference's
   `minmax(0, 1.08fr) minmax(0, 1fr)` split, while allowing the form region to
   remain usable at common laptop widths.
4. THE Identity_Panel SHALL use the reference's dark burgundy family and THE
   Artwork_Panel SHALL use a related darker artwork well with a subtle divider.
5. THE page SHALL not use a generic white card floating over a red gradient,
   decorative blobs, stock photography, or unrelated marketing sections.

### Requirement 2: Company Portal Identity and Copy

**User Story:** As an internal user, I want clear product identity and
purposeful operational copy, so that I know I am signing into the Company
Portal rather than the Vendor Portal.

#### Acceptance Criteria

1. THE brand lockup SHALL identify the product as `CMS` or `Cash Management
   System` and SHALL include an internal/company descriptor rather than
   `CROWN` or `Vendor sign in`.
2. THE page SHALL use a concise operations-focused headline and supporting
   copy inspired by the reference's “every rupiah accounted for” tone, with
   final language approved for the Company Portal.
3. THE page SHALL retain the current Indonesian form copy unless explicitly
   changed: `Selamat datang`, `Masuk untuk melanjutkan ke sistem manajemen kas`,
   `Username`, `Kata Sandi`, `Masuk`, and `Memproses...`.
4. THE page MAY show a short operational caption in the Artwork_Panel, but it
   SHALL describe CMS capabilities without claiming live metrics that are not
   supplied by an API.
5. THE page SHALL not hardcode fabricated production statistics, EOD timestamps,
   software versions, vendor counts, terminal counts, or operational status.
6. THE footer SHALL communicate that access is authorized and logged, if this
   wording is approved, and SHALL retain the existing `© 2026 CIMB Niaga STCC`
   attribution.

### Requirement 3: Artwork Asset

**User Story:** As a user, I want the visual panel to reflect the system's cash
operations domain, so that the design feels specific rather than templated.

#### Acceptance Criteria

1. THE Artwork_Panel SHALL use `login_ui_design/CROWN IMG.png` as the source
   artwork, copied or imported into the Company Portal's own asset pipeline.
2. THE implementation SHALL not depend on the remote image URL in the
   reference HTML.
3. THE artwork SHALL fill its panel using a stable crop with
   `object-fit: cover` or an equivalent responsive strategy, without stretching
   or obscuring the main ATM and cash-in-transit subjects at desktop widths.
4. THE image SHALL have meaningful alternative text when it conveys product
   context; purely decorative repetitions SHALL use `aria-hidden` instead.
5. THE Artwork_Panel SHALL provide a restrained bottom caption/scrim treatment
   only when needed for legibility and SHALL not cover the primary login form.
6. IF the artwork fails to load, THE page SHALL retain a coherent burgundy panel
   and caption rather than displaying a broken-image icon or empty white area.

### Requirement 4: Login Form Structure

**User Story:** As a daily operator, I want the login form to be compact and
easy to scan, so that I can sign in quickly.

#### Acceptance Criteria

1. THE form SHALL contain exactly one username field and one password field,
   with persistent visible labels `Username` and `Kata Sandi`.
2. THE labels SHALL follow the reference direction of compact, high-contrast,
   uppercase or small-caps-like metadata, while remaining readable and not
   relying on letter spacing alone.
3. THE username field SHALL use `type="text"`, `autoComplete="username"`, and
   a username-oriented placeholder.
4. THE password field SHALL use `type="password"` by default and
   `autoComplete="current-password"`.
5. THE password field SHALL provide an accessible show/hide control using an
   existing `lucide-react` eye icon or equivalent, while preserving the value,
   focus, and cursor context as far as the browser permits.
6. THE primary button SHALL be full-width within the form, have a minimum
   height of 44 CSS pixels, and display `Masuk` when idle.
7. THE form SHALL fit within a readable width of approximately 368 to 440
   pixels and SHALL not require horizontal scrolling.
8. THE implementation SHALL not add `Corporate ID`, `Trust this workstation`,
   or `Reset password` controls because those behaviors are not in the current
   Company Portal contract.

### Requirement 5: Validation and Authentication Behavior

**User Story:** As a user, I want the visual revamp to preserve the existing
login behavior, so that appearance changes do not interrupt access.

#### Acceptance Criteria

1. THE Login_Page SHALL reject empty or whitespace-only username and password
   values with the existing localized message `Wajib diisi`.
2. THE Login_Page SHALL enforce a maximum username length of 128 characters and
   a maximum password length of 72 characters at the form boundary.
3. WHEN client validation fails, THE page SHALL not call the authentication
   endpoint and SHALL focus the first invalid field when appropriate.
4. THE form SHALL submit via Enter from either credential field using the same
   handler as the primary button.
5. WHEN submitted, THE Login_Page SHALL call the existing
   `useAuthStore().login(username, password)` action and SHALL not duplicate
   token or API logic in the component.
6. THE Auth_Store SHALL continue sending `POST /api/v1/auth/login` with
   `X-Portal-Type: company` and `credentials: include`.
7. WHEN authentication succeeds, THE Login_Page SHALL navigate to `/` through
   TanStack Router without a full page reload.
8. THE implementation SHALL not store credentials or access tokens in
   localStorage, sessionStorage, or query parameters.
9. IF an authenticated user visits `/login`, THE application SHALL redirect to
   `/` without presenting an actionable login form.

### Requirement 6: Loading, Errors, and Rate Limits

**User Story:** As a user encountering a slow or failed service, I want clear
and recoverable feedback, so that I do not submit duplicate requests or lose my
place.

#### Acceptance Criteria

1. WHILE a login request is pending, THE page SHALL disable the submit button,
   prevent duplicate submissions, show a visible spinner, and display
   `Memproses...`.
2. WHILE loading, THE page SHALL retain the entered username and password values
   and SHALL expose an appropriate busy/status signal to assistive technology.
3. WHEN authentication fails, THE page SHALL show an inline `role="alert"`
   region near the form and SHALL not expose server internals.
4. Invalid credentials SHALL continue to display `Username atau password salah`
   without identifying whether the username or password was incorrect.
5. The page SHALL preserve the existing user-facing handling for inactive
   accounts, portal mismatch, service unavailable, and connection failure.
6. WHEN the Auth_Store reports a Rate_Limit, THE page SHALL display
   `Terlalu banyak percobaan login` and, when available, the remaining duration
   as `M menit S detik`.
7. WHILE a rate-limit countdown is active, THE submit action SHALL be disabled;
   it SHALL become available again at zero without a full page reload.
8. The countdown SHALL never show negative values and SHALL derive elapsed time
   from a stable deadline rather than accumulating timer drift.
9. AFTER any error, THE user SHALL be able to edit the fields and retry without
   refreshing the page.
10. THE current redirect-on-success behavior SHALL remain intact; a decorative
    “Signed in” state SHALL not delay navigation to `/`.

### Requirement 7: Dark Visual System

**User Story:** As a product owner, I want the page to match the supplied
reference without breaking the project's design-token discipline, so that the
revamp is distinctive and maintainable.

#### Acceptance Criteria

1. THE implementation SHALL define the reference-inspired dark tokens in
   `src/styles/tokens.css` or the established stylesheet using OKLCH values,
   including deep burgundy chrome, form well, hover well, border, muted text,
   primary red, and semantic feedback colors.
2. THE visual system SHALL approximate the reference values: deep burgundy near
   `oklch(30% 0.11 25)`, chrome near `oklch(40% 0.155 26)`, form well near
   `oklch(24.5% 0.088 25)`, and primary red near `oklch(56% 0.223 27)`.
3. THE implementation SHALL not introduce arbitrary hex color literals or
   hardcoded one-off colors in JSX.
4. Inputs SHALL use dark wells, subtle borders, readable placeholders, and a
   high-contrast focus ring consistent with the reference.
5. The primary action SHALL use the CIMB red token, have a distinct hover state,
   and have a visibly different disabled/loading state.
6. Error, warning, informational, and success feedback SHALL pair color with
   visible text and/or an icon; status SHALL never be communicated by color
   alone.
7. Typography SHALL use one sans-serif family with multiple weights; a
   monospace token MAY be used for operational metadata only.
8. THE implementation SHALL reuse existing dependencies and `lucide-react`
   icons; no new package SHALL be added for visual effects or fonts.

### Requirement 8: Responsive and Motion Behavior

**User Story:** As a user on a laptop, tablet, or mobile device, I want the
login experience to remain complete and comfortable at every size.

#### Acceptance Criteria

1. ON desktop widths, THE Artwork_Panel SHALL remain visible as a full-height
   panel and THE Identity_Panel SHALL remain vertically usable within the first
   viewport.
2. BELOW the desktop breakpoint, THE layout SHALL stack the artwork above the
   form or reduce it to a compact visual region, following the reference's
   responsive intent.
3. ON narrow mobile widths, THE artwork caption MAY be reduced, but the product
   identity, labels, fields, errors, submit button, and footer SHALL remain
   available without clipping.
4. THE page SHALL allow vertical scrolling on short viewports and SHALL use
   safe horizontal padding; no content SHALL overflow the viewport.
5. ALL interactive controls SHALL have a minimum target size of 44 by 44 CSS
   pixels where practical, including the password visibility control.
6. Artwork loading MAY use a subtle skeleton and fade-in inspired by the
   reference, but the page SHALL remain usable before the image finishes
   loading.
7. WHEN `prefers-reduced-motion: reduce` is enabled, THE implementation SHALL
   minimize or remove artwork fades, skeleton animation, spinner motion where
   possible, and decorative transitions.

### Requirement 9: Accessibility and Security

**User Story:** As a keyboard or assistive-technology user, I want the login
flow to be understandable and operable, so that authentication does not
depend on sight or a mouse.

#### Acceptance Criteria

1. EVERY input SHALL have a unique `id`, a visible associated label via
   `htmlFor`, and an appropriate autocomplete value.
2. FIELD errors SHALL be connected with `aria-describedby` and SHALL set
   `aria-invalid="true"` when present.
3. Login errors and rate-limit messages SHALL use an accessible alert region
   and SHALL remain understandable without color.
4. The password visibility control SHALL expose its current action via an
   accessible name and `aria-pressed` or equivalent state.
5. ALL interactive controls SHALL be reachable in logical Tab order and
   activatable with keyboard input.
6. Focus indicators SHALL be visible against both the burgundy wells and the
   primary button in idle, hover, disabled, and error states.
7. The artwork SHALL include an appropriate `alt` description or be explicitly
   hidden from assistive technology if the final product direction treats it as
   decorative.
8. THE page SHALL not persist, log, or expose plaintext passwords, access
   tokens, or authentication secrets.

### Requirement 10: Verification and Acceptance

**User Story:** As the product team, I want the reference-inspired page tested
against real Company Portal behavior, so that the visual change does not create
regressions.

#### Acceptance Criteria

1. Existing login tests SHALL continue to pass, with changes only for
   intentional new interactions or accessible labels.
2. Tests SHALL cover username/password rendering, required-field validation,
   whitespace-only rejection, invalid credentials, successful navigation,
   loading/disabled behavior, alert semantics, and password visibility.
3. Tests SHALL cover rate-limit rendering and countdown behavior when the
   implementation exposes the existing `rateLimitRetryAfter` state.
4. Tests SHALL verify that the login request contract remains unchanged,
   including the `company` portal type header.
5. The Company Portal SHALL pass `pnpm test` and `pnpm build` from
   `frontend/CompanyPortal-Vite/`.
6. Manual verification SHALL cover desktop split layout, stacked tablet layout,
   narrow mobile layout, keyboard-only use, image-load failure, and reduced
   motion.
7. The final implementation SHALL not change protected-route behavior or
   introduce a second auth shell.

## Design Direction Summary

- **Purpose**: fast, trustworthy entry to an internal cash-operations system.
- **Audience**: CIMB Niaga employees and authorized internal operators.
- **Tone**: dark, precise, technical, and operational; not generic SaaS or
  marketing-led.
- **Memorable detail**: the ATM and cash-in-transit line artwork creates a
  specific visual relationship between the login screen and the CMS domain.
- **Primary constraint**: adapt the supplied visual reference while preserving
  the existing Company Portal auth contract and localized behavior.
