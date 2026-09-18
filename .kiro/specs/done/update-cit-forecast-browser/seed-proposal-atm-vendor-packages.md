# Design Proposal — Seeding `vendor_packages` + `atm_vendor_packages` from MASTER_ATM_ESQ

> **Status: PROPOSAL, awaiting approval.** No code generated yet. This is the STOP-and-confirm artifact for the money/master-data seed (project-context Sec 3 rule 7). All numbers below are measured from the real file `document/MASTER_ATM_ESQ.csv.xlsx` (1913 rows, 9 rows have null vendor/subregion and are skipped → 1904 usable).
>
> The forecast-browser FLM Vendor / FLM Vendor Region columns stay `"-"` until this seed lands. This proposal is what makes them show real values.

---

## What the data actually says (measured, not assumed)

| Fact | Measured | Implication |
|---|---|---|
| Distinct `(FLMVendor, FLMVendorSubRegion)` branches | **348** | Matches the 349 seeded `vendor_branches` (006). The branch layer is already correct; we only link ATMs to it. |
| Branches with >1 distinct `FLMVendorRegion` | **2 of 348** (both dirty: trailing-space dup `TAG Jawa Barat `; one real ROH split) | `FLMVendorRegion` is effectively 1-per-branch → correct to store on `vendor_branches.region` (migration 030). Dirty rows need normalization. |
| Branches with >1 distinct `PAKET` | **228 of 348** | **`PAKET` is a per-ATM attribute, not per-branch.** e.g. `Bijak Jakarta` has PAKET 3/4/5 mixed. |
| Branches with >1 distinct `PriorityClass` | **15 of 348** | `PriorityClass` is also **per-ATM** (VIP/Non VIP/Industri mixed within a branch). |
| `(vendor, subregion, PAKET)` combos | 593 | = number of `vendor_packages` rows if keyed that way |
| `(vendor, subregion, PAKET, PriorityClass)` combos | 624 | = rows if PriorityClass also keys the package |
| `BlackListed` | all `0` | nothing to flag |

**Key realization:** `PAKET` and `PriorityClass` vary *per ATM within a single branch*, so they describe **the ATM's relationship to its vendor package**, not the branch. That reframes the four questions.

---

## Decision 1 — Where do `PAKET` and `PriorityClass` live? (drives everything else)

`vendor_packages` (migration 002) already has `code`, `priority_class`, `price`, unique `(vendor_branch_id, code)`. Three viable models:

| Option | vendor_packages rows | How ATM links | Pros | Cons |
|---|---|---|---|---|
| **1A** — package = `(branch, PAKET)`; `code = 'PAKET N'`; `priority_class` left generic | 593 | `atm_vendor_packages` per ATM → its `(branch, PAKET)` package | Fewest package rows; PAKET is the natural "package" concept | `priority_class` on the package is a lie (it's per-ATM) — must move PriorityClass to the ATM |
| **1B** — package = `(branch, PAKET, PriorityClass)`; `code = 'PAKET N / VIP'` | 624 | per ATM → its exact combo | Every source combo has a package; no data lost | 624 rows, `priority_class` used as intended, but "package" now conflates two concepts |
| **1C** — package = `(branch, PAKET)`; add `atms.priority_class` column | 593 | per ATM | Cleanest: PAKET on package (where it belongs), PriorityClass on ATM (where the data says it belongs) | Needs a 3rd column in migration 030 (`atms.priority_class`) |

**Recommendation: 1C.** The data is unambiguous that PriorityClass is an ATM attribute (15 branches mix it), so modeling it on `atms` is the faithful choice, and PAKET→package keeps `vendor_packages` clean. Cost: one more additive column (`atms.priority_class text`) added to migration 030.

> If you want to avoid touching migration 030 again, pick **1B** (self-contained, no new column) — acceptable, just less clean.

## Decision 2 — `vendor_packages.price` (HARD BLOCKER: NOT NULL, no source column)

ESQ has **no price**. Options:

| Option | What it does | Recommendation |
|---|---|---|
| **2A** | You provide a price-per-PAKET table (e.g. PAKET 3 = Rp X, 4 = Rp Y, 5 = Rp Z) | **Best if you have the numbers** — real data |
| **2B** | Seed `price = 0`, add `-- TODO: real price` marker, backfill later | Acceptable stopgap; unblocks the FLM Vendor column now, price filled when known |
| **2C** | Price comes from a contract/invoice doc not yet seen | Tell me the source and I'll parse it |

**Recommendation: 2A if you have a PAKET price list handy; otherwise 2B** (0 placeholder) so the vendor link works now. Price does not affect the forecast-browser columns — those show vendor/region, not price — so 2B does not block the original goal.

## Decision 3 — `atm_vendor_packages` effective dates (NOT NULL start date, no source date)

| Field | Proposed default |
|---|---|
| `effective_start_date` | `DATE '2026-01-01'` (fixed seed epoch; adjust if you have a real go-live) |
| `effective_end_date` | `NULL` (open-ended = currently active) |
| `is_active` | `true` |

This makes every seeded link "active as of the forecast date", which is exactly what the forecast-browser LATERAL join needs.

## Decision 4 — Data normalization applied during seed (no decision needed, just FYI)

- `FLMVendor`: `Bijak` → `BIJAK` (matches vendor seed 005).
- `PAKET`: `Paket 5` → `PAKET 5`; literal `'NULL'` and blanks → treated as no package (ATM gets no `atm_vendor_packages` row, or a default — see below).
- `FLMVendorRegion`: trim trailing spaces (`TAG Jawa Barat ` → `TAG Jawa Barat`); the one real multi-region branch (`ROH / Jakarta - Bellagio` → Pusat vs Timur) I'll flag for you rather than pick.
- `TermId`: coerce numeric cells to text to match `atms.terminal_id`.
- 48 rows have null/`'NULL'` PAKET → **decision sub-point:** skip their `atm_vendor_packages` link, or assign a default package? Recommend **skip** (they show `"-"` FLM vendor, honest).

---

## What I'll generate once you approve (all idempotent, house style)

Assuming **1C + 2B + Decision-3 defaults** (the fastest safe path):

1. **Amend migration 030** to also add `atms.priority_class text` (nullable, additive) — only if you pick 1C.
2. **New migration `031_seed_vendor_packages.sql`** — insert `vendor_packages` (one per `(branch, PAKET)`, `code = 'PAKET N'`, `price = 0` TODO), joined to seeded `vendor_branches` by `branch_name`. Idempotent `WHERE NOT EXISTS`.
3. **New migration `032_seed_atm_vendor_packages.sql`** — link each ATM to its `(branch, PAKET)` package via `atms.terminal_id` + the vendor chain; `effective_start_date = '2026-01-01'`, `is_active = true`. Also backfill `vendor_branches.region`, `atms.escrow_account`, `atms.priority_class` from the same file. Idempotent.
4. Generated by a Python script reading `document/MASTER_ATM_ESQ.csv.xlsx` and emitting the SQL (kept in a `scripts/` or noted as one-off; not committed as runtime code).

**Not run against DB** (external Postgres unreachable) — you apply migrations `030`-`032` and verify counts (`SELECT count(*) FROM atm_vendor_packages` ≈ 1904 minus null-PAKET rows).

---

## Your call

Reply with the option letters, e.g. **"1C, 2B, D3 defaults, skip null-PAKET"** and I'll generate. Or adjust any line. The only true blocker is Decision 2 (price) — everything else has a safe default.
