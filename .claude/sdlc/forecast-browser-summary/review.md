# forecast-browser-summary — Review (stage 5)

Reviewed 2026-09-30 by Claude (inline review of the full diff: bugs, security, CLAUDE.md compliance, data checked on dev DB).
Separate reviewer agents (code-reviewer / database-reviewer) not run — available on request.
Status: **R1 fixed (option A, 2026-09-30)**; R2 fixed; no open Important; no CRITICAL. Code owner merges; AI never self-approves.

## Important
| # | Finding | Evidence | Status |
|---|---|---|---|
| R1 | **Vendor branch without region → recap row can't be drilled into exactly.** A group `(V, region NULL)` is shown with region "-"; clicking it sets `flm_vendor=V` and region = Semua, so the detail table shows **all** of V's regions (superset — nothing is hidden, but AC3 "ATM count in detail = recap" fails for that row). The region dropdown also has no way to pick "no region". | Dev DB: **51 of 422** `vendor_branches.region` IS NULL (Abacus, Advantage, Brinks, Kejar, Prosegur, …). | **Fixed 2026-09-30 (option A, released by user "lanjut R1")** — `no_region=true` on `GET /forecast` (List + Count: `v.id IS NOT NULL AND vb.region IS NULL`; 400 with `flm_vendor_region`/`unassigned`); recap groups region by `LOWER` (shown `MIN`); UI row "(tanpa region)" sends `vendor + no_region`, chip "Tanpa region ✕", flag dropped when vendor/region select changes. Tests: service validation, handler flag parse, integration `TestIntegration_SummarizeForecastForDate_RegionEdges`, frontend recap/browser tests. |
| R2 | Recap showed the previous date's counts while the new date loaded (`keepPreviousData`) — an operator could read old numbers as the new date's. | `hooks.ts` `useForecastSummary` | **Fixed** — placeholder removed; skeleton shows while loading. Tests/lint green. |

**R1 options**
- **A (recommended):** make "no region" a real filter. Add optional `no_region=true` to `GET /forecast` (like `unassigned`) → `vb.region IS NULL`, mutually exclusive with `flm_vendor_region`. Recap row click sends it; chip "Tanpa region ✕". One predicate in List + Count, one param, tests. No schema change.
- **B:** fix the data — backfill `vendor_branches.region` for the 51 rows (master data via maker-checker, Sec 12), code unchanged. Right long-term, outside this feature; R1 persists until done.
- **C:** accept the superset and label the row "(tanpa region — detail menampilkan semua region vendor ini)". Frontend only; AC3 relaxed for these rows.

## Nit / minor (not fixed, noted)
- ~~Region case variants split into separate recap rows~~ — **fixed with R1** (grouping by `LOWER(region)`, integration-tested).
- Recap groups by vendor **name** — same key the existing `flm_vendor` filter uses; two vendors with an identical name would merge. No known duplicates.
- Select-all 1000-item cap counts already-requested rows too — conservative, never exceeds the server cap.
- Create still accepts an item already in another open request (out of scope per spec; UI badge only).
- `.kiro/specs/done/cit-vendor-request-enhancements` still states Req 1.4 "vendor + region required" — superseded by this feature's `spec.md` FR2; add a pointer there when committing.
- Pre-existing, outside this diff: `internal/handler/admin_region_handler.go` is not gofmt-clean.

## Security pass
- New `GET /forecast/summary`: same `vendorRequestViewerRoles` as `/forecast` (401/403 tested). Returns aggregates of data those roles can already browse row by row — no access widened (intent OQ3: no branch scoping today).
- Input: `forecast_date` format + `validateDateBound`; `unassigned` / `no_region` strict (`""|false|true`, else 400, shared `parseBoolFlag`); filters length-bound 255. All SQL via sqlc parameters.
- No secrets, no request-data logging, no new env vars. Frontend renders text only.

## Bugs / correctness pass
- Count vs List vendor resolution now identical (tie-breaker added to Count) — integration test (f).
- Recap ↔ detail invariants, cancelled/rejected semantics, multi-denom "all requested" rule — integration tests (a)–(e) green on real Postgres.
- Money: `SUM(bigint)::bigint` → `int64` → JSON number; UI `formatIDR` + `tabular-nums`, "Rp" explicit. No float math.
- Frontend: rows without vendor / already requested can't be selected (row, header checkbox, select-all); mixed-vendor selection blocks Create with a visible, non-colour-only message.

## Coverage
New code: service `ForecastSummary` 91.3%, handler `ForecastSummary` + response mappers 100%, service `BrowseForecast` 75% (unchanged error branches). Package totals (`service` 44%, `handler` 64.7%) were below 80% **before** this change — not regressed, not fixed here.

## Definition of Done (CLAUDE.md Sec 11)
- [x] Matches requirement + module/table map — no new tables/columns/migrations.
- [x] Auth path unchanged (LDAP internal); RBAC at route; no role change.
- [x] Maker-checker + audit — N/A (read-only feature, no state change).
- [~] Reads on replica — **deviation D1 (approved 2026-09-30):** primary, for read-after-write after Create.
- [x] Money bigint, no float; timestamps untouched.
- [x] Tests passing incl. RBAC + money aggregation (`tests.md`).
- [x] No secrets/config hardcoded; no new env.
- [x] Response shape: flat JSON matching neighbouring ATM handlers.
- [ ] Builds cleanly in Docker — **not run** (`go build` + `pnpm build` green).
- [ ] Manual browser check — **outstanding (user)**, checklist in `tests.md`.

## Gates
1. R1: done 2026-09-30 (option A) — all tests green (`tests.md`).
2. User manual browser check (`tests.md` AC1–AC6).
3. Commit (one conventional feature commit) → code owner merge.
