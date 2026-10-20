# Maintain / incidents: vendor upload DSR

Status: draft. Stage: 6 Maintain. Closes the loop.
A control-band breach, ticket, or channel message writes a new `intent.md` and the loop restarts.

## Monitored signals (define bands per signal)
- DSR ingest failure rate (`atm_dsr_saldo_files.status = 'failed'` share).
- Files with `error_count > 0` (broken-cell rate) trending up.
- Missing/late uploads vs 09:00 deadline per vendor (feeds FLM penalty report).

## Response tiers (deterministic detection; model only after breach)
- 1sigma: log only.
- 2sigma: diagnose read-only (Read, Grep).
- 3sigma: propose fix as PR into the review gate, or write a new `intent.md`.

## Incident log
| Date | Signal | Tier | Diagnosis | Outcome (PR / intent.md / eval added) |
|------|--------|------|-----------|----------------------------------------|
| - | - | - | - | - |

## Evals added from incidents
<!-- Each production incident becomes a permanent regression eval. -->
- (none yet)
