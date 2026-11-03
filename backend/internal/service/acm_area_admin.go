// AcmAreaAdminService backs the admin Area ACM endpoints (cit-acm-plan FR7).
// Same documented deviation from Golden Rule #3 as RegionAdminService: no
// maker-checker, every mutation applies immediately with an audit_logs row in
// the same transaction (a failed audit write rolls the change back), and the
// ADMIN role is re-checked here independent of the route guard.
package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Sentinel errors for AcmAreaAdminService.
var (
	ErrAcmAreaNotFound        = errors.New("area ACM tidak ditemukan")
	ErrAcmAreaNameConflict    = errors.New("nama area ACM sudah dipakai")
	ErrAcmAreaInactive        = errors.New("area ACM nonaktif")
	ErrAcmAreaStatusUnchanged = errors.New("status area ACM tidak berubah")
)

// AcmAreaBranchConflictError: some requested branches already belong to another area (FR7.2) -> 409.
type AcmAreaBranchConflictError struct {
	Conflicts []db.ListBranchesInOtherAcmAreasRow
}

func (e *AcmAreaBranchConflictError) Error() string {
	return fmt.Sprintf("%d cabang sudah terdaftar di area ACM lain", len(e.Conflicts))
}

// AcmAreaRegionConflictError: some requested regions already belong to another area (cit-vault-select FR1.1) -> 409.
type AcmAreaRegionConflictError struct {
	Conflicts []db.ListRegionsInOtherAcmAreasRow
}

func (e *AcmAreaRegionConflictError) Error() string {
	return fmt.Sprintf("%d region sudah terdaftar di area ACM lain", len(e.Conflicts))
}

// AcmAreaPool is the primary pool: mutation transactions + read-after-write
// (*pgxpool.Pool satisfies it).
type AcmAreaPool interface {
	db.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// AcmAreaDetail is an area with its region, branch and member links.
type AcmAreaDetail struct {
	Area     db.AcmArea
	Regions  []string
	Branches []db.ListAcmAreaBranchesRow
	Members  []db.ListAcmAreaMembersRow
}

// AcmAreaAdminService: reads on readDB (replica), mutations in a tx on pool (primary).
type AcmAreaAdminService struct {
	pool     AcmAreaPool
	read     *db.Queries
	notifier Notifier // nil = no notifications
}

// WithNotifier enables the FR6.3 notification sent when a branch assignment
// creates vault plans (FR7.3).
func (s *AcmAreaAdminService) WithNotifier(n Notifier) *AcmAreaAdminService {
	s.notifier = n
	return s
}

// NewAcmAreaAdminService creates an AcmAreaAdminService.
func NewAcmAreaAdminService(pool AcmAreaPool, readDB db.DBTX) *AcmAreaAdminService {
	return &AcmAreaAdminService{pool: pool, read: db.New(readDB)}
}

func isAcmAreaAdmin(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "ADMIN")
}

// List returns areas filtered by status (replica). Read-only.
func (s *AcmAreaAdminService) List(ctx context.Context, status string) ([]db.ListAcmAreasAdminRow, error) {
	return s.read.ListAcmAreasAdmin(ctx, status)
}

// Get returns an area with branches and members (replica).
func (s *AcmAreaAdminService) Get(ctx context.Context, id int64) (*AcmAreaDetail, error) {
	return loadAcmAreaDetail(ctx, s.read, id)
}

// EligibleUsers lists active ACM-USER/ACM-SPV users for the member picker.
func (s *AcmAreaAdminService) EligibleUsers(ctx context.Context) ([]db.ListAcmEligibleUsersRow, error) {
	return s.read.ListAcmEligibleUsers(ctx)
}

// BranchOptions lists active vendor branches with their current area (picker source).
func (s *AcmAreaAdminService) BranchOptions(ctx context.Context) ([]db.ListAcmBranchOptionsRow, error) {
	return s.read.ListAcmBranchOptions(ctx)
}

// RegionOptions lists region codes of active vendor branches with their current area (picker source, FR1.1).
func (s *AcmAreaAdminService) RegionOptions(ctx context.Context) ([]db.ListAcmRegionOptionsRow, error) {
	return s.read.ListAcmRegionOptions(ctx)
}

// Warnings lists requests stuck in vault_assignment without a plan: region
// without an active area, or no region code (cit-vault-select FR1.4).
func (s *AcmAreaAdminService) Warnings(ctx context.Context) ([]db.ListVaultPlanRegionWarningsRow, error) {
	return s.read.ListVaultPlanRegionWarnings(ctx)
}

func loadAcmAreaDetail(ctx context.Context, q *db.Queries, id int64) (*AcmAreaDetail, error) {
	area, err := getAcmArea(ctx, q, id)
	if err != nil {
		return nil, err
	}
	regions, err := q.ListAcmAreaRegions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading acm area regions: %w", err)
	}
	branches, err := q.ListAcmAreaBranches(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading acm area branches: %w", err)
	}
	members, err := q.ListAcmAreaMembers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading acm area members: %w", err)
	}
	return &AcmAreaDetail{Area: area, Regions: regions, Branches: branches, Members: members}, nil
}

func validateAcmAreaName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || len([]rune(n)) > 100 {
		return "", &ValidationError{Field: "name", Message: "nama wajib diisi, maksimal 100 karakter"}
	}
	return n, nil
}

// inTx re-checks the role, runs fn in a transaction and writes its audit entry before commit.
func (s *AcmAreaAdminService) inTx(ctx context.Context, actorRole string, fn func(q *db.Queries, aw *audit.Writer) (audit.Entry, error)) error {
	if !isAcmAreaAdmin(actorRole) {
		return ErrNotAuthorized
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	aw := audit.NewWriter(tx)
	entry, err := fn(db.New(tx), aw)
	if err != nil {
		return err
	}
	if err := aw.Write(ctx, entry); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return tx.Commit(ctx)
}

// Create inserts a new active area.
func (s *AcmAreaAdminService) Create(ctx context.Context, actorID int64, actorRole, name, ip string) (db.AcmArea, error) {
	var out db.AcmArea
	err := s.inTx(ctx, actorRole, func(q *db.Queries, _ *audit.Writer) (audit.Entry, error) {
		n, err := validateAcmAreaName(name)
		if err != nil {
			return audit.Entry{}, err
		}
		if out, err = q.CreateAcmArea(ctx, db.CreateAcmAreaParams{Name: n, CreatedBy: actorID}); err != nil {
			return audit.Entry{}, mapAcmAreaNameErr(err)
		}
		return audit.Entry{ActorID: actorID, Action: "acm_area_created", EntityType: "acm_area", EntityID: out.ID, After: out, IP: ip}, nil
	})
	return out, err
}

// Rename changes an area's name.
func (s *AcmAreaAdminService) Rename(ctx context.Context, actorID int64, actorRole string, id int64, name, ip string) (db.AcmArea, error) {
	var out db.AcmArea
	err := s.inTx(ctx, actorRole, func(q *db.Queries, _ *audit.Writer) (audit.Entry, error) {
		n, err := validateAcmAreaName(name)
		if err != nil {
			return audit.Entry{}, err
		}
		before, err := getAcmArea(ctx, q, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if out, err = q.RenameAcmArea(ctx, db.RenameAcmAreaParams{ID: id, Name: n}); err != nil {
			return audit.Entry{}, mapAcmAreaNameErr(err)
		}
		return audit.Entry{ActorID: actorID, Action: "acm_area_renamed", EntityType: "acm_area", EntityID: id,
			Before: map[string]any{"name": before.Name}, After: map[string]any{"name": out.Name}, IP: ip}, nil
	})
	return out, err
}

// SetActive enables/disables an area. Disabling releases its regions and
// branches (they can then join another area); members stay. Existing vault
// plans are untouched (FR1.3).
func (s *AcmAreaAdminService) SetActive(ctx context.Context, actorID int64, actorRole string, id int64, active bool, ip string) (db.AcmArea, error) {
	var out db.AcmArea
	err := s.inTx(ctx, actorRole, func(q *db.Queries, _ *audit.Writer) (audit.Entry, error) {
		before, err := getAcmArea(ctx, q, id)
		if err != nil {
			return audit.Entry{}, err
		}
		if before.IsActive == active {
			return audit.Entry{}, ErrAcmAreaStatusUnchanged
		}
		released := []int64{}
		releasedRegions := []string{}
		if !active {
			branches, err := q.ListAcmAreaBranches(ctx, id)
			if err != nil {
				return audit.Entry{}, fmt.Errorf("loading acm area branches: %w", err)
			}
			for _, b := range branches {
				released = append(released, b.VendorBranchID)
			}
			if err := q.ClearAcmAreaBranches(ctx, id); err != nil {
				return audit.Entry{}, fmt.Errorf("releasing acm area branches: %w", err)
			}
			if releasedRegions, err = q.ListAcmAreaRegions(ctx, id); err != nil {
				return audit.Entry{}, fmt.Errorf("loading acm area regions: %w", err)
			}
			if err := q.ClearAcmAreaRegions(ctx, id); err != nil {
				return audit.Entry{}, fmt.Errorf("releasing acm area regions: %w", err)
			}
		}
		if out, err = q.SetAcmAreaActive(ctx, db.SetAcmAreaActiveParams{ID: id, IsActive: active}); err != nil {
			return audit.Entry{}, mapAcmAreaNameErr(err)
		}
		action := "acm_area_reactivated"
		if !active {
			action = "acm_area_deactivated"
		}
		return audit.Entry{ActorID: actorID, Action: action, EntityType: "acm_area", EntityID: id,
			Before: map[string]any{"is_active": before.IsActive, "released_vendor_branch_ids": released, "released_region_codes": releasedRegions},
			After:  map[string]any{"is_active": out.IsActive}, IP: ip}, nil
	})
	return out, err
}

// SetRegions replaces the area's region list (cit-vault-select FR1.1/FR1.2)
// and, FR1.3, creates the missing vault plans for requests already waiting in
// vault_assignment. Returns the updated detail read from the primary.
func (s *AcmAreaAdminService) SetRegions(ctx context.Context, actorID int64, actorRole string, id int64, regionCodes []string, ip string) (*AcmAreaDetail, error) {
	codes, err := uniqueRegionCodes(regionCodes)
	if err != nil {
		return nil, err
	}
	return s.thenLoad(ctx, id, s.inTx(ctx, actorRole, func(q *db.Queries, aw *audit.Writer) (audit.Entry, error) {
		if err := requireActiveAcmArea(ctx, q, id); err != nil {
			return audit.Entry{}, err
		}
		conflicts, err := q.ListRegionsInOtherAcmAreas(ctx, db.ListRegionsInOtherAcmAreasParams{RegionCodes: codes, AcmAreaID: id})
		if err != nil {
			return audit.Entry{}, fmt.Errorf("checking region conflicts: %w", err)
		}
		if len(conflicts) > 0 {
			return audit.Entry{}, &AcmAreaRegionConflictError{Conflicts: conflicts}
		}
		before, err := q.ListAcmAreaRegions(ctx, id)
		if err != nil {
			return audit.Entry{}, fmt.Errorf("loading acm area regions: %w", err)
		}
		if err := q.ClearAcmAreaRegions(ctx, id); err != nil {
			return audit.Entry{}, fmt.Errorf("clearing acm area regions: %w", err)
		}
		for _, c := range codes {
			if err := q.AddAcmAreaRegion(ctx, db.AddAcmAreaRegionParams{AcmAreaID: id, RegionCode: c, CreatedBy: actorID}); err != nil {
				if isUniqueViolation(err) { // raced with another area's mapping
					return audit.Entry{}, &AcmAreaRegionConflictError{}
				}
				return audit.Entry{}, fmt.Errorf("adding acm area region: %w", err)
			}
		}
		if err := createVaultPlans(ctx, q, aw, s.notifier, Actor{UserID: actorID, Role: actorRole, IP: ip}, nil); err != nil {
			return audit.Entry{}, err
		}
		return audit.Entry{ActorID: actorID, Action: "acm_area_regions_set", EntityType: "acm_area", EntityID: id,
			Before: map[string]any{"region_codes": before}, After: map[string]any{"region_codes": codes}, IP: ip}, nil
	}))
}

// uniqueRegionCodes trims, upper-cases, de-duplicates and sorts the codes and
// checks the vendor_branches.region_code format (FR1.1); never nil.
func uniqueRegionCodes(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if !branchRegionCodeRe.MatchString(c) {
			return nil, &ValidationError{Field: "region_codes", Message: fmt.Sprintf("kode region %q harus 2-10 huruf/angka", c)}
		}
		out = append(out, c)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// SetBranches replaces the area's branch list (cit-acm-plan FR7.1/7.2). Since
// cit-vault-select, plans follow the request's region (FR1.5): branch links no
// longer create plans. Returns the updated detail read from the primary.
func (s *AcmAreaAdminService) SetBranches(ctx context.Context, actorID int64, actorRole string, id int64, branchIDs []int64, ip string) (*AcmAreaDetail, error) {
	ids := uniqueIDs(branchIDs)
	return s.thenLoad(ctx, id, s.inTx(ctx, actorRole, func(q *db.Queries, _ *audit.Writer) (audit.Entry, error) {
		if err := requireActiveAcmArea(ctx, q, id); err != nil {
			return audit.Entry{}, err
		}
		n, err := q.CountActiveVendorBranches(ctx, ids)
		if err != nil {
			return audit.Entry{}, fmt.Errorf("checking branches: %w", err)
		}
		if int(n) != len(ids) {
			return audit.Entry{}, &ValidationError{Field: "vendor_branch_ids", Message: "ada cabang yang tidak ditemukan atau nonaktif"}
		}
		conflicts, err := q.ListBranchesInOtherAcmAreas(ctx, db.ListBranchesInOtherAcmAreasParams{BranchIds: ids, AcmAreaID: id})
		if err != nil {
			return audit.Entry{}, fmt.Errorf("checking branch conflicts: %w", err)
		}
		if len(conflicts) > 0 {
			return audit.Entry{}, &AcmAreaBranchConflictError{Conflicts: conflicts}
		}
		before, err := q.ListAcmAreaBranches(ctx, id)
		if err != nil {
			return audit.Entry{}, fmt.Errorf("loading acm area branches: %w", err)
		}
		if err := q.ClearAcmAreaBranches(ctx, id); err != nil {
			return audit.Entry{}, fmt.Errorf("clearing acm area branches: %w", err)
		}
		for _, b := range ids {
			if err := q.AddAcmAreaBranch(ctx, db.AddAcmAreaBranchParams{AcmAreaID: id, VendorBranchID: b, CreatedBy: actorID}); err != nil {
				if isUniqueViolation(err) { // raced with another area's assignment
					return audit.Entry{}, &AcmAreaBranchConflictError{}
				}
				return audit.Entry{}, fmt.Errorf("adding acm area branch: %w", err)
			}
		}
		prev := make([]int64, len(before))
		for i, b := range before {
			prev[i] = b.VendorBranchID
		}
		return audit.Entry{ActorID: actorID, Action: "acm_area_branches_set", EntityType: "acm_area", EntityID: id,
			Before: map[string]any{"vendor_branch_ids": prev}, After: map[string]any{"vendor_branch_ids": ids}, IP: ip}, nil
	}))
}

// SetMembers replaces the area's member list; members must be active ACM-USER/ACM-SPV users.
// Returns the updated detail read from the primary.
func (s *AcmAreaAdminService) SetMembers(ctx context.Context, actorID int64, actorRole string, id int64, userIDs []int64, ip string) (*AcmAreaDetail, error) {
	ids := uniqueIDs(userIDs)
	return s.thenLoad(ctx, id, s.inTx(ctx, actorRole, func(q *db.Queries, _ *audit.Writer) (audit.Entry, error) {
		if err := requireActiveAcmArea(ctx, q, id); err != nil {
			return audit.Entry{}, err
		}
		n, err := q.CountActiveAcmUsers(ctx, ids)
		if err != nil {
			return audit.Entry{}, fmt.Errorf("checking members: %w", err)
		}
		if int(n) != len(ids) {
			return audit.Entry{}, &ValidationError{Field: "user_ids", Message: "anggota harus user aktif dengan role ACM-USER atau ACM-SPV"}
		}
		before, err := q.ListAcmAreaMembers(ctx, id)
		if err != nil {
			return audit.Entry{}, fmt.Errorf("loading acm area members: %w", err)
		}
		if err := q.ClearAcmAreaMembers(ctx, id); err != nil {
			return audit.Entry{}, fmt.Errorf("clearing acm area members: %w", err)
		}
		for _, u := range ids {
			if err := q.AddAcmAreaMember(ctx, db.AddAcmAreaMemberParams{AcmAreaID: id, UserID: u, CreatedBy: actorID}); err != nil {
				return audit.Entry{}, fmt.Errorf("adding acm area member: %w", err)
			}
		}
		prev := make([]int64, len(before))
		for i, m := range before {
			prev[i] = m.UserID
		}
		return audit.Entry{ActorID: actorID, Action: "acm_area_members_set", EntityType: "acm_area", EntityID: id,
			Before: map[string]any{"user_ids": prev}, After: map[string]any{"user_ids": ids}, IP: ip}, nil
	}))
}

// thenLoad returns the area detail from the primary once the mutation committed.
func (s *AcmAreaAdminService) thenLoad(ctx context.Context, id int64, err error) (*AcmAreaDetail, error) {
	if err != nil {
		return nil, err
	}
	return loadAcmAreaDetail(ctx, db.New(s.pool), id)
}

func getAcmArea(ctx context.Context, q *db.Queries, id int64) (db.AcmArea, error) {
	a, err := q.GetAcmAreaAdmin(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.AcmArea{}, ErrAcmAreaNotFound
	}
	if err != nil {
		return db.AcmArea{}, fmt.Errorf("loading acm area: %w", err)
	}
	return a, nil
}

func requireActiveAcmArea(ctx context.Context, q *db.Queries, id int64) error {
	a, err := getAcmArea(ctx, q, id)
	if err != nil {
		return err
	}
	if !a.IsActive {
		return ErrAcmAreaInactive
	}
	return nil
}

func mapAcmAreaNameErr(err error) error {
	if isUniqueViolation(err) {
		return ErrAcmAreaNameConflict
	}
	return fmt.Errorf("writing acm area: %w", err)
}

// uniqueIDs drops non-positive ids and duplicates and sorts (stable audit payloads); never nil.
func uniqueIDs(in []int64) []int64 {
	out := make([]int64, 0, len(in))
	for _, id := range in {
		if id > 0 {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
