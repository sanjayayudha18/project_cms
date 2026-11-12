# Lessons Learned (Gemini / Antigravity)

Catatan pembelajaran penting dari hasil code review, debugging, atau insiden selama pengembangan menggunakan Gemini. Jika sebuah pelajaran bersifat permanen dan berlaku umum, promosikan menjadi aturan tetap di `.agents/rules/GEMINI.md`.

---

## 2026-11-11 — Penulisan Artefak SDLC Wajib Satu per Satu (Sequential Gate)
- **Konteks**: Saat menyiapkan SDLC `vendor-vaults-export-revamp`, agen langsung membuat `design.md` bersamaan dengan pembaruan `requirements.md` setelah user menjawab Open Questions.
- **Aturan**: Tulis dokumen SDLC satu per satu secara berurutan. Selesaikan `requirements.md` terlebih dahulu dan tunggu user me-review serta menyetujuinya sebelum membuat file `design.md`. Setelah `design.md` disetujui, baru buat `tasks.md`. (Sudah ditambahkan ke `.agents/rules/GEMINI.md`, `.agents/agents/cms2-sdlc.md`, dan `.agents/skills/cms2-sdlc/SKILL.md`).
