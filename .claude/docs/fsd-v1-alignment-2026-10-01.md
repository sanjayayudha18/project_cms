# FSD v1.0 Alignment — Session Summary (2026-10-01)

Source document: `DSR End To End Cash Management V01.docx` (repo root) — FSD "End To End Cash Management (Replace Opti Cash)" v1.0, first draft 1-Sept-2026, Febrina Afivah. Scope: vendor management & master data, ATM forecasting, pemenuhan order replenish, cash count (parameter, planning, execution, reconciliation), invoicing, dashboard.

**Outcome:** the FSD now overrides URS v0.3 on 5 conflicting points (all decided by the user: "pakai alur FSD"). **Docs only** — no code, schema, or migration was changed. Point 3 is the only one that changes existing code; it must go through the AI-DLC chain.

## 1. Decisions taken (FSD replaces URS)

| # | Topic | Before (URS) | Now (FSD) | Code impact |
|---|---|---|---|---|
| 1 | Order ATM formula | `(Saldo DSR + Proyeksi Refund) − (Rekomendasi DMAA + Rencana Isi Hari-H)` | `Forecast Replenish = Forecast Amount − Saldo DSR + Forecast Refund` | None — **superseded 2026-10-08**: CROWN does no forecasting; DMAA `amount_replenish` is final (CLAUDE.md Sec 3a) |
| 2 | Cash count reconciliation | In-system 3-way (cash count vs escrow H-1 vs proofing) | CROWN builds the recap report → reconciliation runs externally in **Rec 7** | None (`internal/cashcount` not built) |
| 3 | Master data upload | Upsert, all-or-nothing batch, one approval | **Replace-all**; rejected records corrected + re-uploaded; checker approves only clean data | **Yes** — `backend/internal/service/masterdata_import*.go`; needs AI-DLC (Sec 4 #7) |
| 4 | Pemenuhan dana | Surat tugas, KTP/NIP, maker-checker on officer data | Head approval → ACM Branch Coordinator (escrow H-1, manual pindah buku in BDS) → Vendor Receiver/Provider pickup with employee ID + plate validation | None (not built) |
| 5 | Cash count scheduling | Monthly, random, ignore holidays, email accept/reject, surat tugas | H/M/L category from 6-month avg escrow; onsite 3/2/1× per 6 months; up-only re-categorisation; working-day calendar, exclude last year's date; PIC Coordinator/Executor; reschedule + audit memo | None (not built) |

## 2. Files changed

| File | Change |
|---|---|
| `.claude/CLAUDE.md` | Sec 3a formula (#1); Sec 3 `internal/cashcount` → Rec 7 (#2); Sec 12 import note (#3); Sec 3 proposed tables block |
| `.claude/docs/requirements.md` | #1, #2, #3, #4, #5 rewritten with open questions |
| `.claude/docs/decisions.md` | 5 new decision entries (2026-10-01) |
| `.claude/docs/data-map.md` | New section "Proposed — FSD v1.0" (tables/columns) |
| `.claude/feature-flows/01-master-data/feature-flow.md` | New "upload master (FSD)" flow |
| `.claude/feature-flows/03-atm-cash-forecasting/feature-flow.md` | Formula + diagram |
| `.claude/feature-flows/04-pemenuhan-pengambilan-dana/feature-flow.md` | Rewritten to the FSD flow |
| `.claude/feature-flows/08-cash-count-vault/feature-flow.md` | Rules, category flow, scheduling flow, Rec 7 flow |
| `.kiro/steering/development-plan.md` | Phase 2.1 formula + 2026-09-25 decision marked superseded |

## 3. Proposed schema (NOT approved, NOT migrated)

Detail: [data-map.md → Proposed — FSD v1.0](./data-map.md#proposed--fsd-v10-2026-10-01--not-approved-not-migrated).

- **New columns:** `vendors.director_name/director_contact/pks_name/pks_effective_date/pks_expiry_date/pks_terminated_at` · `vendor_branches.cis_limit_amount/cis_limit_currency` · `vendor_vaults.escrow_account`
- **New tables:** `holidays` · `cash_count_category_params` · `cash_count_pics` · `cash_count_reschedule_requests` + column design for the approved `cash_count_schedules`
- **Computed, not stored:** PKS status/flag/remaining months, CIS coverage (min/max/avg escrow vs limit)
- **Reuse:** daily escrow balance from the planned `escrow_batch_rows`

## 4. Open questions (for BA / product owner)

**Formula (#1)**
- Fallback when DSR is missing/late (old "DMAA alone" not carried over)?
- Is `Forecast Refund` the same as URS `Proyeksi Refund` (opening balance H − predicted transactions H & H+1)?

**Rec 7 (#2)**
- FSD step 7 "Upload Reconciliation" is blank — is the Rec 7 result uploaded back into CROWN?
- Recap report format/columns for Rec 7?
- Are manual "proofing" input and vendor performance evaluation still in scope?

**Master upload (#3)**
- "Replace" for rows absent from the file: soft-disable (hard delete forbidden)? What if still referenced by active assignments/orders?
- Re-uploaded corrected records: merge into the pending batch or a new batch?

**Pemenuhan dana (#4)**
- Role mapping: are Vendor Provider and Receiver both vendors (Provider = holder of the source vault/escrow)?
- Next step after the Provider rejects a pickup?
- Source of the H-1 escrow balance?
- Head approval single-level or tiered?

**Cash count scheduling (#5)**
- FSD contradiction: step 7 "Med to Low: update to Low" vs step 8 "maintain Med" — assumed **up-only**.
- Is H3/M2/L1 the onsite count (rest online)?
- How does the annual date plan combine with the 6-month category plan?

**Schema proposal**
- Is "Area Vault Vendor" = `vendor_branches`? An `ATM_CASH` vault would need two escrow numbers.
- Daily escrow balance: SIBS (FSD) vs Corebanking batch file (Sec 3) — the same file?
- Keep maker-checker for Limit CIS edits (FSD has no checker step)?
- Executor "area" values (Jabodetabek / Outregion?) and meaning of "PIC CC STCC / Non STCC"?
- H/M/L boundaries inclusive or exclusive?

**FSD internal inconsistencies to report to the BA**
- PKS Amber threshold: 9 months vs 8 months.
- CIS coverage direction (average < limit = covered) — confirm.

## 5. Not yet handled from the FSD
- Forecast inputs: complaint/project uploads creating orders with **blank** nominal, manual adjustment 11:45–12:00, daily time windows (forecast 06:00, DSR 06:30–09:00, calc 09:30–10:00, uploads 10:00–11:30, merge 11:35–12:15, final order 12:30) — only partly reflected in `requirements.md`.
- `sdlc/dsr-late-report/intent.md`: add the DSR upload window 06:30–09:00.
- Invoice: "feedback status pembayaran" (payment via SMART is out of scope) — consistent with "no payment execution"; not yet documented.
- Integration and GCP infrastructure diagrams (FSD images) not reviewed.
- Cash count for selected machine: empty in the FSD — flow 09 unchanged.

## 5a. Answered 2026-10-05 (blockers for PKS + Limit CIS)
| # | Question | Answer |
|---|---|---|
| A1 | Approve PKS / CIS / escrow columns | Approved → migration `022` |
| A2 | Area Vault Vendor + ATM_CASH escrow | = `vendor_branches`; one escrow per vault; separate ATM + CASH vaults |
| A3 | Limit CIS edit control | Master-data maker-checker |
| A4 | PKS Amber threshold (9 vs 8 months) | 9 months, configurable |
| A5 | CIS coverage direction | avg ≤ limit → Covered |
| A6 | Daily escrow balance source | Monitoring deferred; first feature = PKS + Limit CIS input |

Schema questions on Area Vault Vendor, CIS maker-checker and PKS threshold above are therefore closed. The remaining open questions are #1 (formula), #2 (Rec 7), #3 (master upload), #4 (pemenuhan), #5 (cash count), and the schema questions on executor area and H/M/L bounds.

## 6. Suggested next steps
1. Product owner answers the open questions above (especially #1 fallback and the schema questions).
2. Approve the schema proposal → move it into CLAUDE.md Sec 3 as approved.
3. Start the AI-DLC for the first build: PKS + Limit CIS (additive columns, existing Master Vendor menu) → migration `021+`.
4. Separate AI-DLC for the master-data replace upload (#3), since it changes existing code and touches deletes.
5. Update `development-progress.md` and `.claude/sdlc/README.md` once a feature starts.
