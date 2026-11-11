# Assessment — Cash Management: Cash Count, Monitoring & Balance Recap

> Development-readiness assessment for the **Cash Count** module (`internal/cashcount`) and its two companion concerns — **monitoring** (daily progress + monthly report) and **balance recap** (3-way reconciliation + vendor performance).
> Status: draft assessment · Date: 2026-09-28 · Target phase: **Phase 5** (`development-plan.md`, currently parked).
> Sources: URS v0.3 Phase 1 (FNC 002 + FNC 003), `feature-flows/08-cash-count-vault`, `feature-flows/09-cash-count-selektif-mesin`, `project-context.md`, `development-plan.md`, `rbac-role-feature-matrix.md`, and inspection of the actual code.

---

## 1. Executive Summary

This is the **Cash Count** function (URS FNC 002) plus the slice of the **Dashboard** function (FNC 003) that reports on it. It splits into three concerns that share one lifecycle:

- **Cash count** — monthly, risk-driven physical count of vendor vaults (ATM + Cash + Valas) and selective ATM machines: random non-patterned scheduling → PIC assignment/accept → surat tugas → digital Berita Acara (BA) + checklist + photo + dual e-sign → final document.
- **Monitoring** — daily progress of in-flight counts and a monthly report, with per-escrow status (`Complete` / `On-progress` / `Not Complete`) and findings, surfaced on the dashboard for ATM Support & Monitoring and Vendor Operations & Control.
- **Balance recap** — the management rollup: 3-way reconciliation (cash count vs escrow H-1 vs manual proofing), the DSR auto-fill comparison, checklist summary, balance-tier categorization (High/Medium/Low from escrow analysis), and vendor performance evaluation per period.

**Current state:** the module is **not built**. Backend `internal/cashcount` does not exist (`backend/internal/` holds only approval, audit, auth, db, handler, repository, rolemgmt, service). No cash-count tables are migrated (`cash_count_schedules` / `cash_count_evidences` names are approved-as-proposal only; migrations stop at `018`, next free `019`). The frontend has a real route `/cash-count` (`routes/cash-count/index.tsx`) but it renders six **disabled** module cards (Penjadwalan, Analisis Tier Saldo, Pelaksanaan (BA), Checklist, Rekonsiliasi, Rekapitulasi) — no `features/cash-count` folder, no API, no data. Nav entries and cash-count roles exist in the RBAC matrix. What is real: the requirements, two feature-flow diagrams, the roadmap item, the route shell, and the RBAC wiring.

**Readiness to start:** ⚠️ **Not ready to work on now.** Per `development-plan.md` this is **Phase 5 (parked)**, gated behind Phase 2 (ATM ops), Phase 0.3/0.4 (notification + document infra), and **Phase 4** (`internal/corebanking` escrow ingest) — the escrow feed is a hard input for both the tier analysis and the 3-way reconciliation, and it does not exist yet.

---

## 2. Module Scope

| Sub-concern | Flow / URS source | Core |
| --- | --- | --- |
| Scheduling & assignment | `08` + FNC 002 | Escrow balance analysis → risk tier → random non-patterned monthly visit schedule (skip holidays, respect regional PIC availability) → PIC accept/reject → surat tugas |
| Execution & e-sign | `08` + `09` | On-site digital BA (physical count per denom) → DSR column auto-fill → auto-diff → checklist → photo upload → dual e-sign (vendor + bank PIC) → final document |
| Selective machine count | `09` | Same mechanism as vault, triggered by a **supervision instruction** for specific ATMs (not the monthly random schedule); recap at machine level |
| Monitoring | FNC 003 + `08` | Daily progress + per-escrow status (Complete/On-progress/Not Complete) + monthly report on the dashboard |
| Balance recap | FNC 002/003 + `08` | 3-way reconciliation (count vs escrow H-1 vs proofing) + DSR comparison + checklist summary + balance-tier categorization + vendor performance |

### BA templates & checklists (from URS + frontend mock)
- BA templates: **Vault ATM**, **Vault Cash**, **Valas**.
- Checklists: **Vendor PJPUR risk checklist (13 items)** and **selective machine checklist (18 items)**.

### Per-escrow cash-count status (URS)
`Complete` (done) · `On-progress` (in progress) · `Not Complete` (not yet done). Recap table columns: No Escrow, Nama Escrow, Cash count status, Findings.

### Balance tier (design token mapping already defined)
`ui_design.md` already maps the badge: **High = danger · Medium = warning · Low = neutral**. Tier drives the risk-based visit schedule.

---

## 3. Current State (from code inspection)

| Component | Status | Evidence |
| --- | --- | --- |
| `backend/internal/cashcount` | ❌ Missing | `internal/` only contains approval, audit, auth, db, handler, repository, rolemgmt, service |
| `cash_count_schedules` / `cash_count_evidences` tables | ❌ Not migrated | Names approved-as-proposal in `project-context.md` Sec 2 / Sec 12; migrations stop at `018`, next free `019` |
| Frontend route `/cash-count` | ✅ Exists (shell) | `routes/cash-count/index.tsx` — 6 module cards, all `disabled: true` |
| `features/cash-count` folder | ❌ Missing | No feature module; the route is a hardcoded card grid only |
| Sub-routes (scheduling, tier-analysis, execution, checklists, reconciliation, recapitulation) | ❌ Missing | Card `href`s point to routes that are not registered |
| Nav entries | ✅ Exist (disabled) | `lib/config/navigation.ts` — cash-count group, all items `disabled: true` |
| RBAC roles | ✅ Defined | `rbac-role-feature-matrix.md` — `cash-count` feature granted to ADMIN/ADMIN_PARAM/APPACCESS + BRANCH-ATM-USER/BRANCH-ATM-SPV |
| Feature-flow diagrams | ✅ Exist | `feature-flows/08` (vault) & `09` (selective machine) |
| Balance-tier badge tokens | ✅ Defined | `ui_design.md` — High/Medium/Low mapping |
| Escrow feed (`internal/corebanking`) | ❌ Not built | Phase 4, not started — hard input for tier + reconciliation |

---

## 4. Dependencies (blocking)

```
Phase 2 (ATM ops)          ─┐
Phase 0.3 notification     ─┤
Phase 0.4 document         ─┼─> Phase 4 escrow ingest ─> Phase 5 cash count
Phase 4 corebanking/escrow ─┘        (tier + 3-way recon input)
```

Concrete blockers:
- **Phase 4 `internal/corebanking`** — escrow balances from SIBS/MIS feed both the risk-tier analysis and the escrow H-1 leg of the 3-way reconciliation. Without it there is no automated escrow input.
- **`internal/dsr`** — the BA auto-fills its DSR column from the vendor's uploaded DSR. DSR upload exists (Phase 1), so this leg is available.
- **0.3 `internal/notification`** — email/in-app notification to the assigned PIC (assignment, accept/reject reminders).
- **0.4 `internal/document`** — generate & store surat tugas + final BA/checklist/photo bundle (PDF via Typst); photo evidence storage.
- **`internal/approval`** — already exists; reuse `approval.Orchestrator` for any maker-checker gate (do not build a second state machine).
- **Holiday calendar** — scheduling must skip holidays/non-working days; no holiday-calendar source exists yet (shared with replenishment's holiday-adjusted classification).

---

## 5. Data & Schema Needed (proposal, not migrated)

To be confirmed & migrated (starting at `019`); money = numeric/integer minor units, timestamps timestamptz:

- `cash_count_schedules` — period (month), target (vault or ATM), risk tier, scheduled date, assigned PIC, status (scheduled/accepted/rejected/in_progress/complete), reject history reference, source (monthly-random vs supervision-instruction).
- `cash_count_evidences` — BA data (physical count per denomination), DSR snapshot used for auto-fill, computed diff, checklist answers, photo/document references, e-sign records (vendor + bank PIC), final document reference.
- **Risk-tier analysis** table/result — per-vault escrow average + history inputs → High/Medium/Low, reproducible (store inputs + result).
- **Reconciliation recap** table/result — 3-way inputs (cash count, escrow H-1, proofing) + deltas + per-escrow status + findings; reproducible & explainable (project-context Sec 5).
- **Proofing** input — manual balance entry by user for the 3-way match.
- **Supervision instruction** table/columns — for selective machine counts (who issued, target ATMs).
- References to `documents` (surat tugas, final bundle), `notifications`, and `audit_logs` (every transition).

> All names/columns above are **proposals**. Cash-count column design is explicitly deferred to the Phase 5 spec (`development-plan.md` Resolved decision #4). Submit final schema to `project-context.md` Sec 2 when applied (schema is mutable at this stage, Sec 0a). **STOP & flag** on the money/count and reconciliation tables.

---

## 6. State Machine (recommendation, per `how_to_handle_flowprocess.md`)

Avoid one long sync endpoint. Model each concern as a DB-backed state machine, separating **business status** from **technical/processing status**:

```
schedule:    draft -> assigned -> accepted -> in_progress -> complete
                          \-> rejected -> (reschedule | reassign)
BA/evidence: on_progress -> counted -> checklist_done -> photos_uploaded
                         -> e_signed_vendor -> e_signed_bank -> finalized
recon recap: pending -> reconciled (matched | discrepancy_flagged)
escrow status (per escrow): not_complete -> on_progress -> complete
```

- Every transition → write `audit_logs`.
- Maker-checker (where required, e.g. schedule approval or discrepancy follow-up) via `approval.Orchestrator`; effects (surat tugas, final document) apply only after the gate clears.
- Idempotent per (period, target) and per file/photo hash — guard "already scheduled/finalized".
- Random scheduling must be reproducible enough to audit (store the seed/inputs), skip holidays, and respect PIC availability.
- Read monitoring/recap from the **replica**, write counts/BA to **primary**; read-after-write in the same flow uses primary.

---

## 7. Open Questions (must be answered before final design)

1. **Risk-tier formula** — the URS does not define how escrow balance + history maps to High/Medium/Low. Needs a business-confirmed formula (thresholds? quarterly recompute? the frontend mock says "per triwulan").
2. **Escrow source availability** — SIBS/MIS escrow H-1 depends on Phase 4 `internal/corebanking`. Is the escrow batch feed confirmed for the fields the tier + reconciliation need?
3. **E-sign** — certified e-sign is optional (NFR). What e-sign mechanism does the vendor use — in-app signature capture, or an external certified provider?
4. **Supervision instruction (selective machine)** — which role may issue it? (`feature-flows/09` open question.)
5. **Selective machine reconciliation** — reconcile against machine balance (journal/switching) or is BA vs DSR enough? (`feature-flows/09` open question.)
6. **Maker-checker scope** — which transitions actually need a checker (schedule publish? discrepancy sign-off? final BA?) vs immediate-apply-with-audit.
7. **Proofing input** — who enters it, and at what point in the recap flow.
8. **Personal data** — BA/e-sign capture PIC identity; confirm DGCC/TPRA handling (shared concern with replenishment surat tugas).
9. **Holiday calendar source** — where does the non-working-day list come from (shared dependency).

---

## 8. Recommendations & Model per Stage

Suggested order (aligned with `development-plan.md`; do not start before Phase 2 + Phase 4 land):

1. **Clear the blockers first** — Phase 2 (ATM ops) → Phase 0.3/0.4 (notification + document) → Phase 4 (escrow ingest). Do not start Phase 5 before the escrow feed exists.
2. **Answer the open questions**, especially the risk-tier formula and escrow field contract.
3. **Write a formal spec** for `internal/cashcount` (requirements → design → tasks), designing the deferred columns there (Resolved decision #4).
4. **Build incrementally:** schema + migrations → risk-tier analysis → scheduling/assignment state machine + surat tugas → BA/checklist/photo/e-sign execution → 3-way reconciliation recap → monitoring (daily progress + monthly report) + vendor performance → selective-machine variant → frontend wiring (replace the disabled cards).

| Stage | Model | Effort | Reason |
| --- | --- | --- | --- |
| Schema + cash-count table migrations | Opus | High | Money/count + reconciliation data, STOP & flag |
| Risk-tier analysis (escrow + history) | Opus | High | Formula ambiguity + drives the whole schedule; needs business-confirmed math |
| Scheduling/assignment state machine + surat tugas | Opus | High | Random-but-auditable scheduling, accept/reject, holiday + PIC constraints |
| BA/checklist/photo/e-sign execution | Opus | High | Physical-count money math + DSR auto-fill diff + e-sign + documents |
| 3-way reconciliation recap | Opus | High | Money reconciliation across three sources; deltas must be reproducible |
| Monitoring (daily progress + monthly report) | Sonnet | Medium | Read-replica reporting within known dashboard patterns |
| Vendor performance evaluation | Sonnet | Medium | Aggregation over recon results, known patterns |
| Selective-machine variant | Sonnet | Medium | Reuses vault mechanism, scoped to ATMs |
| Frontend wiring (replace disabled cards) | Sonnet | Medium | React within known patterns; route shell + nav already exist |

---

## 9. Definition of Done (per project-context Sec 11)

- [ ] Matches the module/table map (Sec 2) + open questions answered; risk-tier formula confirmed
- [ ] Maker-checker via `approval.Orchestrator` where required + `audit_logs` on every transition
- [ ] Money/counts as numeric, timestamps timestamptz; monitoring/recap on read replica, writes on primary
- [ ] Idempotent per (period, target) and per file/photo hash; surat tugas + final BA only after their gate clears
- [ ] Escrow H-1 sourced from `internal/corebanking`; DSR auto-fill from `internal/dsr`; reconciliation deltas reproducible (inputs stored)
- [ ] Per-escrow status (Complete/On-progress/Not Complete) + findings surfaced in monitoring; monthly report + vendor performance produced
- [ ] Frontend switched from disabled cards to API-backed screens
- [ ] Tests passing (auth/RBAC/money/count/tier/3-way-recon deltas), coverage ≥80% on `internal/*`
- [ ] Builds cleanly in Docker
