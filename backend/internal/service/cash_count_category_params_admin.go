package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

const cashCountCategoryParamsEntity = "cash_count_category_params"

// Limits of cash_count_category_params (migration 041, AC-5.1-03).
const (
	cashCountOnsiteMax      = 6
	cashCountAmountScale    = 2  // numeric(20,2)
	cashCountAmountIntDigit = 18 // 20 - 2
)

// ErrCashCountCategoryParamsMissing: no active set (seeded by 041, so only a broken DB).
var ErrCashCountCategoryParamsMissing = errors.New("set parameter kategori vault aktif tidak ditemukan")

// CashCountCategoryParamsAdminRepo is the read surface the service needs.
// *repository.CashCountCategoryParamsAdminRepository satisfies it.
type CashCountCategoryParamsAdminRepo interface {
	List(ctx context.Context) ([]db.CashCountCategoryParam, error)
	GetActive(ctx context.Context) (*db.CashCountCategoryParam, error)
}

// CashCountCategoryParamsPayload is the op=create payload (= propose a new set).
// Amounts are exact decimal strings (never float). SupersedesID is set by the
// server to the active set at submit time; the applier refuses to apply if
// another set became active meanwhile (create ops skip the engine's Before
// staleness check).
type CashCountCategoryParamsPayload struct {
	MediumMinAmount string `json:"medium_min_amount"`
	MediumMaxAmount string `json:"medium_max_amount"`
	CurrencyCode    string `json:"currency_code"`
	OnsiteHigh      int16  `json:"onsite_high"`
	OnsiteMedium    int16  `json:"onsite_medium"`
	OnsiteLow       int16  `json:"onsite_low"`
	SupersedesID    int64  `json:"supersedes_id"`
}

// CashCountCategoryParamsAdminService validates "Parameter Kategori Vault"
// (ScDD 5.1, AC-5.1-01/03/04) and stages a new set via Submit (which re-checks
// the CC-CWM maker role); CashCountCategoryParamsApplier writes it once approved.
type CashCountCategoryParamsAdminService struct {
	repo    CashCountCategoryParamsAdminRepo
	changes VendorVaultSubmitter
}

// NewCashCountCategoryParamsAdminService creates the service.
func NewCashCountCategoryParamsAdminService(repo CashCountCategoryParamsAdminRepo, changes VendorVaultSubmitter) *CashCountCategoryParamsAdminService {
	return &CashCountCategoryParamsAdminService{repo: repo, changes: changes}
}

// List returns the active set first, then history (replica). Read-only.
func (s *CashCountCategoryParamsAdminService) List(ctx context.Context) ([]db.CashCountCategoryParam, error) {
	return s.repo.List(ctx)
}

// Create validates and stages a new set; Before = the active set it replaces.
func (s *CashCountCategoryParamsAdminService) Create(ctx context.Context, makerID int64, req CashCountCategoryParamsPayload, actorIP string) (db.MasterDataChangeRequest, error) {
	p, err := normalizeCashCountCategoryParams(req)
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}
	active, err := s.repo.GetActive(ctx)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading active cash count category params: %w", err)
	}
	if active == nil {
		return db.MasterDataChangeRequest{}, ErrCashCountCategoryParamsMissing
	}
	p.SupersedesID = active.ID
	return s.changes.Submit(ctx, makerID, SubmitRequest{EntityType: cashCountCategoryParamsEntity, Op: "create", Payload: p, Before: active}, actorIP)
}

// normalizeCashCountCategoryParams applies AC-5.1-03 and returns the payload
// with canonical amounts ("5000000000.00") and currency. Client SupersedesID is ignored.
func normalizeCashCountCategoryParams(req CashCountCategoryParamsPayload) (CashCountCategoryParamsPayload, error) {
	minR, err := parseCashCountAmount("medium_min_amount", req.MediumMinAmount)
	if err != nil {
		return req, err
	}
	maxR, err := parseCashCountAmount("medium_max_amount", req.MediumMaxAmount)
	if err != nil {
		return req, err
	}
	if minR.Sign() <= 0 {
		return req, &ValidationError{Field: "medium_min_amount", Message: "harus lebih besar dari 0"}
	}
	if minR.Cmp(maxR) >= 0 {
		return req, &ValidationError{Field: "medium_max_amount", Message: "harus lebih besar dari batas bawah Medium"}
	}
	currency := strings.ToUpper(strings.TrimSpace(req.CurrencyCode))
	if currency == "" {
		currency = "IDR"
	}
	if currency != "IDR" {
		return req, &ValidationError{Field: "currency_code", Message: "hanya IDR yang didukung"}
	}
	for _, f := range []struct {
		name string
		v    int16
	}{{"onsite_high", req.OnsiteHigh}, {"onsite_medium", req.OnsiteMedium}, {"onsite_low", req.OnsiteLow}} {
		if f.v < 0 || f.v > cashCountOnsiteMax {
			return req, &ValidationError{Field: f.name, Message: "jumlah kunjungan onsite harus 0–6 per 6 bulan"}
		}
	}
	return CashCountCategoryParamsPayload{
		MediumMinAmount: minR.FloatString(cashCountAmountScale),
		MediumMaxAmount: maxR.FloatString(cashCountAmountScale),
		CurrencyCode:    currency,
		OnsiteHigh:      req.OnsiteHigh,
		OnsiteMedium:    req.OnsiteMedium,
		OnsiteLow:       req.OnsiteLow,
	}, nil
}

// parseCashCountAmount parses a required exact decimal that fits numeric(20,2)
// without rounding: at most 2 decimals, at most 18 integer digits. No float.
func parseCashCountAmount(field, v string) (*big.Rat, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil, &ValidationError{Field: field, Message: "wajib diisi"}
	}
	r, err := parseDecimalField(field, &s)
	if err != nil {
		return nil, err
	}
	scaled := new(big.Rat).Mul(r, big.NewRat(100, 1))
	if !scaled.IsInt() {
		return nil, &ValidationError{Field: field, Message: "maksimal 2 angka di belakang koma"}
	}
	limit := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(cashCountAmountIntDigit), nil))
	if new(big.Rat).Abs(r).Cmp(limit) >= 0 {
		return nil, &ValidationError{Field: field, Message: "nilai terlalu besar"}
	}
	return r, nil
}
