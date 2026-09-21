package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// --- fakes ------------------------------------------------------------

type fakeMasterDataChangeRepo struct {
	findPendingFunc         func(ctx context.Context, entityType string, entityID int64) (*int64, error)
	createFunc              func(ctx context.Context, arg db.CreateMasterDataChangeRequestParams) (db.MasterDataChangeRequest, error)
	setApprovalRequestIDErr error

	findPendingCalled         bool
	createCalled              bool
	setApprovalRequestIDCalls []struct{ id, approvalRequestID int64 }
}

func (f *fakeMasterDataChangeRepo) FindPending(ctx context.Context, entityType string, entityID int64) (*int64, error) {
	f.findPendingCalled = true
	if f.findPendingFunc != nil {
		return f.findPendingFunc(ctx, entityType, entityID)
	}
	return nil, nil
}

func (f *fakeMasterDataChangeRepo) Create(ctx context.Context, arg db.CreateMasterDataChangeRequestParams) (db.MasterDataChangeRequest, error) {
	f.createCalled = true
	if f.createFunc != nil {
		return f.createFunc(ctx, arg)
	}
	return db.MasterDataChangeRequest{
		ID:         42,
		EntityType: arg.EntityType,
		EntityID:   arg.EntityID,
		Op:         arg.Op,
		Payload:    arg.Payload,
		Before:     arg.Before,
		Status:     "pending",
		MakerID:    arg.MakerID,
		BatchID:    arg.BatchID,
	}, nil
}

func (f *fakeMasterDataChangeRepo) GetByID(ctx context.Context, id int64) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: id}, nil
}

func (f *fakeMasterDataChangeRepo) List(ctx context.Context, arg db.ListMasterDataChangeRequestsParams) ([]db.MasterDataChangeRequest, error) {
	return nil, nil
}

func (f *fakeMasterDataChangeRepo) Count(ctx context.Context, arg db.CountMasterDataChangeRequestsParams) (int64, error) {
	return 0, nil
}

func (f *fakeMasterDataChangeRepo) SetApprovalRequestID(ctx context.Context, id, approvalRequestID int64) error {
	f.setApprovalRequestIDCalls = append(f.setApprovalRequestIDCalls, struct{ id, approvalRequestID int64 }{id, approvalRequestID})
	return f.setApprovalRequestIDErr
}

type fakeMasterDataOrchestrator struct {
	submitFunc func(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error)
	called     bool
	lastAmount pgtype.Numeric
}

func (f *fakeMasterDataOrchestrator) SubmitForApproval(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error) {
	f.called = true
	f.lastAmount = amount
	if f.submitFunc != nil {
		return f.submitFunc(ctx, makerID, documentType, documentID, amount, actorIP)
	}
	return db.ApprovalRequest{ID: 99, MakerID: makerID, DocumentType: documentType, DocumentID: documentID, Status: "pending"}, true, nil
}

type fakeMasterDataAuditWriter struct {
	writeFunc func(ctx context.Context, entry audit.Entry) error
	entries   []audit.Entry
}

func (f *fakeMasterDataAuditWriter) Write(ctx context.Context, entry audit.Entry) error {
	f.entries = append(f.entries, entry)
	if f.writeFunc != nil {
		return f.writeFunc(ctx, entry)
	}
	return nil
}

// --- tests --------------------------------------------------------------

func TestMasterDataChangeService_Submit_Create(t *testing.T) {
	repo := &fakeMasterDataChangeRepo{}
	orch := &fakeMasterDataOrchestrator{}
	auditWriter := &fakeMasterDataAuditWriter{}
	svc := NewMasterDataChangeService(repo, orch, auditWriter)

	change, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "create",
		Payload:    map[string]string{"code": "VEN1"},
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if !repo.createCalled {
		t.Error("expected repo.Create to be called")
	}
	if !orch.called {
		t.Error("expected orchestrator.SubmitForApproval to be called")
	}
	if orch.lastAmount.Int == nil || orch.lastAmount.Int.Sign() != 0 {
		t.Errorf("expected amount=0, got %+v", orch.lastAmount)
	}
	if len(repo.setApprovalRequestIDCalls) != 1 || repo.setApprovalRequestIDCalls[0].id != 42 || repo.setApprovalRequestIDCalls[0].approvalRequestID != 99 {
		t.Errorf("expected SetApprovalRequestID(42, 99), got %+v", repo.setApprovalRequestIDCalls)
	}
	if change.ApprovalRequestID == nil || *change.ApprovalRequestID != 99 {
		t.Errorf("expected returned change.ApprovalRequestID=99, got %+v", change.ApprovalRequestID)
	}
	if len(auditWriter.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditWriter.entries))
	}
	entry := auditWriter.entries[0]
	if entry.Action != "submit" || entry.EntityType != "master_data_change_request" || entry.EntityID != 42 || entry.ActorID != 7 {
		t.Errorf("unexpected audit entry: %+v", entry)
	}
}

func TestMasterDataChangeService_Submit_Update_RequiresEntityID(t *testing.T) {
	svc := NewMasterDataChangeService(&fakeMasterDataChangeRepo{}, &fakeMasterDataOrchestrator{}, &fakeMasterDataAuditWriter{})

	_, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "update",
		Payload:    map[string]string{"name": "new"},
	}, "127.0.0.1")

	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "entity_id" {
		t.Fatalf("expected ValidationError on entity_id, got %v", err)
	}
}

func TestMasterDataChangeService_Submit_Create_RejectsEntityID(t *testing.T) {
	svc := NewMasterDataChangeService(&fakeMasterDataChangeRepo{}, &fakeMasterDataOrchestrator{}, &fakeMasterDataAuditWriter{})
	entityID := int64(5)

	_, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "create",
		EntityID:   &entityID,
		Payload:    map[string]string{"code": "VEN1"},
	}, "127.0.0.1")

	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "entity_id" {
		t.Fatalf("expected ValidationError on entity_id, got %v", err)
	}
}

func TestMasterDataChangeService_Submit_Update_RejectsWhenAlreadyPending(t *testing.T) {
	existingID := int64(11)
	repo := &fakeMasterDataChangeRepo{
		findPendingFunc: func(ctx context.Context, entityType string, entityID int64) (*int64, error) {
			return &existingID, nil
		},
	}
	auditWriter := &fakeMasterDataAuditWriter{}
	svc := NewMasterDataChangeService(repo, &fakeMasterDataOrchestrator{}, auditWriter)
	entityID := int64(5)

	_, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "update",
		EntityID:   &entityID,
		Payload:    map[string]string{"name": "new"},
	}, "127.0.0.1")

	if !errors.Is(err, ErrMasterDataChangePending) {
		t.Fatalf("expected ErrMasterDataChangePending, got %v", err)
	}
	if repo.createCalled {
		t.Error("expected repo.Create not to be called when a pending change already exists")
	}
	if len(auditWriter.entries) != 0 {
		t.Errorf("expected no audit entry written, got %d", len(auditWriter.entries))
	}
}

func TestMasterDataChangeService_Submit_Create_SkipsPendingCheck(t *testing.T) {
	// op=create has no entity_id yet -- nothing to collide on, and the check
	// must not be applied per-entity_type (that would wrongly block
	// unrelated concurrent creates of the same entity_type).
	repo := &fakeMasterDataChangeRepo{}
	svc := NewMasterDataChangeService(repo, &fakeMasterDataOrchestrator{}, &fakeMasterDataAuditWriter{})

	_, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "create",
		Payload:    map[string]string{"code": "VEN1"},
	}, "127.0.0.1")

	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if repo.findPendingCalled {
		t.Error("expected FindPending not to be called for op=create")
	}
}

func TestMasterDataChangeService_Submit_UniqueViolationOnCreate_MapsToErrPending(t *testing.T) {
	// Simulates the race the pre-check misses: FindPending sees nothing, but
	// the DB's partial unique index rejects the INSERT anyway.
	repo := &fakeMasterDataChangeRepo{
		createFunc: func(ctx context.Context, arg db.CreateMasterDataChangeRequestParams) (db.MasterDataChangeRequest, error) {
			return db.MasterDataChangeRequest{}, &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	svc := NewMasterDataChangeService(repo, &fakeMasterDataOrchestrator{}, &fakeMasterDataAuditWriter{})
	entityID := int64(5)

	_, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "update",
		EntityID:   &entityID,
		Payload:    map[string]string{"name": "new"},
	}, "127.0.0.1")

	if !errors.Is(err, ErrMasterDataChangePending) {
		t.Fatalf("expected ErrMasterDataChangePending, got %v", err)
	}
}

func TestMasterDataChangeService_Submit_InvalidOp(t *testing.T) {
	svc := NewMasterDataChangeService(&fakeMasterDataChangeRepo{}, &fakeMasterDataOrchestrator{}, &fakeMasterDataAuditWriter{})

	_, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "delete",
	}, "127.0.0.1")

	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "op" {
		t.Fatalf("expected ValidationError on op, got %v", err)
	}
}

func TestMasterDataChangeService_Submit_OrchestratorError_NoAuditWritten(t *testing.T) {
	repo := &fakeMasterDataChangeRepo{}
	orch := &fakeMasterDataOrchestrator{
		submitFunc: func(ctx context.Context, makerID int64, documentType string, documentID int64, amount pgtype.Numeric, actorIP string) (db.ApprovalRequest, bool, error) {
			return db.ApprovalRequest{}, false, errors.New("boom")
		},
	}
	auditWriter := &fakeMasterDataAuditWriter{}
	svc := NewMasterDataChangeService(repo, orch, auditWriter)

	_, err := svc.Submit(adminCtx(7), 7, SubmitRequest{
		EntityType: "vendor",
		Op:         "create",
		Payload:    map[string]string{"code": "VEN1"},
	}, "127.0.0.1")

	if err == nil {
		t.Fatal("expected error from orchestrator failure")
	}
	if len(auditWriter.entries) != 0 {
		t.Errorf("expected no audit entry written on orchestrator failure, got %d", len(auditWriter.entries))
	}
}
