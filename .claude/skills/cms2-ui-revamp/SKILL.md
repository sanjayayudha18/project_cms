---
name: cms2-ui-revamp
description: Start or advance a CMS2 UI Revamp — redesign of an existing screen in CompanyPortal-Vite or VendorPortal-Vite (layout, table/filter/modal overhaul, 4 UI states, brand alignment, responsive, a11y) through audit.md → design artifact (design.md) → ui-spec.md → tasks.md under .claude/ui-revamps/<screen>/, stopping at every human gate. Use when the user says "revamp UI", "UI revamp", "redesain layar", "rombak tampilan", or picks UI Revamp at CLAUDE.md §4a rule 0.
---

# CMS2 UI Revamp driver

Source of truth: CLAUDE.md §4a (UI Revamp) + §13, `.claude/rules/frontend-ui.md`
(brand, auto-loaded for `frontend/**`), `.kiro/steering/ui_design.md` (tokens).
This skill adds **order + which skill per step**. If they disagree: CLAUDE.md wins,
then frontend-ui.md, then ui_design.md, then any design skill's suggestion.

## Step 0 — every time

1. Method chosen? If not, ask CLAUDE.md §4a rule 0 with your suggestion. Wait.
2. **Fit check** — UI Revamp only if ALL hold, else suggest Kiro/ScDD instead:
   - existing screen(s) in one portal; no new business rule;
   - no schema/migration, no new or changed endpoint (`backend/api/openapi.yaml` unchanged);
   - nothing in §4 #7 (auth, money math, recon, delete) — money *display* is fine.
   A finding mid-revamp that needs backend/API → stage `blocked`, ask; it becomes its own Kiro/ScDD item.
3. Inputs: screen name (kebab-case folder), portal, route/feature folder. Missing → ask.
4. Folder `.claude/ui-revamps/<screen>/` exists? Read every file's `> Status:` and
   continue from the first unfinished gate. Code vs file disagree → STOP, ask.
5. Read the screen's code end to end first (`graphify query "<screen>"`, then the
   feature folder, its hooks, tests) — never audit from screenshots alone.

## Stages

| # | Artifact | Content | Gate (user) | Skills |
|---|---|---|---|---|
| 1 | `audit.md` | Current layout + component tree + data flow; friction points (P1…); brand check (Merah Sirih ≤10% red internal / Merah Menyala vendor; brand red vs danger red); **4 UI states** (loading skeleton, empty, error + retry, maker-checker 202 pending); money display (IDR explicit, right-aligned, `tabular-nums`, no float math in JS); a11y (contrast, focus, keyboard, status not by colour alone); copy | Audit accepted | `design:design-critique`, `design:accessibility-review` / `ecc:accessibility`, `design:ux-copy`, `ui-ux-pro-max` (review mode) |
| 2 | Design artifact + `design.md` | Mockup on the **"Design" Artifact type** (`Artifact` quickstart `intent: "design"` → publish with its `type_url`; follow the type's instructions). Artboards: current vs proposed, each P# addressed, 4 UI states, desktop 1280 + 1024 (+ 768 if used). Build on the **CMS2 UI/UX Design Document** artifact (a Design System type) (https://claude.ai/artifact/51N3yMA3JbsKuSAJ7rm6ST — read its `project/README.md` first; theme `sirih` for CompanyPortal, `menyala` for VendorPortal). Token changes go to the repo first (`tokens.css` / `ui_design.md`), then the design system. Realistic dummy data (IDR amounts, terminal IDs), never real records. `design.md` = artifact link + artboard → P# map + open questions. User comments on the artifact → revise + republish same URL | Design accepted | `ui-ux-pro-max` (mandatory), `artifact-design`, `ui-styling`, `ecc:make-interfaces-feel-better`, `design:design-critique` (self-check before showing) |
| 3 | `ui-spec.md` | Turns the accepted design into build rules: goals mapped to P#; component list (reuse existing `src/components` first, new ones named); tokens used (existing CSS vars only — new token = flag); every interactive state; 4 UI states per section; breakpoints; copy table; out of scope. Every section points to an artboard | Spec accepted | `ui-ux-pro-max`, `ecc:design-system`, `design:design-handoff` |
| 4 | `tasks.md` | Waves: W1 atoms/molecules → W2 organisms (table, toolbar, dialog) → W3 page integration (hooks, states, toasts) → W4 tests. Each task: files (exclusive per task), `_Spec:_ S#`, `_Model:_`, `_Skills:_`. `### Task Dependency Graph` | Tasks accepted | `mattpocock-skills:codebase-design` (only if splitting components) |
| 5 | build → `tasks.md` `## Test Results` | Execute per wave (`claude-cms-orchestrator` / `cms-task-runner`, or inline when small). Tests first for behaviour that changes. Run `pnpm --dir frontend/<App> run test`, `lint`, `build`. Manual browser check = `outstanding (user)` | Build + tests green | `ui-ux-pro-max` before any UI code, `ecc:react-patterns`, `ecc:react-testing`, `ecc:frontend-a11y`, `ecc:verification-loop` |
| 6 | `tasks.md` `## Review` | R1… Important vs nit, brand + a11y + 4-states re-check vs spec and design artboards, DoD §11 | Code owner merges → `done <date> <sha>`, move folder to `.claude/ui-revamps/done/<screen>/` | `/code-review`, `ecc:react-review` |

Each file starts with `# <Title>` then `> Status: draft`. One stage file at a time.

## Gates — never self-approve

- Claude never writes `accepted` or `done`. Ask, then wait.
- No commit unless the user says "commit"; then report the SHA.
- Mockups/screenshots are proposals; the user does the browser check (Golden Rule #10).

## At every gate

1. Update the screen's row in `.claude/development-progress.md`.
2. Lessons → `.claude/lessons-learned.md`.
3. After code changes: `graphify update .`

## Don't

- `hallmark` or other skills that impose a new visual style — brand rules are fixed.
- `ui-craft` as the design driver for the mockup — the user preferred the `ui-ux-pro-max` result (2026-11-20, Settings Hub trial). `ui-craft` review agents may still be used for a critique pass if asked.
- New npm deps without approval (§0 GR#1); new colours outside the tokens.
- Change API calls, response handling semantics, RBAC checks or maker-checker flow.
- Mix two screens in one folder, or revamp both portals in one folder.
