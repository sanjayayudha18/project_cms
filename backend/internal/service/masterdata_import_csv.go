package service

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// cleanCell undoes the export's formula-injection guard (strips ONE leading
// apostrophe that precedes = @ + - TAB CR) and trims spaces.
func cleanCell(s string) string {
	return strings.TrimSpace(csvexport.UnCSVSafe(s))
}

// readImportCSV parses the upload: size guard, BOM strip, `,` or `;` delimiter
// (whichever the header line uses -- Excel in an Indonesian locale writes `;`),
// header validation, empty-row skipping. A non-empty errs means the file is
// unusable as a whole (bad CSV / wrong header) and no row was processed.
func readImportCSV(r io.Reader, header []string, legacy [][]string, legacyFill map[string]string) (recs []importRecord, errs []ImportRowError, err error) {
	data, err := io.ReadAll(io.LimitReader(r, MasterDataImportMaxBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > MasterDataImportMaxBytes {
		return nil, nil, &ValidationError{Field: "file", Message: fmt.Sprintf("ukuran file melebihi %d MB", MasterDataImportMaxBytes>>20)}
	}
	data = bytes.TrimPrefix(data, []byte(utf8BOM))

	firstLine := data
	if n := bytes.IndexByte(data, '\n'); n >= 0 {
		firstLine = data[:n]
	}
	comma := ','
	if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		comma = ';'
	}
	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = comma
	cr.FieldsPerRecord = -1

	gotHeader := false
	var pick []int // legacy header in use: header column -> file column, -1 = fill
	legacyLen := -1
	for {
		rec, rerr := cr.Read()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			line := 0
			var pe *csv.ParseError
			if errors.As(rerr, &pe) {
				line = pe.Line
			}
			return nil, []ImportRowError{{Row: line, Field: "file", Message: "format CSV tidak valid: " + rerr.Error()}}, nil
		}
		line, _ := cr.FieldPos(0)
		if !gotHeader {
			gotHeader = true
			if headerMatches(rec, header) {
				continue
			}
			for _, l := range legacy {
				if headerMatches(rec, l) {
					pick, legacyLen = legacyPick(header, l), len(l)
					break
				}
			}
			if pick == nil {
				return nil, []ImportRowError{{Row: 1, Field: "header", Message: "kolom header tidak sesuai; diharapkan persis: " + strings.Join(header, ",")}}, nil
			}
			continue
		}
		cells, empty := cleanRecord(rec)
		if empty {
			continue
		}
		if len(cells) == legacyLen {
			cells = expandLegacy(cells, header, pick, legacyFill)
		}
		if len(recs) >= MasterDataImportMaxRows {
			return nil, nil, &ValidationError{Field: "file", Message: fmt.Sprintf("jumlah baris melebihi %d", MasterDataImportMaxRows)}
		}
		recs = append(recs, importRecord{line: line, cells: cells})
	}
	if !gotHeader {
		return nil, []ImportRowError{{Row: 1, Field: "header", Message: "file kosong"}}, nil
	}
	return recs, nil, nil
}

// cleanRecord applies cleanCell to every cell; empty reports an all-blank row.
func cleanRecord(rec []string) (cells []string, empty bool) {
	cells = make([]string, len(rec))
	empty = true
	for i, c := range rec {
		cells[i] = cleanCell(c)
		if cells[i] != "" {
			empty = false
		}
	}
	return cells, empty
}

// expandLegacy reorders a legacy-header row into header order, filling the
// columns the legacy file lacks from legacyFill.
func expandLegacy(cells, header []string, pick []int, legacyFill map[string]string) []string {
	full := make([]string, len(header))
	for c, from := range pick {
		if from < 0 {
			full[c] = legacyFill[header[c]]
		} else {
			full[c] = cells[from]
		}
	}
	return full
}

// legacyPick maps each column of full to its index in legacy (-1 = missing).
func legacyPick(full, legacy []string) []int {
	pick := make([]int, len(full))
	for c, h := range full {
		pick[c] = slices.Index(legacy, h)
	}
	return pick
}

func headerMatches(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if strings.ToLower(strings.TrimSpace(got[i])) != want[i] {
			return false
		}
	}
	return true
}

// importSpan is one active kelolaan period of an ATM ("" end = open-ended).
type importSpan struct {
	start, end string
	id         int64
	row        int
}

func spansOverlap(a, b importSpan) bool {
	return (b.end == "" || a.start <= b.end) && (a.end == "" || b.start <= a.end)
}
