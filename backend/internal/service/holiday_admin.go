package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Hari libur (.claude/sdlc/replenishment-realisasi/spec.md FR1): the calendar the realisasi job uses
// to count workdays. ADMIN only (Q14). Documented GR#3 deviation like Region / Role Management:
// changes apply immediately, but every one writes audit_logs in the same transaction (a failed audit
// rolls the change back) and the role is re-checked here as well as at the route.

var (
	ErrHolidayNotFound        = errors.New("holiday not found")
	ErrHolidayDateConflict    = errors.New("an active holiday already exists on that date")
	ErrHolidayStatusUnchanged = errors.New("holiday status unchanged")
)

const (
	holidayNameMax     = 100
	holidayImportMax   = 1000
	holidayMaxPageSize = 100
	holidayEntity      = "holiday"
)

type HolidayAdminService struct {
	pool *pgxpool.Pool
	read *db.Queries
}

// NewHolidayAdminService: writes on pool (primary), the list on readPool (replica).
func NewHolidayAdminService(pool, readPool *pgxpool.Pool) *HolidayAdminService {
	return &HolidayAdminService{pool: pool, read: db.New(readPool)}
}

type HolidayView struct {
	ID          int64      `json:"id"`
	HolidayDate *time.Time `json:"holiday_date"`
	Name        string     `json:"name"`
	IsActive    bool       `json:"is_active"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

type HolidayList struct {
	Items    []HolidayView `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type HolidayImportResult struct {
	Created int      `json:"created"`
	Skipped []string `json:"skipped"` // dates already active
}

func holidayView(h db.Holiday) HolidayView {
	return HolidayView{ID: h.ID, HolidayDate: dateToPtr(h.HolidayDate), Name: h.Name, IsActive: h.IsActive,
		UpdatedAt: timestamptzToPtr(h.UpdatedAt)}
}

func authorizeHolidayAdmin(actor Actor) error {
	if strings.ToUpper(strings.TrimSpace(actor.Role)) != "ADMIN" {
		return ErrNotAuthorized
	}
	return nil
}

func validateHolidayName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(name); n < 1 || n > holidayNameMax {
		return "", &ValidationError{Field: "name", Message: fmt.Sprintf("nama 1-%d karakter", holidayNameMax)}
	}
	return name, nil
}

// List: GET /api/v1/admin/holidays?year=&status=all|active|inactive&page=&page_size=.
func (s *HolidayAdminService) List(ctx context.Context, actor Actor, year *int32, status string, page, pageSize int) (*HolidayList, error) {
	if err := authorizeHolidayAdmin(actor); err != nil {
		return nil, err
	}
	if status != "active" && status != "inactive" {
		status = "all"
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > holidayMaxPageSize {
		pageSize = 50
	}
	rows, err := s.read.ListHolidaysAdmin(ctx, db.ListHolidaysAdminParams{Year: year, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize)})
	if err != nil {
		return nil, fmt.Errorf("list holidays: %w", err)
	}
	total, err := s.read.CountHolidaysAdmin(ctx, db.CountHolidaysAdminParams{Year: year, Status: status})
	if err != nil {
		return nil, fmt.Errorf("count holidays: %w", err)
	}
	items := make([]HolidayView, len(rows))
	for i, h := range rows {
		items[i] = holidayView(h)
	}
	return &HolidayList{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// inTx runs fn + its audit entry in one transaction (GR#3 deviation: audit failure rolls back).
func (s *HolidayAdminService) inTx(ctx context.Context, actor Actor, fn func(q *db.Queries) (audit.Entry, error)) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	entry, err := fn(db.New(tx))
	if err != nil {
		if isUniqueViolation(err) {
			return ErrHolidayDateConflict // holidays_active_date_uq backstops the pre-check
		}
		return err
	}
	entry.ActorID, entry.IP, entry.EntityType = actor.UserID, actor.IP, holidayEntity
	if err := audit.NewWriter(tx).Write(ctx, entry); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return tx.Commit(ctx)
}

// requireFreeDate fails with ErrHolidayDateConflict when an active holiday already has the date.
func requireFreeDate(ctx context.Context, q *db.Queries, d time.Time) error {
	_, err := q.FindActiveHolidayByDate(ctx, toPgDate(d))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("check holiday date: %w", err)
	default:
		return ErrHolidayDateConflict
	}
}

// Create: POST /api/v1/admin/holidays {holiday_date, name}.
func (s *HolidayAdminService) Create(ctx context.Context, actor Actor, date time.Time, rawName string) (*HolidayView, error) {
	if err := authorizeHolidayAdmin(actor); err != nil {
		return nil, err
	}
	name, err := validateHolidayName(rawName)
	if err != nil {
		return nil, err
	}
	var out db.Holiday
	err = s.inTx(ctx, actor, func(q *db.Queries) (audit.Entry, error) {
		if err := requireFreeDate(ctx, q, date); err != nil {
			return audit.Entry{}, err
		}
		created, err := q.CreateHoliday(ctx, db.CreateHolidayParams{HolidayDate: toPgDate(date), Name: name, CreatedBy: actor.UserID})
		if err != nil {
			return audit.Entry{}, err
		}
		out = created
		return audit.Entry{Action: "holiday_created", EntityID: created.ID, After: holidayView(created)}, nil
	})
	if err != nil {
		return nil, err
	}
	v := holidayView(out)
	return &v, nil
}

func loadHoliday(ctx context.Context, q *db.Queries, id int64) (db.Holiday, error) {
	h, err := q.GetHolidayAdminByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return h, ErrHolidayNotFound
	}
	if err != nil {
		return h, fmt.Errorf("load holiday %d: %w", id, err)
	}
	return h, nil
}

// UpdateName: PUT /api/v1/admin/holidays/{id} {name}. The date is fixed; a wrong date = disable + create.
func (s *HolidayAdminService) UpdateName(ctx context.Context, actor Actor, id int64, rawName string) (*HolidayView, error) {
	if err := authorizeHolidayAdmin(actor); err != nil {
		return nil, err
	}
	name, err := validateHolidayName(rawName)
	if err != nil {
		return nil, err
	}
	var out db.Holiday
	err = s.inTx(ctx, actor, func(q *db.Queries) (audit.Entry, error) {
		before, err := loadHoliday(ctx, q, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if out, err = q.UpdateHolidayName(ctx, db.UpdateHolidayNameParams{Name: name, ID: id}); err != nil {
			return audit.Entry{}, fmt.Errorf("update holiday %d: %w", id, err)
		}
		return audit.Entry{Action: "holiday_updated", EntityID: id,
			Before: map[string]any{"name": before.Name}, After: map[string]any{"name": out.Name}}, nil
	})
	if err != nil {
		return nil, err
	}
	v := holidayView(out)
	return &v, nil
}

// SetActive: POST /{id}/disable|enable. Enabling is refused when another active row has the date.
func (s *HolidayAdminService) SetActive(ctx context.Context, actor Actor, id int64, active bool) (*HolidayView, error) {
	if err := authorizeHolidayAdmin(actor); err != nil {
		return nil, err
	}
	var out db.Holiday
	err := s.inTx(ctx, actor, func(q *db.Queries) (audit.Entry, error) {
		before, err := loadHoliday(ctx, q, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if before.IsActive == active {
			return audit.Entry{}, ErrHolidayStatusUnchanged
		}
		if active {
			if err := requireFreeDate(ctx, q, before.HolidayDate.Time); err != nil {
				return audit.Entry{}, err
			}
		}
		if out, err = q.SetHolidayActive(ctx, db.SetHolidayActiveParams{IsActive: active, ID: id}); err != nil {
			return audit.Entry{}, fmt.Errorf("set holiday %d active=%v: %w", id, active, err)
		}
		action := "holiday_disabled"
		if active {
			action = "holiday_enabled"
		}
		return audit.Entry{Action: action, EntityID: id,
			Before: map[string]any{"is_active": before.IsActive}, After: map[string]any{"is_active": out.IsActive}}, nil
	})
	if err != nil {
		return nil, err
	}
	v := holidayView(out)
	return &v, nil
}

type holidayRow struct {
	line int
	date time.Time
	name string
}

// parseHolidayCSV reads "holiday_date,name" (YYYY-MM-DD, 1-100 chars). Any bad row fails the whole
// file (FR1.3 all-or-nothing); the message names the line.
func parseHolidayCSV(r io.Reader) ([]holidayRow, error) {
	invalid := func(format string, args ...any) error {
		return &ValidationError{Field: "file", Message: fmt.Sprintf(format, args...)}
	}
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, invalid("CSV tidak dapat dibaca: %v", err)
	}
	if len(records) == 0 || len(records[0]) != 2 ||
		strings.TrimPrefix(strings.TrimSpace(records[0][0]), utf8BOM) != "holiday_date" || strings.TrimSpace(records[0][1]) != "name" {
		return nil, invalid("baris 1 harus header holiday_date,name")
	}
	var rows []holidayRow
	seen := map[string]int{}
	for i, rec := range records[1:] {
		line := i + 2
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue
		}
		if len(rec) != 2 {
			return nil, invalid("baris %d: harus 2 kolom", line)
		}
		d, err := time.Parse("2006-01-02", strings.TrimSpace(rec[0]))
		if err != nil {
			return nil, invalid("baris %d: tanggal harus YYYY-MM-DD", line)
		}
		name, err := validateHolidayName(rec[1])
		if err != nil {
			return nil, invalid("baris %d: nama 1-%d karakter", line, holidayNameMax)
		}
		key := d.Format("2006-01-02")
		if prev, ok := seen[key]; ok {
			return nil, invalid("baris %d: tanggal %s sudah ada di baris %d", line, key, prev)
		}
		seen[key] = line
		rows = append(rows, holidayRow{line: line, date: d, name: name})
	}
	if len(rows) == 0 {
		return nil, invalid("file tidak berisi baris hari libur")
	}
	if len(rows) > holidayImportMax {
		return nil, invalid("maksimal %d baris per file", holidayImportMax)
	}
	return rows, nil
}

// Import: POST /api/v1/admin/holidays/import (CSV). One transaction, one audit entry; dates that are
// already active are skipped and reported (FR1.3).
func (s *HolidayAdminService) Import(ctx context.Context, actor Actor, r io.Reader) (*HolidayImportResult, error) {
	if err := authorizeHolidayAdmin(actor); err != nil {
		return nil, err
	}
	rows, err := parseHolidayCSV(r)
	if err != nil {
		return nil, err
	}
	res := &HolidayImportResult{Skipped: []string{}}
	err = s.inTx(ctx, actor, func(q *db.Queries) (audit.Entry, error) {
		created := []string{}
		for _, row := range rows {
			key := row.date.Format("2006-01-02")
			switch err := requireFreeDate(ctx, q, row.date); {
			case errors.Is(err, ErrHolidayDateConflict):
				res.Skipped = append(res.Skipped, key)
				continue
			case err != nil:
				return audit.Entry{}, err
			}
			if _, err := q.CreateHoliday(ctx, db.CreateHolidayParams{HolidayDate: toPgDate(row.date), Name: row.name, CreatedBy: actor.UserID}); err != nil {
				return audit.Entry{}, fmt.Errorf("baris %d: %w", row.line, err)
			}
			created = append(created, key)
		}
		res.Created = len(created)
		return audit.Entry{Action: "holiday_imported", After: map[string]any{"created": created, "skipped": res.Skipped}}, nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
