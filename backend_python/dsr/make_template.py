"""Generate the universal DSR upload template (sheets 'Daily', 'Rencana Isi', 'Petunjuk').

The layout is fixed on purpose so every vendor fills the same cells. It is built to be read
by dsr_etl.read_daily_rows / read_rencana_isi_rows unchanged (no DB change).

    python -m dsr.make_template                 # writes templates/Template_DSR_Universal.xlsx
    python -m dsr.make_template --sample out.xlsx   # filled sample with literal totals (parser test)
"""
from __future__ import annotations

import argparse
from datetime import date
from pathlib import Path

from openpyxl import Workbook
from openpyxl.styles import Alignment, Border, Font, PatternFill, Side
from openpyxl.utils import get_column_letter as L

OUT = Path(__file__).resolve().parent / "templates" / "Template_DSR_Universal.xlsx"
DENOMS = [100000, 50000, 20000, 10000, 5000, 2000, 1000]
D0 = 6                     # first denom column (F)
DN = D0 + len(DENOMS) - 1  # last denom column (L)
TOT = DN + 1               # total column (M)
REM = TOT + 2              # keterangan column (O)

HEAD = PatternFill("solid", fgColor="EDE6E5")
INPUT = PatternFill("solid", fgColor="FFF8E1")
CALC = PatternFill("solid", fgColor="F3F1F1")
THIN = Side(style="thin", color="C9C2C1")
BOX = Border(left=THIN, right=THIN, top=THIN, bottom=THIN)
BOLD = Font(bold=True)
NUM = "#,##0"


def _label(ws, ref, text, bold=False):
    ws[ref] = text
    if bold:
        ws[ref].font = BOLD


def _inputs(ws, row, values=None):
    for i, c in enumerate(range(D0, DN + 1)):
        cell = ws.cell(row, c, None if values is None else values[i])
        cell.fill, cell.border, cell.number_format = INPUT, BOX, NUM


def _row_total(ws, row, sample):
    cell = ws.cell(row, TOT)
    if sample:
        cell.value = sum((ws.cell(row, c).value or 0) for c in range(D0, DN + 1))
    else:
        cell.value = f"=SUM({L(D0)}{row}:{L(DN)}{row})"
    cell.fill, cell.border, cell.number_format = CALC, BOX, NUM


def _calc(ws, row, formula, sample, value_fn):
    for c in range(D0, DN + 1):
        cell = ws.cell(row, c)
        cell.value = value_fn(c) if sample else formula.replace("{c}", L(c))
        cell.fill, cell.border, cell.number_format = CALC, BOX, NUM
    _row_total(ws, row, sample)


def build(sample: bool = False) -> Workbook:
    wb = Workbook()
    ws = wb.active
    ws.title = "Daily"

    # --- identitas laporan: label di kolom D, nilai di kolom E, satu baris per field ---
    meta = [("Kepada", "Cash Processing Center"), ("Dari", ""), ("Bank", "BANK CIMB NIAGA"),
            ("Company", "PT. NAMA VENDOR" if not sample else "PT. CONTOH VENDOR"),
            ("Tanggal", None), ("Perihal", "Laporan saldo harian ATM")]
    for i, (k, v) in enumerate(meta, start=1):
        _label(ws, f"D{i}", k, True)
        ws[f"E{i}"] = v
        ws[f"E{i}"].fill, ws[f"E{i}"].border = INPUT, BOX
    ws["E5"] = date(2026, 7, 15)
    ws["E5"].number_format = "dd/mm/yyyy"

    _label(ws, "A8", "SALDO HARIAN ATM", True)
    hdr = 9
    for ref, txt in ((f"A{hdr}", "Tanggal"), (f"C{hdr}", "Uraian"),
                     (f"{L(D0)}{hdr}", "Denom (Rupiah penuh, bukan x1.000)"),
                     (f"{L(TOT)}{hdr}", "Total Rupiah (nominal penuh)"),
                     (f"{L(REM)}{hdr}", "Keterangan")):
        ws[ref] = txt
    for c in range(1, REM + 1):
        ws.cell(hdr, c).fill, ws.cell(hdr, c).font = HEAD, BOLD
    for i, d in enumerate(DENOMS):
        cell = ws.cell(hdr + 1, D0 + i, d)
        cell.fill, cell.font, cell.number_format = HEAD, BOLD, NUM

    sv = {c: (i + 1) * 1000 for i, c in enumerate(range(D0, DN + 1))}  # sample values
    r = 11
    sv_in = (lambda k: [sv[c] * k for c in range(D0, DN + 1)]) if sample else (lambda k: None)

    def date_cell(row, d):
        ws.cell(row, 1, d).number_format = "dd/mm/yyyy"
        ws.cell(row, 1).fill, ws.cell(row, 1).border = INPUT, BOX

    # === d0: saldo posisi 00:00 ===
    date_cell(r, date(2026, 7, 15))
    _label(ws, f"C{r}", "SALDO AWAL", True)
    _inputs(ws, r, sv_in(10)); _row_total(ws, r, sample)
    awal = r
    r += 1
    recv = ["Dari Rekonsiliasi Cartridge ATM dan CRM", "Hasil bongkaran CDM",
            "Uang nyangkut CRM", "Dari CIMB Niaga CIT", "Penyelesaian Klaim Nasabah",
            "Penyelesaian Klaim Selisih Kurang Fisik"]
    first = r
    for i, t in enumerate(recv):
        if i == 0:
            _label(ws, f"C{r}", "Penerimaan", True)
        ws[f"D{r}"] = t
        _inputs(ws, r, sv_in(2) if (sample and i == 3) else ([0] * len(DENOMS) if sample else None))
        _row_total(ws, r, sample)
        r += 1
    last = r - 1
    _label(ws, f"C{r}", "Subtotal Penerimaan", True)
    _calc(ws, r, f"=SUM({{c}}{first}:{{c}}{last})", sample,
          lambda c: sum((ws.cell(x, c).value or 0) for x in range(first, last + 1)))
    sub_in = r
    r += 1
    pay = ["Untuk Cartridge Replenishment ATM & CRM", "Setoran Ke Cash Management"]
    first = r
    for i, t in enumerate(pay):
        if i == 0:
            _label(ws, f"C{r}", "Pengeluaran", True)
        ws[f"D{r}"] = t
        _inputs(ws, r, [0] * len(DENOMS) if sample else None)
        _row_total(ws, r, sample)
        r += 1
    last = r - 1
    _label(ws, f"C{r}", "Subtotal Pengeluaran", True)
    _calc(ws, r, f"=SUM({{c}}{first}:{{c}}{last})", sample,
          lambda c: sum((ws.cell(x, c).value or 0) for x in range(first, last + 1)))
    sub_out = r
    r += 1
    _label(ws, f"C{r}", "SALDO AKHIR SAMPAI PUKUL 00:00", True)
    _calc(ws, r, f"={{c}}{awal}+{{c}}{sub_in}-{{c}}{sub_out}", sample,
          lambda c: ws.cell(awal, c).value + ws.cell(sub_in, c).value - ws.cell(sub_out, c).value)
    akhir = r
    r += 2

    # === d1: status saldo sementara (H+1) ===
    date_cell(r, date(2026, 7, 16))
    _label(ws, f"C{r}", "STATUS SALDO SEMENTARA", True)
    r += 1
    _label(ws, f"C{r}", "Penerimaan", True)
    ws[f"D{r}"] = "Dari Batal Replenishment"
    _inputs(ws, r, [0] * len(DENOMS) if sample else None); _row_total(ws, r, sample)
    first = last = r
    r += 1
    _label(ws, f"C{r}", "Subtotal Penerimaan", True)
    _calc(ws, r, f"=SUM({{c}}{first}:{{c}}{last})", sample, lambda c: ws.cell(first, c).value)
    sub_in2 = r
    r += 1
    first = r
    for i, t in enumerate(["Untuk Cartridge Replenishment", "Untuk Kas Cadangan", "Untuk Lain-lain"]):
        if i == 0:
            _label(ws, f"C{r}", "Pengeluaran", True)
        ws[f"D{r}"] = t
        _inputs(ws, r, sv_in(1) if (sample and i == 0) else ([0] * len(DENOMS) if sample else None))
        _row_total(ws, r, sample)
        r += 1
    last = r - 1
    _label(ws, f"C{r}", "Subtotal Pengeluaran", True)
    _calc(ws, r, f"=SUM({{c}}{first}:{{c}}{last})", sample,
          lambda c: sum((ws.cell(x, c).value or 0) for x in range(first, last + 1)))
    sub_out2 = r
    r += 1
    _label(ws, f"C{r}", "SALDO SEMENTARA PUKUL 09:00", True)
    _calc(ws, r, f"={{c}}{akhir}+{{c}}{sub_in2}-{{c}}{sub_out2}", sample,
          lambda c: ws.cell(akhir, c).value + ws.cell(sub_in2, c).value - ws.cell(sub_out2, c).value)
    r += 2

    _label(ws, f"C{r}", "STATUS UANG CADANGAN ATM", True)
    r += 1
    _label(ws, f"C{r}", "Kondisi Layak Edar")
    _inputs(ws, r, sv_in(9) if sample else None); _row_total(ws, r, sample)
    layak = r
    r += 1
    _label(ws, f"C{r}", "Kondisi Rusak & Tidak Layak Edar")
    _inputs(ws, r, [0] * len(DENOMS) if sample else None); _row_total(ws, r, sample)
    r += 1
    _label(ws, f"C{r}", "TOTAL STATUS UANG CADANGAN", True)
    _calc(ws, r, f"=SUM({{c}}{layak}:{{c}}{r - 1})", sample,
          lambda c: ws.cell(layak, c).value + ws.cell(layak + 1, c).value)
    r += 3
    for col, t in ((4, "Dibuat"), (6, "Diperiksa"), (11, "Disetujui")):
        ws.cell(r, col, t).font = BOLD
        ws.cell(r + 4, col, "( nama )")

    ws.column_dimensions["A"].width = 12
    ws.column_dimensions["B"].width = 2
    ws.column_dimensions["C"].width = 34
    ws.column_dimensions["D"].width = 42
    ws.column_dimensions["E"].width = 22
    for c in range(D0, DN + 1):
        ws.column_dimensions[L(c)].width = 14
    ws.column_dimensions[L(TOT)].width = 16
    ws.column_dimensions[L(REM)].width = 28
    ws.freeze_panes = "A11"

    # --- Rencana Isi ---
    ri = wb.create_sheet("Rencana Isi")
    ri["A1"] = "RENCANA PENGISIAN"
    ri["A1"].font = BOLD
    ri["A2"] = "Tanggal :"
    ri["B2"] = date(2026, 7, 16)
    ri["B2"].number_format = "dd/mm/yyyy"
    ri["B2"].fill, ri["B2"].border = INPUT, BOX
    heads = ["ATM ID", "Lokasi ATM", "50/100", "Jumlah Isi Denom 100rb",
             "Jumlah Isi Denom 50rb", "Saldo Splank Pukul 08:00", "Keterangan"]
    for i, h in enumerate(heads, start=1):
        c = ri.cell(3, i, h)
        c.fill, c.font, c.border = HEAD, BOLD, BOX
        c.alignment = Alignment(wrap_text=True, vertical="center")
    for rr in range(4, 4 + 200):
        for cc in range(1, 8):
            cell = ri.cell(rr, cc)
            cell.fill, cell.border = INPUT, BOX
            if cc in (4, 5, 6):
                cell.number_format = NUM
    if sample:
        for i, v in enumerate(["5902", "KRW.SPBU CONTOH", "100", 400000, 0, 0, None], start=1):
            ri.cell(4, i, v)
    for col, w in zip("ABCDEFG", (12, 38, 10, 22, 22, 24, 30)):
        ri.column_dimensions[col].width = w
    ri.freeze_panes = "A4"

    # --- Petunjuk ---
    pt = wb.create_sheet("Petunjuk")
    lines = [
        "PETUNJUK PENGISIAN DSR (TEMPLATE UNIVERSAL)",
        "",
        "1. Isi hanya sel berwarna kuning. Sel abu-abu berisi rumus, jangan diubah.",
        "2. Jangan mengubah nama sheet (Daily, Rencana Isi), teks label di kolom C/D, urutan baris, "
        "atau posisi kolom. Sistem membaca file berdasarkan label itu.",
        "3. Semua nominal dalam Rupiah penuh (contoh 1.307.300), bukan dalam ribuan (x1.000).",
        "4. Kolom Denom berisi nominal rupiah per pecahan (bukan jumlah lembar). Pecahan yang "
        "tidak dipakai diisi 0.",
        "5. Sheet Daily: isi Company dan Tanggal laporan (E4, E5), lalu SALDO AWAL, penerimaan, "
        "pengeluaran. Blok STATUS SALDO SEMENTARA memakai tanggal H+1.",
        "6. Sheet Rencana Isi: satu baris per ATM. ATM ID harus sama dengan data master ATM CIMB Niaga.",
        "7. Simpan sebagai .xlsx, lalu unggah di menu DSR. Sistem akan menampilkan hasil baca "
        "(dry-run) untuk Anda konfirmasi sebelum disimpan.",
        "8. Jika vendor memiliki lebih dari satu vault, salin seluruh blok SALDO HARIAN ATM "
        "(baris 8 sampai TOTAL STATUS UANG CADANGAN) ke bawahnya dan tulis nama vault di "
        "kolom A tepat di atas judul 'SALDO HARIAN ATM' pada salinan itu.",
    ]
    for i, t in enumerate(lines, start=1):
        pt.cell(i, 1, t).alignment = Alignment(wrap_text=True, vertical="top")
    pt["A1"].font = Font(bold=True, size=13)
    pt.column_dimensions["A"].width = 110
    return wb


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--sample", metavar="OUT", help="write a filled sample with literal totals")
    a = ap.parse_args()
    if a.sample:
        build(sample=True).save(a.sample)
        print("sample ->", a.sample)
    else:
        OUT.parent.mkdir(parents=True, exist_ok=True)
        build().save(OUT)
        print("template ->", OUT)


if __name__ == "__main__":
    main()
