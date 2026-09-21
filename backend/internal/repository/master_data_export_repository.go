package repository

import (
	"context"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// MasterDataExportRepository backs the master-data CSV export (plan.md T5.1).
// Every method is one keyset page (rows with id > afterID, ordered by id, at
// most limit of them) so the exporter streams a table of any size with
// bounded memory. Read-only.
//
// This is the repository main.go constructs on the READ-REPLICA pool
// (DATABASE_REPLICA_URL; the primary when unset) -- exports are heavy reads
// and must not load the primary (CLAUDE.md Sec 6). Consequence: a row written
// moments ago may not be in the file yet (replica lag); an export is a
// point-in-time snapshot, not read-your-writes.
type MasterDataExportRepository struct {
	queries *db.Queries
}

// NewMasterDataExportRepository creates a MasterDataExportRepository on the given (read) connection.
func NewMasterDataExportRepository(dbConn db.DBTX) *MasterDataExportRepository {
	return &MasterDataExportRepository{queries: db.New(dbConn)}
}

func (r *MasterDataExportRepository) Vendors(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorsBatchRow, error) {
	return r.queries.ExportVendorsBatch(ctx, db.ExportVendorsBatchParams{AfterID: afterID, Status: status, BatchSize: limit})
}

func (r *MasterDataExportRepository) VendorBranches(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorBranchesBatchRow, error) {
	return r.queries.ExportVendorBranchesBatch(ctx, db.ExportVendorBranchesBatchParams{AfterID: afterID, Status: status, BatchSize: limit})
}

func (r *MasterDataExportRepository) VendorVaults(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorVaultsBatchRow, error) {
	return r.queries.ExportVendorVaultsBatch(ctx, db.ExportVendorVaultsBatchParams{AfterID: afterID, Status: status, BatchSize: limit})
}

func (r *MasterDataExportRepository) VendorPICs(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorPicsBatchRow, error) {
	return r.queries.ExportVendorPicsBatch(ctx, db.ExportVendorPicsBatchParams{AfterID: afterID, Status: status, BatchSize: limit})
}

func (r *MasterDataExportRepository) ATMs(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportATMsBatchRow, error) {
	return r.queries.ExportATMsBatch(ctx, db.ExportATMsBatchParams{AfterID: afterID, Status: status, BatchSize: limit})
}

func (r *MasterDataExportRepository) ATMAssignments(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportATMAssignmentsBatchRow, error) {
	return r.queries.ExportATMAssignmentsBatch(ctx, db.ExportATMAssignmentsBatchParams{AfterID: afterID, Status: status, BatchSize: limit})
}

// PackageKeys and LocationIDs are the reference sets the import dry-run
// (T5.3) validates foreign keys against. Construct the repository for the
// importer on the PRIMARY pool: validating a write flow must not be fooled by
// replica lag.
func (r *MasterDataExportRepository) PackageKeys(ctx context.Context) ([]db.ImportPackageKeysRow, error) {
	return r.queries.ImportPackageKeys(ctx)
}

func (r *MasterDataExportRepository) LocationIDs(ctx context.Context) ([]int64, error) {
	return r.queries.ImportLocationIDs(ctx)
}
