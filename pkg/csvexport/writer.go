package csvexport

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// plainNumberRe matches a bare number/phone ("+62 812-345", "-6.2", "(021) 555"),
// which CSVSafe leaves alone even though it starts with + or -.
var plainNumberRe = regexp.MustCompile(`^[+-]?[0-9][0-9 ().-]*$`)

// CSVSafe defuses spreadsheet formula injection: a text cell starting with
// = @ TAB CR (or + / - when it is not a plain number or phone) gets a leading
// apostrophe, so opening an export in Excel cannot execute a formula that an
// admin typed into a vendor name or address.
func CSVSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '@', '\t', '\r':
		return "'" + s
	case '+', '-':
		if !plainNumberRe.MatchString(s) {
			return "'" + s
		}
	}
	return s
}

// UnCSVSafe strips one leading apostrophe that precedes = @ + - TAB CR: the
// importer's existing rule (masterdata_import_csv.go cleanCell), kept as-is so
// import behaviour does not change (Req 13.2). Exact inverse of CSVSafe for
// every s that does not itself start with "'"; that case is inherently
// ambiguous (CSVSafe("=x") == "'=x" == CSVSafe("'=x")).
func UnCSVSafe(s string) string {
	if len(s) >= 2 && s[0] == '\'' && strings.ContainsRune("=@+-\t\r", rune(s[1])) {
		return s[1:]
	}
	return s
}

// Writer writes BOM + header once, then rows with CSVSafe applied per cell.
type Writer struct {
	cw *csv.Writer
}

// NewWriter writes the BOM and header to w.
func NewWriter(w io.Writer, header []string) (*Writer, error) {
	if _, err := io.WriteString(w, BOM); err != nil {
		return nil, err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return nil, err
	}
	return &Writer{cw: cw}, nil
}

// Write writes one row, defusing every cell (the input slice is not modified).
func (w *Writer) Write(cells []string) error {
	safe := make([]string, len(cells))
	for i, c := range cells {
		safe[i] = CSVSafe(c)
	}
	return w.cw.Write(safe)
}

// Flush flushes buffered rows and returns the first write error, if any.
func (w *Writer) Flush() error {
	w.cw.Flush()
	return w.cw.Error()
}

// WriteTemplate writes the blank template of an importable entity: BOM + header only.
func WriteTemplate(w io.Writer, entity string) error {
	e, ok := Lookup(entity)
	if !ok {
		return fmt.Errorf("csvexport: unknown entity %q", entity)
	}
	cw, err := NewWriter(w, e.Header)
	if err != nil {
		return err
	}
	return cw.Flush()
}
