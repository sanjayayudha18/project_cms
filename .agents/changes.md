# Small Work Log (Gemini / Antigravity)

Perubahan kecil **bukan bug** (*Small Work Dev*) yang tidak memerlukan spec (`.agents/specs/`), scope-driven dev (`.agents/scopes/`), maupun UI Revamp. Kembaran Claude: `.claude/small-works.md` (CLAUDE.md Sec 4a).
Urutan terbaru di paling atas. Satu entri per perubahan.

**Fit check** (semua harus lolos; gagal satu → Spec / Scope / UI Revamp / Bug Fix):
- Satu concern, sedikit file (patokan ≤ 5), satu portal atau satu modul backend
- Tanpa skema/migrasi, tanpa endpoint baru/berubah (`openapi.yaml` tidak berubah)
- Tanpa aturan bisnis baru, tidak menyentuh Sec 4 #7 (auth, uang, rekonsiliasi, migrasi, delete)
- Bukan bug (→ [bugfixes.md](./bugfixes.md)), bukan redesain layar (→ UI Revamp)

Contoh: ubah label/teks, tambah/sembunyikan/urutkan elemen UI, tweak kecil perilaku yang ada, nilai default, docs/tooling/config.

**Alur**: AI tulis entri (Request + Fit check + AC) → **user OK AC** (satu-satunya gate) → koding + test → isi Change/Tests → review → commit saat user bilang "commit".

Template:

```markdown
## YYYY-MM-DD — <judul singkat>
- **Request**: permintaan user, pakai kata-kata user
- **Fit check**: lolos (atau poin yang gagal → metode lain)
- **AC**: 1. … 2. …  (accepted YYYY-MM-DD oleh user)
- **Change**: file yang disentuh
- **Tests**: automated check yang ditambah/dijalankan; manual browser verification: outstanding (user)
- **Commit**: <sha>
```

---
