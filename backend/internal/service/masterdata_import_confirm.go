package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ImportSubmitter is the Submit surface every per-entity admin service takes.
// The importer hands the services a collector instead of the real
// MasterDataChangeService, so a row is validated and its payload/"before"
// snapshot built by exactly the code the single-record endpoints use, and the
// result is then staged as ONE batch (T5.4).
type ImportSubmitter interface {
	Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
}

// ImportServices are the per-entity admin services, each built over the collector.
type ImportServices struct {
	Vendors     *VendorAdminService
	Branches    *VendorBranchAdminService
	Vaults      *VendorVaultAdminService
	PICs        *VendorPicAdminService
	ATMs        *ATMAdminService
	Assignments *ATMAssignmentAdminService
}

// ImportBatchStore stages a whole import file atomically. *repository.MasterDataImportBatchRepository satisfies it.
type ImportBatchStore interface {
	FindOpen(ctx context.Context, entity, fileHash string) (*db.FindOpenMasterDataImportBatchRow, error)
	Create(ctx context.Context, entity, fileHash string, makerID int64, rows []db.CreateMasterDataChangeRequestParams) (*db.FindOpenMasterDataImportBatchRow, bool, error)
	SetApprovalRequestID(ctx context.Context, batchID, approvalRequestID int64) error
}

// ImportConfirmDeps are the extra dependencies of Confirm.
type ImportConfirmDeps struct {
	Batches      ImportBatchStore
	Orchestrator MasterDataChangeOrchestrator
	Audit        MasterDataChangeAuditWriter
	// Services builds the per-entity services over the given submitter.
	Services func(ImportSubmitter) ImportServices
}

// WithConfirm enables Confirm on the importer.
func (i *MasterDataImporter) WithConfirm(deps ImportConfirmDeps) *MasterDataImporter {
	i.confirm = &deps
	return i
}

// ImportInvalidError is returned by Confirm when the file has row errors (the
// same list the dry-run shows, or a rejection by the entity's own validation).
// Nothing was staged. Handlers map it to 422.
type ImportInvalidError struct{ Errors []ImportRowError }

func (e *ImportInvalidError) Error() string {
	return fmt.Sprintf("import file has %d error(s)", len(e.Errors))
}

// ErrImportNothingToDo means every row already equals the current data.
var ErrImportNothingToDo = errors.New("import contains no changes")

// ImportConfirmResult describes the staged batch. Existing is true when the
// same file (same SHA-256) had already been staged: the earlier batch is returned.
type ImportConfirmResult struct {
	BatchID           int64  `json:"batch_id"`
	Rows              int    `json:"rows"`
	ApprovalRequestID *int64 `json:"approval_request_id"`
	Existing          bool   `json:"existing"`
}

type collectedChange struct {
	row int
	req SubmitRequest
}

// importCollector records what the entity services would have submitted.
type importCollector struct {
	row  int
	reqs []collectedChange
}

func (c *importCollector) Submit(_ context.Context, _ int64, req SubmitRequest, _ string) (db.MasterDataChangeRequest, error) {
	c.reqs = append(c.reqs, collectedChange{row: c.row, req: req})
	return db.MasterDataChangeRequest{}, nil
}

// Confirm stages a validated import file as one batch of change requests under a
// SINGLE approval request (plan.md T5.4). Idempotent per file: the same entity +
// SHA-256 while its earlier batch is not rejected/stale returns that batch
// instead of staging again (and finishes its approval submission if an earlier
// attempt died between staging and submitting). Nothing changes until the
// approval passes; then every row is applied in one transaction.
func (i *MasterDataImporter) Confirm(ctx context.Context, makerID int64, entity string, r io.Reader, actorIP string) (*ImportConfirmResult, error) {
	if err := authorizeMasterDataMaker(ctx, makerID); err != nil {
		return nil, err
	}
	if i.confirm == nil {
		return nil, errors.New("import confirm is not configured")
	}
	if _, ok := exportSpecs[entity]; !ok {
		return nil, ErrExportUnknownEntity
	}
	data, err := io.ReadAll(io.LimitReader(r, MasterDataImportMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MasterDataImportMaxBytes {
		return nil, &ValidationError{Field: "file", Message: fmt.Sprintf("ukuran file melebihi %d MB", MasterDataImportMaxBytes>>20)}
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	// Idempotency first: once a batch has been applied the file no longer
	// validates against the new data ("already exists"), but it must still answer "done".
	if existing, err := i.confirm.Batches.FindOpen(ctx, entity, hash); err != nil {
		return nil, fmt.Errorf("find import batch: %w", err)
	} else if existing != nil {
		return i.finish(ctx, makerID, entity, hash, existing, true, actorIP)
	}

	res, err := i.DryRun(ctx, entity, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(res.Preview.Errors) > 0 {
		return nil, &ImportInvalidError{Errors: res.Preview.Errors}
	}

	collector := &importCollector{}
	svcs := i.confirm.Services(collector)
	for _, row := range res.Rows {
		if row.Op == ImportOpUnchanged {
			continue
		}
		collector.row = row.Row
		if err := stageRow(ctx, makerID, entity, row, res.env, svcs, actorIP); err != nil {
			return nil, rowRejection(row.Row, err)
		}
	}
	if len(collector.reqs) == 0 {
		return nil, ErrImportNothingToDo
	}

	params := make([]db.CreateMasterDataChangeRequestParams, len(collector.reqs))
	for n, c := range collector.reqs {
		payload, err := marshalOrNilJSON(c.req.Payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload (row %d): %w", c.row, err)
		}
		before, err := marshalOrNilJSON(c.req.Before)
		if err != nil {
			return nil, fmt.Errorf("marshal before snapshot (row %d): %w", c.row, err)
		}
		params[n] = db.CreateMasterDataChangeRequestParams{EntityType: c.req.EntityType, EntityID: c.req.EntityID, Op: c.req.Op,
			Payload: payload, Before: before, MakerID: makerID}
	}

	batch, existed, err := i.confirm.Batches.Create(ctx, entity, hash, makerID, params)
	if err != nil {
		var rowErr interface{ BatchRowIndex() int }
		if errors.As(err, &rowErr) && rowErr.BatchRowIndex() < len(collector.reqs) {
			return nil, fmt.Errorf("baris %d: %w", collector.reqs[rowErr.BatchRowIndex()].row, ErrMasterDataChangePending)
		}
		return nil, fmt.Errorf("stage import batch: %w", err)
	}
	return i.finish(ctx, makerID, entity, hash, batch, existed, actorIP)
}

// finish submits the batch's single approval request when it has none yet.
func (i *MasterDataImporter) finish(ctx context.Context, makerID int64, entity, hash string, batch *db.FindOpenMasterDataImportBatchRow, existing bool, actorIP string) (*ImportConfirmResult, error) {
	out := &ImportConfirmResult{BatchID: batch.ID, Rows: int(batch.RowCount), ApprovalRequestID: batch.ApprovalRequestID, Existing: existing}
	if batch.ApprovalRequestID != nil {
		return out, nil
	}
	var amount pgtype.Numeric
	if err := amount.Scan("0"); err != nil {
		return nil, fmt.Errorf("build zero amount: %w", err)
	}
	approval, _, err := i.confirm.Orchestrator.SubmitForApproval(ctx, makerID, masterDataDocumentType, batch.HeadID, amount, actorIP)
	if err != nil {
		return nil, fmt.Errorf("submit import batch %d for approval: %w", batch.ID, err)
	}
	if err := i.confirm.Batches.SetApprovalRequestID(ctx, batch.ID, approval.ID); err != nil {
		return nil, fmt.Errorf("link approval request: %w", err)
	}
	out.ApprovalRequestID = &approval.ID
	if err := i.confirm.Audit.Write(ctx, audit.Entry{
		ActorID: makerID, Action: "submit_batch", EntityType: "master_data_import_batch", EntityID: batch.ID,
		After: map[string]any{"entity": entity, "file_hash": hash, "rows": batch.RowCount}, IP: actorIP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}
	return out, nil
}

// rowRejection turns an entity service's refusal of a row into a 422 row error.
func rowRejection(row int, err error) error {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return &ImportInvalidError{Errors: []ImportRowError{{Row: row, Field: ve.Field, Message: ve.Message}}}
	}
	for _, known := range []error{ErrVendorCodeConflict, ErrATMTerminalIDConflict, ErrATMAssignmentOverlap, ErrATMAssignmentDuplicate,
		ErrVendorVaultCodeConflict, ErrATMNotFound, ErrATMInvalidReference} {
		if errors.Is(err, known) {
			return &ImportInvalidError{Errors: []ImportRowError{{Row: row, Field: "row", Message: err.Error()}}}
		}
	}
	return fmt.Errorf("row %d: %w", row, err)
}

func optInt64(s string) *int64 {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// stageRow runs one validated row through the matching entity service (which
// re-validates and records its change on the collector).
func stageRow(ctx context.Context, makerID int64, entity string, row ImportRow, env *importEnv, s ImportServices, ip string) error {
	switch {
	case row.Op == ImportOpCreate:
		return stageCreate(ctx, makerID, entity, row.Cells, env, s, ip)
	case row.Toggle:
		return stageToggle(ctx, makerID, entity, row, env, s, ip)
	}
	return stageUpdate(ctx, makerID, entity, row, env, s, ip)
}

func atmFields(v map[string]string) (loc int64, req UpdateATMRequest) {
	loc, _ = strconv.ParseInt(v["location_id"], 10, 64)
	return loc, UpdateATMRequest{LocationID: loc, MachineType: v["machine_type"], Brand: v["brand"], Model: v["model"],
		OperationHours: v["operation_hours"], DeploymentType: v["deployment_type"], CapacityAmount: optStr(v["capacity_amount"]),
		LowThresholdAmount: optStr(v["low_threshold_amount"]), CriticalThresholdAmount: optStr(v["critical_threshold_amount"]),
		Blacklisted: v["blacklisted"] == "true", EscrowAccount: optStr(v["escrow_account"]), PriorityClass: optStr(v["priority_class"])}
}

func stageCreate(ctx context.Context, maker int64, entity string, v map[string]string, env *importEnv, s ImportServices, ip string) error {
	var err error
	switch entity {
	case ExportVendors:
		_, err = s.Vendors.Create(ctx, maker, CreateVendorRequest{Code: v["code"], Name: v["name"], LegalName: v["legal_name"], NPWP: v["npwp"],
			ContactEmail: v["contact_email"], ContactPhone: v["contact_phone"], HqAddress: v["hq_address"]}, ip)
	case ExportVendorBranches:
		_, err = s.Branches.Create(ctx, maker, VendorBranchPayload{VendorID: env.vendors[v["vendor_code"]], BranchCode: v["branch_code"],
			BranchName: v["branch_name"], LocationID: optInt64(v["location_id"]), Region: optStr(v["region"])}, ip)
	case ExportVendorVaults:
		_, err = s.Vaults.Create(ctx, maker, env.vendors[v["vendor_code"]], VendorVaultPayload{
			VendorBranchID: env.branches[v["vendor_code"]+"|"+v["branch_code"]], VaultCode: v["vault_code"],
			VendorVaultUpdatePayload: vaultFields(v)}, ip)
	case ExportVendorPICs:
		_, err = s.PICs.Create(ctx, maker, env.vendors[v["vendor_code"]], picFields(v, env), ip)
	case ExportATMs:
		loc, u := atmFields(v)
		_, err = s.ATMs.Create(ctx, maker, CreateATMRequest{TerminalID: v["terminal_id"], LocationID: loc, MachineType: u.MachineType, Brand: u.Brand,
			Model: u.Model, OperationHours: u.OperationHours, DeploymentType: u.DeploymentType, CapacityAmount: u.CapacityAmount,
			LowThresholdAmount: u.LowThresholdAmount, CriticalThresholdAmount: u.CriticalThresholdAmount,
			Blacklisted: u.Blacklisted, EscrowAccount: u.EscrowAccount, PriorityClass: u.PriorityClass}, ip)
	case ExportATMAssignments:
		_, err = s.Assignments.Create(ctx, maker, env.atms[v["terminal_id"]], assignmentFields(v, env), ip)
	}
	return err
}

func stageUpdate(ctx context.Context, maker int64, entity string, row ImportRow, env *importEnv, s ImportServices, ip string) error {
	v, id := row.Cells, row.ID
	var err error
	switch entity {
	case ExportVendors:
		legal, npwp := v["legal_name"], v["npwp"]
		_, err = s.Vendors.Update(ctx, maker, id, UpdateVendorRequest{Name: v["name"], LegalName: &legal, NPWP: &npwp,
			ContactEmail: v["contact_email"], ContactPhone: v["contact_phone"], HqAddress: v["hq_address"]}, ip)
	case ExportVendorBranches:
		_, err = s.Branches.Update(ctx, maker, id, VendorBranchUpdatePayload{BranchName: v["branch_name"], LocationID: optInt64(v["location_id"]), Region: optStr(v["region"])}, ip)
	case ExportVendorVaults:
		_, err = s.Vaults.Update(ctx, maker, id, vaultFields(v), ip)
	case ExportVendorPICs:
		_, err = s.PICs.Update(ctx, maker, env.vendors[v["vendor_code"]], id, picFields(v, env), ip)
	case ExportATMs:
		_, u := atmFields(v)
		_, err = s.ATMs.Update(ctx, maker, id, u, ip)
	case ExportATMAssignments:
		_, err = s.Assignments.Update(ctx, maker, env.atms[v["terminal_id"]], id, assignmentFields(v, env), ip)
	}
	return err
}

func stageToggle(ctx context.Context, maker int64, entity string, row ImportRow, env *importEnv, s ImportServices, ip string) error {
	v, id := row.Cells, row.ID
	enable := v["is_active"] == "true"
	var err error
	switch entity {
	case ExportVendors:
		if enable {
			_, err = s.Vendors.Enable(ctx, maker, id, ip)
		} else {
			_, err = s.Vendors.Disable(ctx, maker, id, ip)
		}
	case ExportVendorBranches:
		if enable {
			_, err = s.Branches.Enable(ctx, maker, id, ip)
		} else {
			_, err = s.Branches.Disable(ctx, maker, id, ip)
		}
	case ExportVendorVaults:
		if enable {
			_, err = s.Vaults.Enable(ctx, maker, id, ip)
		} else {
			_, err = s.Vaults.Disable(ctx, maker, id, ip)
		}
	case ExportVendorPICs:
		vendorID := env.vendors[v["vendor_code"]]
		if enable {
			_, err = s.PICs.Enable(ctx, maker, vendorID, id, ip)
		} else {
			_, err = s.PICs.Disable(ctx, maker, vendorID, id, ip)
		}
	case ExportATMs:
		if enable {
			_, err = s.ATMs.Enable(ctx, maker, id, ip)
		} else {
			_, err = s.ATMs.Disable(ctx, maker, id, ip)
		}
	case ExportATMAssignments:
		atmID := env.atms[v["terminal_id"]]
		if enable {
			_, err = s.Assignments.Enable(ctx, maker, atmID, id, ip)
		} else {
			_, err = s.Assignments.Disable(ctx, maker, atmID, id, ip)
		}
	}
	return err
}

func vaultFields(v map[string]string) VendorVaultUpdatePayload {
	return VendorVaultUpdatePayload{Category: v["category"], CurrencyCode: v["currency_code"],
		MinCapacityAmount: optStr(v["min_capacity_amount"]), MaxCapacityAmount: optStr(v["max_capacity_amount"]),
		Latitude: optStr(v["latitude"]), Longitude: optStr(v["longitude"]),
		OperatingHours: optStr(v["operating_hours"]), LocationID: optInt64(v["location_id"])}
}

func picFields(v map[string]string, env *importEnv) VendorPicUpdatePayload {
	var branch *int64
	if v["branch_code"] != "" {
		id := env.branches[v["vendor_code"]+"|"+v["branch_code"]]
		branch = &id
	}
	return VendorPicUpdatePayload{VendorBranchID: branch, Name: v["name"], Position: optStr(v["position"]),
		Phone: optStr(v["phone"]), Email: optStr(v["email"]), IsNotificationRecipient: v["is_notification_recipient"] == "true"}
}

func assignmentFields(v map[string]string, env *importEnv) ATMAssignmentUpdatePayload {
	return ATMAssignmentUpdatePayload{VendorPackageID: env.packages[v["vendor_code"]+"|"+v["branch_code"]+"|"+v["package_code"]],
		EffectiveStartDate: v["effective_start_date"], EffectiveEndDate: optStr(v["effective_end_date"])}
}
