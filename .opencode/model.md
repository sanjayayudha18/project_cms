# Model Reference - Backend vs Frontend Coding

Panduan 20 model dari list, diklasifikasikan berdasarkan kecocokan untuk **coding backend** dan **coding frontend**.

**Kategori:**
- 🔧 **Backend**: Logika server, API, database, algoritma, business logic
- 🎨 **Frontend**: UI/UX, komponen visual, styling, interaktivitas
- 🔄 **Keduanya**: Cocok untuk full-stack

---

## Grok 4.5 (`opencode/grok-4.5`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Reasoning kuat dan coding menakjubkan. Natural dalam menjelaskan solusi.
- **Kapan dipakai**: Proyek full-stack, debugging kompleks, atau butuh perspektif reasoning yang berbeda.

## GPT 5.6 Luna (`opencode/gpt-5.6-luna`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Varian GPT-5 yang efisien — membawa reasoning & tool use modern dengan resource yang lebih ringan.
- **Kapan dipakai**: Tugas coding rutin harian (backend/frontend) yang butuh respons cepat dan hemat token.

## GLM-5.3 (`opencode/glm-5.3`)
- **Kecocokan**: 🔧 Backend
- **Kelebihan**: Reasoning sangat kuat, matematika dan formal logic kuat. Cocok untuk algoritma dan logika bisnis.
- **Kapan dipakai**: Tugas matematika, logika berat, kode yang butuh deduktif reasoning, atau service layer kompleks.

## GLM-5.2 (`opencode/glm-5.2`)
- **Kecocokan**: 🔧 Backend
- **Kelebihan**: GLM kelas atas — reasoning kuat, context window besar. Stabil untuk workload backend.
- **Kapan dipakai**: API development, data processing, dan business logic yang butuh konteks panjang.

## GLM-5.1 (`opencode/glm-5.1`)
- **Kecocokan**: 🔧 Backend
- **Kelebihan**: GLM versi stabil — predictabel untuk kebanyakan workload backend.
- **Kapan dipakai**: Warisan kode backend, atau butuh model reasoning dengan resource terbatas.

## Kimi K3 (`opencode/kimi-k3`)
- **Kecocokan**: 🔧 Backend
- **Kelebihan**: Context window sangat besar (hingga 1M token), reasoning kuat. Cocok menganalisis codebase luas.
- **Kapan dipakai**: Monolith/large codebase, refactor sistem lama, atau butuh context ekstrem untuk understanding arsitektur.

## Kimi K2.7 Code (`opencode/kimi-k2.7-code`)
- **Kecocokan**: 🔄 Keduanya (fokus coding)
- **Kelebihan**: Disesuaikan khusus untuk coding — kode bersih, bug sedikit, reasoning sufficient.
- **Kapan dipakai**: Penulisan kode harian baik backend (service/API) maupun frontend (komponen).

## Kimi K2.6 (`opencode/kimi-k2.6`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Kimi kelas menengah — balance performa yang baik untuk coding umum.
- **Kapan dipakai**: Kebanyakan tugas sehari-hari, terutama jika butuh support bahasa Asia.

## MiMo-V2.5-Pro (`opencode/mimo-v2.5-pro`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Model MiMo versi profesional — reasoning lebih dalam dari V2.5 dasar, coding layak.
- **Kapan dipakai**: Coba model MiMo, tugas coding menengah, atau fallback dengan kualitas di atas rata-rata.

## MiMo-V2.5 (`opencode/mimo-v2.5`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: MiMo dasar — cukup untuk tugas coding basic baik backend maupun frontend.
- **Kapan dipakai**: Saat butuh model gratis untuk eksplorasi coding cepat tanpa komitmen.

## Qwen3.8 Max (`opencode/qwen3.8-max`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Qwen terbaru — performa diharapkan lebih tinggi, reasoning & coding kuat.
- **Kapan dipakai**: Full-stack development, terutama jika butuh model Qwen terbaru.

## Qwen3.7 Max (`opencode/qwen3.7-max`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Qwen kelas atas — reasoning kuat, coding baik, kemampuan bahasa Cina luar biasa.
- **Kapan dipakai**: Proyek bilingual, atau butuh model yang paham nuansa bahasa Asia dengan kualitas tinggi.

## Qwen3.7 Plus (`opencode/qwen3.7-plus`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Qwen efisien — kualitas di atas rata-rata untuk coding umum dengan resource ringan.
- **Kapan dipakai**: Coding harian, penulisan modul, dokumentasi, dan tugas full-stack umum.

## Qwen3.6 Plus (`opencode/qwen3.6-plus`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Qwen kelas menengah — balance performa yang solid untuk coding umum.
- **Kapan dipakai**: Kebanyakan tugas sehari-hari, terutama jika butuh support bahasa China/Asia.

## MiniMax M3 (`opencode/minimax-m3`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Model Cina yang efisien — cocok untuk prototyping dan iterasi cepat.
- **Kapan dipakai**: Coding dasar, prototyping, tugas ringan, baik backend maupun frontend.

## MiniMax M2.7 (`opencode/minimax-m2.7`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: MiniMax kelas menengah — stabil untuk coding umum.
- **Kapan dipakai**: Seperti M3 tapi butuh sedikit variasi performa — coding ringan harian.

## MiniMax M2.5 (`opencode/minimax-m2.5`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: MiniMax entry-level — paling ringan, cocok untuk tugas sangat dasar.
- **Kapan dipakai**: Tugas sangat ringan, scaffolding, atau fallback ketika model utama unavailable.

## Muse Spark 1.2 Contributor (`opencode/muse-spark-1.2-contributor-free`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Gratis — cukup untuk eksplorasi coding dan tugas dasar.
- **Kapan dipakai**: Jadi "contributor" untuk akses gratis, atau tugas coding kecil dengan syarat setuju pakai data untuk training.

## DeepSeek V4 Pro (`opencode/deepseek-v4-pro`)
- **Kecocokan**: 🔧 Backend
- **Kelebihan**: Reasoning kuat, coding baik, kemampuan long-chain reasoning. Sangat cocok untuk logika kompleks.
- **Kapan dipakai**: Ketika butuh reasoning kuat untuk service layer, algoritma, atau pipeline data.

## DeepSeek V4 Flash (`opencode/deepseek-v4-flash`)
- **Kecocokan**: 🔧 Backend
- **Kelebihan**: DeepSeek cepat — cocok untuk iterasi coding backend yang butuh respons ringan.
- **Kapan dipakai**: Tugas coding backend cepat, prototyping API, atau iterasi logika.

## Hy3 (`opencode/hy3-free`)
- **Kecocokan**: 🔄 Keduanya
- **Kelebihan**: Model gratis default — cukup untuk eksplorasi dan tugas coding basic.
- **Kapan dipakai**: Ketika butuh model dasar untuk catatan cepat atau fallback. Kualitas di bawah model lainnya tapi mencukup untuk basic tasks.

---

## Ringkasan Cepat

| Kategori | Model |
|----------|-------|
| 🔧 **Backend-focused** | GLM-5.3, GLM-5.2, GLM-5.1, Kimi K3, DeepSeek V4 Pro, DeepSeek V4 Flash |
| 🎨 **Frontend-friendly** | (semua model 🔄 juga bisa frontend; khususnya GPT 5.6 Luna, Kimi K2.7 Code, MiniMax M3 untuk prototyping UI) |
| 🔄 **Full-stack / Keduanya** | Grok 4.5, GPT 5.6 Luna, Kimi K2.7 Code, Kimi K2.6, MiMo-V2.5-Pro, MiMo-V2.5, Qwen3.8 Max, Qwen3.7 Max, Qwen3.7 Plus, Qwen3.6 Plus, MiniMax M3, MiniMax M2.7, MiniMax M2.5, Muse Spark 1.2, Hy3 |

---
*Catatan: Klasifikasi berdasarkan karakteristik model (reasoning vs UI generation). Semua model mendukung tool use dan dapat dipakai untuk coding umum. Setiap model menggunakan format `opencode/<model-id>` di OpenCode Zen.*