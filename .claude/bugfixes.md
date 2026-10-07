# Bug Fix Log

Small fixes that the user decided don't need a full AI-DLC chain (CLAUDE.md Sec 4a rule 0). Newest first. One entry per fix.

Template:

```markdown
## YYYY-MM-DD — <short title>
- **Symptom**: what the user saw
- **Root cause**: why it happened
- **Fix**: what changed (files)
- **Tests**: automated check added/run; manual browser verification: outstanding
- **Commit**: <sha>
```

---

## 2026-09-30 — Halaman bisa di-scroll ke area kosong (forecast-browser)
- **Symptom**: `/replenishment/forecast-browser` masih bisa scroll ke bawah melewati konten; seluruh shell (sidebar + main) naik dan menyisakan area kosong.
- **Root cause**: elemen `sr-only` (`position: absolute`) di `ForecastSummary.tsx`/`ForecastBrowser.tsx` tidak punya ancestor ber-`position`, jadi containing block-nya = viewport. Elemen itu lolos dari clipping `overflow` `<main>`/grid dan memperpanjang tinggi dokumen.
- **Fix**: `AppShell.tsx` — `<main>` diberi `relative` (berlaku untuk semua halaman protected). Grid rows `auto minmax(0, 1fr)` ikut di diff yang sama.
- **Tests**: `AppShell.test.tsx` assert `<main>` punya `relative` — 11/11 lulus; manual browser verification: outstanding.
- **Commit**: _(belum)_
