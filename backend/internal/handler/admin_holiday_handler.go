package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// Hari libur admin (replenishment-realisasi FR1): ADMIN only at the route and in the service.
// Changes apply immediately with audit in the same transaction (documented GR#3 deviation).

const holidayImportMaxBytes = 1 << 20 // 1 MB: <= 1000 short CSV rows

// HolidayAdminer is the slice of service.HolidayAdminService behind these routes.
type HolidayAdminer interface {
	List(ctx context.Context, actor service.Actor, year *int32, status string, page, pageSize int) (*service.HolidayList, error)
	Create(ctx context.Context, actor service.Actor, date time.Time, name string) (*service.HolidayView, error)
	UpdateName(ctx context.Context, actor service.Actor, id int64, name string) (*service.HolidayView, error)
	SetActive(ctx context.Context, actor service.Actor, id int64, active bool) (*service.HolidayView, error)
	Import(ctx context.Context, actor service.Actor, r io.Reader) (*service.HolidayImportResult, error)
}

type AdminHolidayHandler struct {
	svc HolidayAdminer
}

func NewAdminHolidayHandler(svc HolidayAdminer) *AdminHolidayHandler {
	return &AdminHolidayHandler{svc: svc}
}

func (h *AdminHolidayHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequireRoles("ADMIN"))
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Post("/import", h.Import)
	r.Put("/{id}", h.UpdateName)
	r.Post("/{id}/disable", h.setActive(false))
	r.Post("/{id}/enable", h.setActive(true))
	return r
}

func (h *AdminHolidayHandler) respond(w http.ResponseWriter, status int, out any, err error) {
	var ve *service.ValidationError
	switch {
	case err == nil:
		writeJSON(w, status, out)
	case errors.As(err, &ve):
		writeValidationError(w, ve.Field, ve.Message)
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Hanya ADMIN yang dapat mengelola hari libur")
	case errors.Is(err, service.ErrHolidayNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Hari libur tidak ditemukan")
	case errors.Is(err, service.ErrHolidayDateConflict):
		writeError(w, http.StatusConflict, "conflict", "Sudah ada hari libur aktif pada tanggal tersebut")
	case errors.Is(err, service.ErrHolidayStatusUnchanged):
		writeError(w, http.StatusConflict, "conflict", "Status hari libur sudah sesuai")
	default:
		writeUnexpectedError(w, err)
	}
}

// List handles GET /?year=&status=&page=&page_size=.
func (h *AdminHolidayHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	q := r.URL.Query()
	var year *int32
	if raw := q.Get("year"); raw != "" {
		y, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || y < 2000 || y > 2100 {
			writeValidationError(w, "year", "tahun tidak valid")
			return
		}
		y32 := int32(y)
		year = &y32
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page > maxPage {
		writeValidationError(w, "page", "maksimal "+strconv.Itoa(maxPage))
		return
	}
	size, _ := strconv.Atoi(q.Get("page_size"))
	out, err := h.svc.List(r.Context(), actor, year, q.Get("status"), page, size)
	h.respond(w, http.StatusOK, out, err)
}

// Create handles POST / {holiday_date: "YYYY-MM-DD", name}.
func (h *AdminHolidayHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	var body struct {
		HolidayDate string `json:"holiday_date"`
		Name        string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}
	d, err := time.Parse("2006-01-02", body.HolidayDate)
	if err != nil {
		writeValidationError(w, "holiday_date", "format tanggal YYYY-MM-DD")
		return
	}
	out, err := h.svc.Create(r.Context(), actor, d, body.Name)
	h.respond(w, http.StatusCreated, out, err)
}

func holidayID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "id tidak valid")
		return 0, false
	}
	return id, true
}

// UpdateName handles PUT /{id} {name}.
func (h *AdminHolidayHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, ok := holidayID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}
	out, err := h.svc.UpdateName(r.Context(), actor, id, body.Name)
	h.respond(w, http.StatusOK, out, err)
}

func (h *AdminHolidayHandler) setActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFromRequest(r)
		if !ok {
			writeUnauthorized(w, "Token tidak valid")
			return
		}
		id, ok := holidayID(w, r)
		if !ok {
			return
		}
		out, err := h.svc.SetActive(r.Context(), actor, id, active)
		h.respond(w, http.StatusOK, out, err)
	}
}

// Import handles POST /import, multipart field "file" (CSV holiday_date,name).
func (h *AdminHolidayHandler) Import(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, holidayImportMaxBytes)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeValidationError(w, "file", "unggah file CSV (maks 1 MB) pada field file")
		return
	}
	defer func() { _ = file.Close() }()
	out, err := h.svc.Import(r.Context(), actor, file)
	h.respond(w, http.StatusOK, out, err)
}
