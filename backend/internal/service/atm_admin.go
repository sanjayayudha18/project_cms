package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for ATMAdminService. Handlers map these to HTTP status
// codes per design.md's error table (Req 3.3-3.9, 4.2-4.7):
// ErrATMNotFound -> 404, ErrATMTerminalIDConflict -> 409,
// ErrATMTerminalIDImmutable -> 400, ErrATMInvalidReference -> 400
// (invalid_reference).
//
// ErrATMNotFound is not redeclared here -- it already exists as a
// package-level var in atm_portal_profile.go with the same meaning ("no
// such ATM"), and Go disallows redeclaring a var of the same name in one
// package, so ATMAdminService reuses that existing sentinel directly.
var (
	ErrATMTerminalIDConflict  = errors.New("atm terminal_id already exists")
	ErrATMTerminalIDImmutable = errors.New("atm terminal_id cannot be changed")
	ErrATMInvalidReference    = errors.New("referenced location does not exist")
)

var validATMPriorityClasses = map[string]bool{"VIP": true, "Non VIP": true, "Industri": true}

// ATMAdminSubmitter is the narrow MasterDataChangeService surface
// ATMAdminService needs (maker-checker, D1 / plan.md T4.2).
type ATMAdminSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// ATMAdminRepo is the read-only repository surface ATMAdminService needs.
// *repository.ATMAdminRepository satisfies this automatically. Deliberately
// has no write methods: every ATM mutation goes through
// MasterDataChangeService.Submit and is applied by ATMApplier.
type ATMAdminRepo interface {
	List(ctx context.Context, arg db.ListATMsAdminParams) ([]db.ListATMsAdminRow, error)
	Count(ctx context.Context, arg db.CountATMsAdminParams) (int64, error)
	GetByID(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error)
	FindByTerminalID(ctx context.Context, terminalID string) (*int64, error)
	ListLocations(ctx context.Context) ([]db.ListLocationsForSelectRow, error)
	LocationExists(ctx context.Context, locationID int64) (bool, error)
}

// ATMAdminService owns validation, reference resolution and uniqueness
// pre-checks for ATM create/update/disable/enable (Req 3-5) and stages each
// change via MasterDataChangeService.Submit. Since T4.2 nothing is applied
// here: the atms row is written by ATMApplier once the change is approved,
// and the audit trail is the engine's (submit + apply), not this service's.
type ATMAdminService struct {
	repo    ATMAdminRepo
	changes ATMAdminSubmitter
}

// NewATMAdminService creates an ATMAdminService with the given dependencies.
func NewATMAdminService(repo ATMAdminRepo, changes ATMAdminSubmitter) *ATMAdminService {
	return &ATMAdminService{repo: repo, changes: changes}
}

// ATM is the admin-facing ATM record: money as decimal strings and
// timestamps as *time.Time, never a lossy float path (same convention as
// ATMProfileResult in atm_portal_profile.go).
type ATM struct {
	ID                      int64
	TerminalID              string
	LocationID              int64
	LocationName            *string
	MachineType             string
	Brand                   string
	Model                   string
	OperationHours          string
	DeploymentType          string
	CapacityAmount          *string
	LowThresholdAmount      *string
	CriticalThresholdAmount *string
	Blacklisted             bool
	EscrowAccount           *string
	PriorityClass           *string
	IsActive                bool
	CreatedAt               *time.Time
	UpdatedAt               *time.Time
	DeletedAt               *time.Time
}

// LocationOption is one entry of the Location select backing data (Req 6).
type LocationOption struct {
	ID            int64
	Name          string
	CityOrRegency string
	Province      string
}

// CreateATMRequest holds the data for a POST /api/v1/admin/atms request
// (Req 3.1). Monetary fields are decimal strings, nil meaning "not
// provided" (stored as SQL NULL).
type CreateATMRequest struct {
	TerminalID              string
	LocationID              int64
	MachineType             string
	Brand                   string
	Model                   string
	OperationHours          string
	DeploymentType          string
	CapacityAmount          *string
	LowThresholdAmount      *string
	CriticalThresholdAmount *string
	Blacklisted             bool
	EscrowAccount           *string
	PriorityClass           *string
}

// UpdateATMRequest holds the data for a PUT /api/v1/admin/atms/{id} request
// (Req 4.1). TerminalID is accepted only so an attempt to change it can be
// detected and rejected (Req 4.2) -- nil means the field was not sent at all.
type UpdateATMRequest struct {
	TerminalID              *string
	LocationID              int64
	MachineType             string
	Brand                   string
	Model                   string
	OperationHours          string
	DeploymentType          string
	CapacityAmount          *string
	LowThresholdAmount      *string
	CriticalThresholdAmount *string
	Blacklisted             bool
	EscrowAccount           *string
	PriorityClass           *string
}

// atmUpdatePayload is the jsonb payload ATMApplier unmarshals for op=update,
// and the embedded tail of atmCreatePayload. terminal_id is deliberately not
// part of it: it is immutable after create. Money travels as trimmed decimal
// strings (never float) and is parsed to numeric only in the applier; blank
// optionals are nil.
type atmUpdatePayload struct {
	LocationID              int64   `json:"location_id"`
	MachineType             string  `json:"machine_type"`
	Brand                   string  `json:"brand"`
	Model                   string  `json:"model"`
	OperationHours          string  `json:"operation_hours"`
	DeploymentType          string  `json:"deployment_type"`
	CapacityAmount          *string `json:"capacity_amount"`
	LowThresholdAmount      *string `json:"low_threshold_amount"`
	CriticalThresholdAmount *string `json:"critical_threshold_amount"`
	Blacklisted             bool    `json:"blacklisted"`
	EscrowAccount           *string `json:"escrow_account"`
	PriorityClass           *string `json:"priority_class"`
}

// atmCreatePayload is the jsonb payload ATMApplier unmarshals for op=create.
type atmCreatePayload struct {
	TerminalID string `json:"terminal_id"`
	atmUpdatePayload
}

// List returns a page of ATMs matching the given filters (Req 8). Read-only,
// never writes an audit entry (Req 6.3, 7.4).
func (s *ATMAdminService) List(ctx context.Context, arg db.ListATMsAdminParams) ([]ATM, error) {
	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]ATM, len(rows))
	for i, row := range rows {
		atm, err := atmFromListRow(row)
		if err != nil {
			return nil, err
		}
		out[i] = atm
	}
	return out, nil
}

// Count returns the total number of ATMs matching the given filters, without
// pagination. Read-only, never writes an audit entry.
func (s *ATMAdminService) Count(ctx context.Context, arg db.CountATMsAdminParams) (int64, error) {
	return s.repo.Count(ctx, arg)
}

// Get returns an ATM by id, including soft-deleted rows. Returns
// ErrATMNotFound if no matching ATM exists. Read-only, never writes an audit
// entry.
func (s *ATMAdminService) Get(ctx context.Context, id int64) (ATM, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ATM{}, fmt.Errorf("loading atm: %w", err)
	}
	if row == nil {
		return ATM{}, ErrATMNotFound
	}
	return atmFromGetRow(*row)
}

// ListLocations returns every location for the ATM form's Location select
// (Req 6.1-6.2), ordered by name. Read-only, never writes an audit entry.
func (s *ATMAdminService) ListLocations(ctx context.Context) ([]LocationOption, error) {
	rows, err := s.repo.ListLocations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LocationOption, len(rows))
	for i, row := range rows {
		out[i] = LocationOption{ID: row.ID, Name: row.Name, CityOrRegency: row.CityOrRegency, Province: row.Province}
	}
	return out, nil
}

// Create validates and stages a new ATM (Req 3.1-3.9). The terminal_id
// uniqueness check here is an early answer only; the DB unique constraint is
// re-enforced at apply time (a duplicate that was pending twice surfaces then
// as ErrATMTerminalIDConflict).
func (s *ATMAdminService) Create(ctx context.Context, actorID int64, req CreateATMRequest, actorIP string) (db.MasterDataChangeRequest, error) {
	terminalID := strings.TrimSpace(req.TerminalID)
	if terminalID == "" {
		return db.MasterDataChangeRequest{}, &ValidationError{Field: "terminal_id", Message: "wajib diisi"}
	}
	payload, err := s.validateEditable(ctx, atmEditableInput{
		locationID: req.LocationID, machineType: req.MachineType, brand: req.Brand, model: req.Model,
		operationHours: req.OperationHours, deploymentType: req.DeploymentType, priorityClass: req.PriorityClass,
		capacity: req.CapacityAmount, lowThreshold: req.LowThresholdAmount, criticalThreshold: req.CriticalThresholdAmount,
		blacklisted: req.Blacklisted, escrowAccount: req.EscrowAccount,
	})
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}

	existing, err := s.repo.FindByTerminalID(ctx, terminalID)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("checking terminal_id uniqueness: %w", err)
	}
	if existing != nil {
		return db.MasterDataChangeRequest{}, ErrATMTerminalIDConflict
	}

	return s.changes.Submit(ctx, actorID, SubmitRequest{
		EntityType: "atm", Op: "create",
		Payload: atmCreatePayload{TerminalID: terminalID, atmUpdatePayload: payload},
	}, actorIP)
}

// Update validates and stages an overwrite of an ATM's editable fields,
// rejecting any attempt to change terminal_id (Req 4.1-4.7). A missing or
// soft-disabled target id is a 404 (ErrATMNotFound) -- a disabled ATM must be
// enabled before it can be edited. The ATM row as loaded here is the change's
// "before" snapshot, so an edit that lands after someone else changed the ATM
// is marked stale rather than silently overwriting (T2.5).
func (s *ATMAdminService) Update(ctx context.Context, actorID, id int64, req UpdateATMRequest, actorIP string) (db.MasterDataChangeRequest, error) {
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading atm: %w", err)
	}
	if before == nil || before.DeletedAt.Valid {
		return db.MasterDataChangeRequest{}, ErrATMNotFound
	}
	if req.TerminalID != nil && strings.TrimSpace(*req.TerminalID) != before.TerminalID {
		return db.MasterDataChangeRequest{}, ErrATMTerminalIDImmutable
	}

	payload, err := s.validateEditable(ctx, atmEditableInput{
		locationID: req.LocationID, machineType: req.MachineType, brand: req.Brand, model: req.Model,
		operationHours: req.OperationHours, deploymentType: req.DeploymentType, priorityClass: req.PriorityClass,
		capacity: req.CapacityAmount, lowThreshold: req.LowThresholdAmount, criticalThreshold: req.CriticalThresholdAmount,
		blacklisted: req.Blacklisted, escrowAccount: req.EscrowAccount,
	})
	if err != nil {
		return db.MasterDataChangeRequest{}, err
	}

	return s.changes.Submit(ctx, actorID, SubmitRequest{EntityType: "atm", Op: "update", EntityID: &id, Before: before, Payload: payload}, actorIP)
}

// Disable stages a soft-disable of an ATM (Req 5.1, 5.4-5.5). A non-existent
// id is a 404 and stages nothing.
func (s *ATMAdminService) Disable(ctx context.Context, actorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, actorID, id, "disable", actorIP)
}

// Enable stages the reverse of Disable (Req 5.2, 5.6). A non-existent id is a
// 404 and stages nothing.
func (s *ATMAdminService) Enable(ctx context.Context, actorID, id int64, actorIP string) (db.MasterDataChangeRequest, error) {
	return s.toggle(ctx, actorID, id, "enable", actorIP)
}

func (s *ATMAdminService) toggle(ctx context.Context, actorID, id int64, op, actorIP string) (db.MasterDataChangeRequest, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return db.MasterDataChangeRequest{}, fmt.Errorf("loading atm: %w", err)
	}
	if existing == nil {
		return db.MasterDataChangeRequest{}, ErrATMNotFound
	}
	return s.changes.Submit(ctx, actorID, SubmitRequest{EntityType: "atm", Op: op, EntityID: &id, Before: existing}, actorIP)
}

// atmEditableInput is the raw editable field set shared by Create and Update.
type atmEditableInput struct {
	locationID                                                              int64
	machineType, brand, model, operationHours, deploymentType               string
	priorityClass, capacity, lowThreshold, criticalThreshold, escrowAccount *string
	blacklisted                                                             bool
}

// validateEditable runs the Req 3.2/3.5/3.6 (and identical 4.1/4.5)
// validation shared by Create and Update -- required text fields, priority
// enum, non-negative exact decimals -- checks the location reference, and
// returns the normalized (trimmed, blank->nil) payload to stage.
func (s *ATMAdminService) validateEditable(ctx context.Context, in atmEditableInput) (atmUpdatePayload, error) {
	fields, priorityClass, _, _, _, err := validateATMEditableFields(
		in.locationID, in.machineType, in.brand, in.model, in.operationHours, in.deploymentType,
		in.priorityClass, in.capacity, in.lowThreshold, in.criticalThreshold,
	)
	if err != nil {
		return atmUpdatePayload{}, err
	}

	exists, err := s.repo.LocationExists(ctx, fields.locationID)
	if err != nil {
		return atmUpdatePayload{}, fmt.Errorf("checking location reference: %w", err)
	}
	if !exists {
		return atmUpdatePayload{}, ErrATMInvalidReference
	}

	return atmUpdatePayload{
		LocationID: fields.locationID, MachineType: fields.machineType, Brand: fields.brand, Model: fields.model,
		OperationHours: fields.operationHours, DeploymentType: fields.deploymentType,
		CapacityAmount: trimOptional(in.capacity), LowThresholdAmount: trimOptional(in.lowThreshold),
		CriticalThresholdAmount: trimOptional(in.criticalThreshold), Blacklisted: in.blacklisted,
		EscrowAccount: trimOptional(in.escrowAccount), PriorityClass: priorityClass,
	}, nil
}

// atmEditableFields holds the trimmed, presence-validated shared fields
// between Create and Update (everything except terminal_id, which each
// caller validates on its own terms -- required-and-immutable on create,
// rejected on change for update).
type atmEditableFields struct {
	locationID     int64
	machineType    string
	brand          string
	model          string
	operationHours string
	deploymentType string
}

// validateATMEditableFields runs the Req 3.2/3.5/3.6 (and identical
// 4.1/4.5) validation shared by Create and Update: required text fields
// present, priority_class enum, and each monetary amount parses as a
// non-negative exact decimal. Returns the parsed pgtype.Numeric values
// (the staging path only needs the validity check and re-parses in
// ATMApplier).
func validateATMEditableFields(
	locationID int64, machineType, brand, model, operationHours, deploymentType string,
	priorityClass, capacityAmount, lowThresholdAmount, criticalThresholdAmount *string,
) (atmEditableFields, *string, pgtype.Numeric, pgtype.Numeric, pgtype.Numeric, error) {
	var zero pgtype.Numeric

	if locationID <= 0 {
		return atmEditableFields{}, nil, zero, zero, zero, &ValidationError{Field: "location_id", Message: "wajib diisi"}
	}
	machineType = strings.TrimSpace(machineType)
	if machineType == "" {
		return atmEditableFields{}, nil, zero, zero, zero, &ValidationError{Field: "machine_type", Message: "wajib diisi"}
	}
	brand = strings.TrimSpace(brand)
	if brand == "" {
		return atmEditableFields{}, nil, zero, zero, zero, &ValidationError{Field: "brand", Message: "wajib diisi"}
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return atmEditableFields{}, nil, zero, zero, zero, &ValidationError{Field: "model", Message: "wajib diisi"}
	}
	operationHours = strings.TrimSpace(operationHours)
	if operationHours == "" {
		return atmEditableFields{}, nil, zero, zero, zero, &ValidationError{Field: "operation_hours", Message: "wajib diisi"}
	}
	deploymentType = strings.TrimSpace(deploymentType)
	if deploymentType == "" {
		return atmEditableFields{}, nil, zero, zero, zero, &ValidationError{Field: "deployment_type", Message: "wajib diisi"}
	}

	priority, err := validateATMPriorityClass(priorityClass)
	if err != nil {
		return atmEditableFields{}, nil, zero, zero, zero, err
	}

	capacity, err := parseNonNegativeDecimal("capacity_amount", capacityAmount)
	if err != nil {
		return atmEditableFields{}, nil, zero, zero, zero, err
	}
	lowThreshold, err := parseNonNegativeDecimal("low_threshold_amount", lowThresholdAmount)
	if err != nil {
		return atmEditableFields{}, nil, zero, zero, zero, err
	}
	criticalThreshold, err := parseNonNegativeDecimal("critical_threshold_amount", criticalThresholdAmount)
	if err != nil {
		return atmEditableFields{}, nil, zero, zero, zero, err
	}

	return atmEditableFields{
		locationID:     locationID,
		machineType:    machineType,
		brand:          brand,
		model:          model,
		operationHours: operationHours,
		deploymentType: deploymentType,
	}, priority, capacity, lowThreshold, criticalThreshold, nil
}

// validateATMPriorityClass enforces the Req 3.5 enum when a value is
// provided; nil or blank means "not provided" (stored as SQL NULL).
func validateATMPriorityClass(priorityClass *string) (*string, error) {
	if priorityClass == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*priorityClass)
	if trimmed == "" {
		return nil, nil
	}
	if !validATMPriorityClasses[trimmed] {
		return nil, &ValidationError{Field: "priority_class", Message: "harus salah satu dari VIP, Non VIP, Industri"}
	}
	return &trimmed, nil
}

// parseNonNegativeDecimal enforces Req 3.6: when a monetary amount is
// provided, it must parse as an exact decimal and be non-negative --
// rejected as a 422 ValidationError otherwise. nil or blank means "not
// provided" (stored as SQL NULL, matching the nullable capacity_amount/
// low_threshold_amount/critical_threshold_amount columns). Uses big.Rat to
// check validity/sign without ever routing the value through float64 (same
// convention as validatePolicyRequest in rbac_list_handler.go).
func parseNonNegativeDecimal(field string, amount *string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if amount == nil {
		return n, nil
	}
	trimmed := strings.TrimSpace(*amount)
	if trimmed == "" {
		return n, nil
	}
	rat, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return n, &ValidationError{Field: field, Message: "harus berupa angka desimal yang valid"}
	}
	if rat.Sign() < 0 {
		return n, &ValidationError{Field: field, Message: "tidak boleh negatif"}
	}
	if err := n.Scan(trimmed); err != nil {
		return n, &ValidationError{Field: field, Message: "harus berupa angka desimal yang valid"}
	}
	return n, nil
}

// atmFromListRow/atmFromGetRow convert the sqlc row types (identical field
// sets) into the shared ATM domain struct, decimal-stringifying money via the
// existing numericToDecimalStringPtr helper.

func atmFromListRow(row db.ListATMsAdminRow) (ATM, error) {
	capacity, low, critical, err := decimalStrings(row.CapacityAmount, row.LowThresholdAmount, row.CriticalThresholdAmount)
	if err != nil {
		return ATM{}, err
	}
	return ATM{
		ID: row.ID, TerminalID: row.TerminalID, LocationID: row.LocationID, LocationName: row.LocationName,
		MachineType: row.MachineType, Brand: row.Brand, Model: row.Model, OperationHours: row.OperationHours,
		DeploymentType: row.DeploymentType, CapacityAmount: capacity, LowThresholdAmount: low,
		CriticalThresholdAmount: critical, Blacklisted: row.Blacklisted, EscrowAccount: row.EscrowAccount,
		PriorityClass: row.PriorityClass, IsActive: row.IsActive,
		CreatedAt: timestamptzToPtr(row.CreatedAt), UpdatedAt: timestamptzToPtr(row.UpdatedAt), DeletedAt: timestamptzToPtr(row.DeletedAt),
	}, nil
}

func atmFromGetRow(row db.GetATMAdminByIDRow) (ATM, error) {
	capacity, low, critical, err := decimalStrings(row.CapacityAmount, row.LowThresholdAmount, row.CriticalThresholdAmount)
	if err != nil {
		return ATM{}, err
	}
	return ATM{
		ID: row.ID, TerminalID: row.TerminalID, LocationID: row.LocationID, LocationName: row.LocationName,
		MachineType: row.MachineType, Brand: row.Brand, Model: row.Model, OperationHours: row.OperationHours,
		DeploymentType: row.DeploymentType, CapacityAmount: capacity, LowThresholdAmount: low,
		CriticalThresholdAmount: critical, Blacklisted: row.Blacklisted, EscrowAccount: row.EscrowAccount,
		PriorityClass: row.PriorityClass, IsActive: row.IsActive,
		CreatedAt: timestamptzToPtr(row.CreatedAt), UpdatedAt: timestamptzToPtr(row.UpdatedAt), DeletedAt: timestamptzToPtr(row.DeletedAt),
	}, nil
}

// decimalStrings converts the three nullable atms money columns to decimal
// strings in one call, reusing numericToDecimalStringPtr (atm_portal_profile.go).
func decimalStrings(capacity, low, critical pgtype.Numeric) (*string, *string, *string, error) {
	capacityStr, err := numericToDecimalStringPtr(capacity)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("capacity_amount: %w", err)
	}
	lowStr, err := numericToDecimalStringPtr(low)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("low_threshold_amount: %w", err)
	}
	criticalStr, err := numericToDecimalStringPtr(critical)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("critical_threshold_amount: %w", err)
	}
	return capacityStr, lowStr, criticalStr, nil
}
