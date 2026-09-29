# Assessment — ATM Replenishment Development

> Development-readiness assessment for the **ATM Replenishment** module (`internal/replenishment`).
> Status: draft assessment · Date: 2026-09-28 · Target phase: **Phase 2.2** (`development-plan.md`).
> Sources: URS v0.3 Phase 1, `feature-flows/04` & `06`, `project-context.md`, `development-plan.md`, and inspection of the actual code.

---

## 1. Executive Summary

Replenishment is the daily transactional core of ATM operations: it turns order recommendations (from forecast) into fill instructions, facilitates fund fulfillment & pickup with a surat tugas (assignment letter) + handover, then validates realization vs order and produces reports.

**Current state:** the module is **not built yet**. The backend `internal/replenishment` does not exist (only generic layers are present: handler/service/repository/db/approval/audit/rolemgmt/auth). The frontend `features/replenishment` is still **mock** (reads from `src/data/replenishment-schedules.json`, not API-backed). What exists so far is the business requirements, two feature-flow diagrams, and the roadmap items.

**Readiness to start:** ⚠️ **Not ready to work on now.** Per `development-plan.md`, replenishment (2.2) depends on the **2.1 forecast engine** and several Phase 0 infrastructure foundations. Neither is complete.

---

## 2. Module Scope

Based on the URS and feature flows, the module covers three sub-flows:

| Sub-flow | Flow source | Core |
| --- | --- | --- |
| Replenishment instruction | `06` + URS FNC 001 | Publish approved order → per-ATM/vendor fill instruction |
| Fund fulfillment & pickup | `feature-flows/04` | Branch/CM sets location+amount+schedule → FLM inputs officer/vehicle → maker-checker → surat tugas → handover |
| Realization validation & reports | `feature-flows/06` | Classify realization vs order (holiday-adjusted) + 8 reports + export |

### Realization classification (key business rule, holiday-adjusted)
1. On-schedule (0 working-day difference)
2. Early fill (1–2 days early)
3. Late fill (1–2 days late)
4. Not done (>2 days off or not filled)
5. Fill without order (realization with no matching order)

---

## 3. Current State (from code inspection)

| Component | Status | Evidence |
| --- | --- | --- |
| `backend/internal/replenishment` | ❌ Missing | `internal/` only contains approval, audit, auth, db, handler, repository, rolemgmt, service |
| `replenishment_instructions` table | ❌ Not migrated | Name approved in `project-context.md` Sec 2, no migration yet (next free: `019`) |
| Frontend `features/replenishment` | ⚠️ Mock | `ReplenishmentScreen`, `replenishment.utils.ts` (sortByStatusPriority, filterSchedules) read `replenishment-schedules.json` |
| Route `/replenishment` | ✅ Exists (mock) | `routes/replenishment.tsx` |
| Dashboard `ReplenishmentSummary` | ⚠️ Mock | Part of `features/dashboard`, hardcoded data |
| Feature flow diagrams | ✅ Exist | `feature-flows/04` & `06` |
| Order ATM formula | ✅ Confirmed valid (2026-09-25) | Belongs to the forecast module (2.1), not replenishment |

---

## 4. Dependencies (blocking)

```
Phase 1 (close in-flight)  ─┐
Phase 0 (infra)            ─┤
  0.2 import/export jobs   ─┤
  0.3 notification (SMTP)  ─┼─> 2.1 forecast engine ─> 2.2 replenishment
  0.4 document (surat tugas)┘
```

Concrete blockers for replenishment:
- **2.1 `internal/forecast`** — replenishment consumes approved orders from forecast. Without the order engine, there is no input.
- **0.3 `internal/notification`** — pickup-schedule notification to FLM + instruction publication.
- **0.4 `internal/document`** — generate & store the Surat Tugas (PDF via Typst) for download.
- **`internal/approval`** — already exists; must be reused via `approval.Orchestrator` (do not build a second state machine).

---

## 5. Data & Schema Needed (proposal, not migrated)

Needs to be confirmed & migrated (starting at `019`); money = numeric/integer minor units:

- `replenishment_instructions` — approved order → per-ATM/vendor instruction; lifecycle status.
- **Fund fulfillment** table/columns — location, amount per denom, date, time, providing vault.
- **Fund pickup** table/columns — officer (name, KTP, NIP, company, purpose), vehicle, amount + per denom, sheet count.
- **Realization classification** table/result — reproducible inputs + deltas (project-context Sec 5).
- References to `documents` (surat tugas) & `audit_logs` (every transition).

> All names/columns above are **proposals** — submit them to `project-context.md` Sec 2 when applied (schema is mutable at this stage, Sec 0a).

---

## 6. State Machine (recommendation, per `how_to_handle_flowprocess.md`)

Avoid one long sync endpoint. Model it as a DB-backed state machine:

```
instruction:   draft -> published -> in_progress -> completed
fund pickup:    requested -> pending_approval -> approved -> handed_over
                            \-> rejected
realization:    pending -> classified (on_schedule/early/late/not_done/without_order)
```

- Separate **business status** from **technical/processing status**.
- Every transition → write `audit_logs`.
- Maker-checker via `approval.Orchestrator`; effects (surat tugas, publish) apply only after `approved`.
- Idempotent per instruction/order (guard "already published/handed over").
- Read reports from the **replica**, write to **primary**.

---

## 7. Open Questions (must be answered before final design)

1. **Realization data source** — from Corebanking, switching, or DSR? (`feature-flows/06`)
2. **Maker & Checker for fund pickup** — bank side or vendor side? (`feature-flows/04`)
3. **Handover** — needs certified e-sign or is in-app confirmation enough?
4. **Vendor officer KTP no.** — falls under DGCC/TPRA scope; requires a decision on personal-data retention/masking.
5. **Order ATM fallback** (DSR missing → DMAA only) — already confirmed valid, but its impact on the replenishment instruction needs to be affirmed.

---

## 8. Recommendations & Model per Stage

Suggested order (aligned with `development-plan.md`):

1. **Clear the blockers first** — Phase 1 (in-flight) → Phase 0.2/0.3/0.4 → 2.1 forecast. Do not start 2.2 before this.
2. **Write a formal spec** for `internal/replenishment` (requirements → design → tasks) after the open questions are answered.
3. **Build incrementally:** schema + migrations → instruction state machine → fulfillment/pickup + surat tugas → handover → realization classification → reports + export → frontend wiring (replace mock).

| Stage | Model | Effort | Reason |
| --- | --- | --- | --- |
| Schema + replenishment table migrations | Opus | High | Money + data migration, STOP & flag |
| Instruction state machine + lifecycle | Opus | High | A wrong transition corrupts fill status |
| Fulfillment/pickup + maker-checker + surat tugas | Opus | High | Maker-checker + documents + personal data |
| Realization classification (holiday-adjusted) | Opus | High | Money math + holiday edge cases |
| Reports + export | Sonnet | Medium | Read-replica + export patterns already exist |
| Frontend wiring (replace mock) | Sonnet | Medium | React within known patterns |

---

## 9. Definition of Done (per project-context Sec 11)

- [ ] Matches the module/table map (Sec 2) + open questions answered
- [ ] Maker-checker via `approval.Orchestrator` + `audit_logs` on every transition
- [ ] Money as numeric, timestamps timestamptz; read replica / write primary
- [ ] Idempotent per instruction/order; surat tugas only after approved
- [ ] Frontend switched from mock to API-backed
- [ ] Tests passing (auth/RBAC/money/classification), coverage ≥80% on `internal/*`
- [ ] Builds cleanly in Docker
