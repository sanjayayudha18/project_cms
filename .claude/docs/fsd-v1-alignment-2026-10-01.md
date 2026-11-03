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

---

## 7. Addendum 2026-11-03 — full FSD PDF review (`DSR End To End Cash Management V01.md`, converted from the PDF at repo root)

The PDF is a fuller version of FSD v1.0 (1-Sept-2026). Items below were **not** in the repo before. Nothing here is approved or built; schema changes still need approval (CLAUDE.md Sec 3) and the AI-DLC question (Sec 4a rule 0).

### 7.1 New requirements
- **Invoicing trip (FSD §6)** — replaces the URS "vendor uploads invoice" idea with CROWN-computed billing:
  - *Parameter Invoice*: ATM Monitoring uploads a price scheme (Vendor, Paket, Jenis mesin ATM/CDM, Kategori VIP/Reguler, Price, Additional Trip Price); upload replaces the old scheme, upload date = effective date.
  - Package price is per month with a max trip per package (paket 3 = 30, 4 = 40, 5 = 50); trips above max use Additional Trip price.
  - Prorata for ATM activated/deactivated mid-month: price/day = package price ÷ days in month; deactivate = (deactivate date − active date + 1) × price/day; new activate = (days in month − activation date + 1) × price/day.
  - Penalty: system shows count from late-DSR flags per vendor area; ATM Monitoring enters the amount.
  - Dispute loop: ATM Monitoring sends billing → vendor confirms or appeals (reason + mandatory evidence) → ATM Monitoring responds; **max 3 appeal rounds**; final → task for VOC (download invoice, add penalty memo, email ATM Business). Payment via SMART out of scope.
  - ATM master source: ESQ daily file via MFT; Area/Region/City/Provinsi/FLM vendor/Paket formats must match Master Vendor.
  - Schedule text is contradictory: "last 3 business days of the first week of next month" vs "3 business days before month end".
- **Cash count vault**: H/M/L ranges **High > 15 bio (3×), Medium 5–15 bio (2×), Low < 5 bio (1×)** per 6 months (answers open question). Variance threshold **Rp10 juta**: ≤ → recount then archive; > → email escalation, final report downloaded and escalated to Ops & IT Director. Approval by **D3**; final report emailed to D1 officers (CBO, ORM, OIT Risk, CB Risk Control, Digital Banking, NSO, SPAPM). Formulas: Selisih Cash Count = Uang Fisik + Uang Beredar − Uang Masuk Geser Vault H+0 − Uang Bongkaran H+0 − Saldo DSR H-1; Selisih Cek Escrow = same without DSR, minus Saldo Escrow. BA also has UTLE and Uang Rusak. Visit method onsite or virtual (WhatsApp/Webex/Teams).
- **Cash count selektif mesin (was "empty in FSD")**: schedule by Cash Control & Resolution (one surat tugas may cover several ATM IDs), PIC 1/2 accept or reschedule; statuses Assigned → Requested Reschedule → Reschedule → Accepted → Ready for Execution; unique task ID; ID card upload; task to ATM Vendor. Day 1 (H+1): inspection, seal BA, checklist points 1–9; machine problem → estimated resolution duration: ≤ 1 month = On Progress then Day 2, > 1 month = Cancelled and Day 2 stops. Day 2 (H+2): per-cassette count vs Remaining Bill Counter, checklist points 10–15, discrepancy → investigation report signed/stamped by vendor; CCR completeness review loop; D3 then D2 approval. New parameter menus: Category ATM Anomaly, Checklist Berita Acara (selected machine / non-machine, Day 1 / Day 2). Manual field "Konfirmasi Hasil Proofing RPS".
- **Reports (FSD §5)**: 11 cash count reports not in `requirements.md` — Dashboard Summary Plan Vault, Status Cash Count Vault, Progress ATM / Cash / Cash Valas, Selisih Cash Count Vault, Penalty, Temuan Vault (categories 1–20), Performance Month-to-Month, Performance Rank (Human Error, Workstation Support, Infrastructure), Rekap ATM/Cash/Valas. Transaction report reads H+1, kept 1 month then replaced.
- **Forecast daily timeline**: forecast 06:00, DSR upload 06:30–09:00, calc 09:30–10:00, complain/project upload 10:00–11:30, order merge 11:35–12:15, manual adjust 11:45–12:00, final order 12:30. Duplicate ATM ID in complain/project is dropped and a one-way info email goes to the uploading team; ATM ID absent from forecast becomes an order with blank amount, filled by ATM Monitoring.
- **Master vendor / kelolaan**: columns Service Type, ATM Escrow, Cash Escrow, Limit CIS (Ops-editable), Region; in-app notification of Amber/Red PKS to authorized PIC only; manual "Terminated" by company name.

### 7.2 Conflicts with what is recorded
1. Order formula step 8 = `(Forecast Replenish + Plan Replenish) − (Saldo DSR + Forecast Refund)`, Plan Replenish from the H-1 forecast file — conflicts with the 2026-10-08 decision (DMAA amount is final).
2. Cash count reconciliation: §3 has ATM Digital & Reconciliation entering results in CROWN, §4 says results come from Rec 7 → Rec 7 results are **keyed into CROWN manually** (partial answer to open question #2); the note "CROWN computes no cash count deltas" needs revisiting.
3. PKS: flow says check < 8 months, Amber < 6, Red 3, Green ≥ 9; repo has 9 months (A4). Status value lists differ between table (Active/Expired/Terminated) and field description (Draft … Dihentikan).
4. Invoice schedule wording (see 7.1).
5. Overall plan table uses "Category 3 Mths / 1 Mths" while the flow uses a 6-month average.

### 7.3 Open questions now answered
Order approval = single level (ATM Monitoring & Support Head); Provider and Receiver are both Vendor ATM (distinguished by escrow number); H/M/L ranges (7.1). Still open: next step after Provider rejects a pickup; source of H-1 escrow balance (SIBS vs Corebanking file).
