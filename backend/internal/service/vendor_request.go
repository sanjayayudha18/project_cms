package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Actor carries who is acting, resolved from the JWT by the handler.
// Role is singular (middleware.AuthContext.Role) -- this codebase's users
// have exactly one role, not a set.
type Actor struct {
	UserID int64
	Role   string
	IP     string
}

// checkerRoles: Checker = approves/rejects/cancels-from-approved a request.
// ADMIN is both maker and checker (Req 2.4, 2.5, design.md "Roles"; the
// handler's own vendorRequestMakerRoles/vendorRequestCheckerRoles carry the
// maker-role set for route gates). Role comparison is case-insensitive,
// matching middleware.RequireRoles' convention.
var checkerRoles = []string{"ADMIN", "ATM-SPV", "BRANCH-ATM-SPV"}

func isChecker(role string) bool { return roleIn(role, checkerRoles) }

func roleIn(role string, set []string) bool {
	for _, r := range set {
		if strings.EqualFold(role, r) {
			return true
		}
	}
	return false
}

// Sentinel errors the service wraps its own errors with, so callers (the
// HTTP handler) can map them to specific status codes via errors.Is — same
// convention as approval.ErrNotAuthorized etc.
var (
	ErrNotFound          = errors.New("vendor request not found")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrNotCreator        = errors.New("only the creator can perform this action")
	ErrNotChecker        = errors.New("only a checker can perform this action")
	ErrNotAuthorized     = errors.New("actor is not authorized to perform this action")
	ErrSelfApproval      = errors.New("four-eyes: checker must differ from maker")
	ErrEmptyItems        = errors.New("request must contain at least one item")
	ErrRejectReasonEmpty = errors.New("rejection reason is required")
	ErrCancelReasonEmpty = errors.New("cancellation reason is required")
	ErrInvalidItems      = errors.New("one or more items are invalid")
	ErrDuplicateItems    = errors.New("duplicate item in payload")
	ErrNumberExhausted   = errors.New("request number sequence exhausted for date")
	ErrNumberGeneration  = errors.New("could not generate request number")
	ErrAlreadyCanceled   = errors.New("vendor request is already canceled")
)

// InvalidItemsError wraps ErrInvalidItems with the specific items that don't
// match any dmaa_atm_forecast row, so the handler can name them in the 400
// response (Req 4.4 "identifying the invalid items"). errors.Is(err,
// ErrInvalidItems) still works via Unwrap.
type InvalidItemsError struct {
	Items []ItemInput
}

func (e *InvalidItemsError) Error() string {
	return fmt.Sprintf("%d item(s) do not match any dmaa_atm_forecast row", len(e.Items))
}

func (e *InvalidItemsError) Unwrap() error { return ErrInvalidItems }

// ItemInput is one requested line item (create or update-items payload).
// TerminalID is the requirements' "atm_id" (design.md terminology note).
// Brand/LokasiATM (Req 3, Q4) are populated by the operator for a
// Manual_Request's items only; DMAA-backed items leave them "" and the read
// path derives display values from the existing join chain instead.
type ItemInput struct {
	TerminalID      string
	PeriodePred     time.Time
	Denom           int32
	AmountReplenish int64
	Brand           string
	LokasiATM       string
}

// CreateVendorRequestInput is the validated Create payload. CIT-2 (Req 2, 3,
// 4) adds ReplenishDate/RequestCategory/IsManual/VendorID: every create is
// now constrained to one resolved CIT vendor (Q2).
// replenishment-request-enhancements (Req 1): RequestCategory is required
// for EVERY create (manual and DMAA-backed alike) and validated against
// ReplenishDate by validateCategoryDateConsistency.
type CreateVendorRequestInput struct {
	ForecastDate    time.Time
	ReplenishDate   time.Time
	RequestCategory string // required: "planned" | "emergency" | "additional"
	IsManual        bool
	VendorID        int64
	Notes           string // <= 500 chars
	Items           []ItemInput
}

// wibZone is Asia/Jakarta (WIB, UTC+7) — same fixed-offset convention as
// handler.wibZone (audit_log_handler.go): Asia/Jakarta has no DST, so this is
// exact and doesn't depend on the container image shipping IANA tzdata.
var wibZone = time.FixedZone("WIB", 7*60*60)

// jakartaCalendarDate returns today (Asia/Jakarta) + offsetDays, normalized
// to a UTC-midnight time.Time so it compares directly against date-only
// fields parsed via time.Parse("2006-01-02", ...) (e.g. ReplenishDate).
func jakartaCalendarDate(offsetDays int) time.Time {
	now := time.Now().In(wibZone)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, offsetDays)
}

// UserRef is a user id + display name, embedded wherever a response names
// an actor (creator/approver/rejecter).
type UserRef struct {
	ID       int64
	FullName string
}

// VendorRequestItemOut is one persisted line item in a response.
type VendorRequestItemOut struct {
	ID              int64
	TerminalID      string
	PeriodePred     time.Time
	Denom           int32
	AmountReplenish int64
	AmountRefund    int64
}

// VendorRequestDetail is the full single-request response shape (Req 9.6).
// CIT-2 (Req 2.9-2.10, 5.10) adds ReplenishDate/RequestCategory/IsCanceled/
// IsManual, additive fields alongside the existing flat shape.
type VendorRequestDetail struct {
	ID                 int64
	RequestNumber      string
	ForecastDate       time.Time
	Status             string
	Notes              string
	CreatedBy          UserRef
	ApprovedBy         *UserRef
	RejectedBy         *UserRef
	RejectionReason    string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	SubmittedAt        *time.Time
	ApprovedAt         *time.Time
	RejectedAt         *time.Time
	Items              []VendorRequestItemOut
	TotalAmount        int64
	ReplenishDate      *time.Time // nil only for legacy rows
	RequestCategory    *string    // nil only for legacy rows (Req 1.9: set on every new create)
	IsCanceled         bool
	IsManual           bool
	CancellationReason *string // nil unless canceled after migration 038 (Req 3.11 Opsi B)
}

// VendorRequestSummary is one row of the list response (Req 9.4). CIT-2
// additions mirror VendorRequestDetail's.
type VendorRequestSummary struct {
	ID                 int64
	RequestNumber      string
	ForecastDate       time.Time
	Status             string
	Notes              string
	ItemCount          int64
	TotalAmount        int64
	CreatedBy          UserRef
	ApprovedBy         *UserRef
	CreatedAt          time.Time
	SubmittedAt        *time.Time
	ApprovedAt         *time.Time
	RejectedAt         *time.Time
	ReplenishDate      *time.Time
	RequestCategory    *string
	IsCanceled         bool
	IsManual           bool
	CancellationReason *string
}

// ListVendorRequestParams holds validated list-endpoint filters (Req 9.1-9.3).
// Empty Status / ForecastDate ("") and CreatedBy (0) mean "no filter" —
// same empty-sentinel convention as ListDmaaForecastParams.
type ListVendorRequestParams struct {
	Status          []string // comma-joined into one filter arg; each must be a valid status
	ForecastDate    string   // YYYY-MM-DD or ""
	CreatedBy       int64    // 0 = no filter
	RequestNumber   string   // partial match, "" = no filter (Req 13.3, frontend-driven; not in requirements.md Req 9)
	IncludeCanceled bool     // false (default) excludes is_canceled rows (Req 5.5)
	Page            int
	PageSize        int
}

// ListVendorRequestResult is the paginated list response.
type ListVendorRequestResult struct {
	Data       []VendorRequestSummary
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// BrowseForecastParams holds validated forecast-browse filters (Req 3.1-3.3,
// CIT-2 Req 1). Brand is optional (""= no filter, Req 1.9); FLMVendor and
// FLMVendorRegion are required with no empty-sentinel (Req 1.2-1.5) --
// BrowseForecast rejects a browse missing either before calling the repo.
type BrowseForecastParams struct {
	ForecastDate    string // YYYY-MM-DD, required
	TerminalID      string
	Brand           string
	FLMVendor       string
	FLMVendorRegion string
	Page            int
	PageSize        int
}

// ForecastRow is one dmaa_atm_forecast row for the Forecast Browser.
// replenishment-request-enhancements (Req 5): PriorityClass/Paket map NULL
// to "" (UI renders "-"); Escrow is a decimal string (never float, Req
// 5.10), nil when the terminal has no itm_replenish row (Req 5.11 renders
// "-", never "0.00"). AmountRefund stays in the DTO and on the wire (Req
// 5.1 drops only the rendered column).
type ForecastRow struct {
	TerminalID      string
	PeriodePred     time.Time
	Denom           int32
	AmountReplenish int64
	AmountRefund    int64
	DmaaFileID      int64
	LokasiATM       string
	Brand           string
	FLMVendor       string
	FLMVendorRegion string
	PriorityClass   string
	Paket           string
	Escrow          *string
}

// BrowseForecastResult is the paginated forecast-browse response.
type BrowseForecastResult struct {
	Data       []ForecastRow
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// VendorOption is one selectable CIT vendor (CIT-2 Req 1.2, 3 Q2): the id is
// what a manual create's vendor_id must carry; the name is what the
// Forecast Browser's required FLM Vendor filter matches on.
type VendorOption struct {
	ID   int64
	Name string
}

// VendorOptionsResult backs GET /vendor-requests/vendors: the option lists
// for the Forecast Browser's required FLM Vendor / FLM Vendor Region
// selects (Req 1.2, 1.3) and the manual-request vendor select (Req 3, Q2).
type VendorOptionsResult struct {
	Vendors []VendorOption
	Regions []string
}

// AuditEntry is one projected audit_logs row for GET /{id}/audit-log (Req
// 16.4). PreviousState/NewState are projected out of before.state/after.state
// (see design.md's audit_logs.before/after mapping); Metadata is After minus
// the "state" key.
type AuditEntry struct {
	ID            int64
	EntityType    string
	EntityID      int64
	Action        string
	PerformedBy   int64
	PerformedAt   time.Time
	PreviousState *string
	NewState      *string
	Metadata      map[string]any
}

// VendorRequestRepository is the sqlc-generated query surface the service
// needs, satisfied by *db.Queries whether pool-scoped (reads) or
// tx-scoped via db.New(tx) (mutations) — defined where it's used, per Go
// convention (see DmaaForecastRepository).
type VendorRequestRepository interface {
	CreateVendorRequest(ctx context.Context, arg db.CreateVendorRequestParams) (db.VendorRequest, error)
	InsertVendorRequestItem(ctx context.Context, arg db.InsertVendorRequestItemParams) (db.VendorRequestItem, error)
	DeleteVendorRequestItems(ctx context.Context, vendorRequestID int64) error
	GetVendorRequest(ctx context.Context, id int64) (db.VendorRequest, error)
	GetVendorRequestForUpdate(ctx context.Context, id int64) (db.VendorRequest, error)
	ListVendorRequestItems(ctx context.Context, vendorRequestID int64) ([]db.VendorRequestItem, error)
	GetVendorRequestDetail(ctx context.Context, id int64) (db.GetVendorRequestDetailRow, error)
	ListVendorRequests(ctx context.Context, arg db.ListVendorRequestsParams) ([]db.ListVendorRequestsRow, error)
	CountVendorRequests(ctx context.Context, arg db.CountVendorRequestsParams) (int64, error)
	ListForecastForDate(ctx context.Context, arg db.ListForecastForDateParams) ([]db.ListForecastForDateRow, error)
	CountForecastForDate(ctx context.Context, arg db.CountForecastForDateParams) (int64, error)
	ForecastRowExists(ctx context.Context, arg db.ForecastRowExistsParams) (db.ForecastRowExistsRow, error)
	UpdateVendorRequestStatus(ctx context.Context, arg db.UpdateVendorRequestStatusParams) (db.VendorRequest, error)
	ListAuditLogsByEntity(ctx context.Context, entityType string, entityID int64) ([]db.AuditLog, error)
	// CIT-2 additions (Req 4, 5). GetVendorForRequestNumber is not listed
	// here: like ForecastRowExists's sibling resolveItems, the number
	// generator (createWithRetryingNumber) takes the tx-scoped *db.Queries
	// concretely rather than through this interface.
	NextRequestNumberSeq(ctx context.Context, arg db.NextRequestNumberSeqParams) (int32, error)
	SoftCancelVendorRequest(ctx context.Context, arg db.SoftCancelVendorRequestParams) (db.VendorRequest, error)
	// ListVendorOptions support (Req 1.2, 1.3, 3 Q2).
	ListActiveVendors(ctx context.Context) ([]db.ListActiveVendorsRow, error)
	ListDistinctVendorBranchRegions(ctx context.Context) ([]*string, error)
}

// VendorRequestPool is the *pgxpool.Pool surface the service needs: db.DBTX
// (Exec/Query/QueryRow) for read-only queries that don't need a transaction,
// plus BeginTx for every mutating operation. Every state transition, Create,
// and UpdateItems runs inside a single DB transaction (design.md "Error
// Handling: Transactional integrity") — deliberately stronger than this
// codebase's other services, which so far only need autocommit single
// statements.
type VendorRequestPool interface {
	db.DBTX
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// VendorRequestServicer is the interface the HTTP handler (task 4) depends on.
type VendorRequestServicer interface {
	BrowseForecast(ctx context.Context, params BrowseForecastParams) (*BrowseForecastResult, error)
	ListVendorOptions(ctx context.Context) (*VendorOptionsResult, error)
	Create(ctx context.Context, actor Actor, in CreateVendorRequestInput) (*VendorRequestDetail, error)
	UpdateItems(ctx context.Context, actor Actor, id int64, items []ItemInput) (*VendorRequestDetail, error)
	Submit(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error)
	Approve(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error)
	Reject(ctx context.Context, actor Actor, id int64, reason string) (*VendorRequestDetail, error)
	Revise(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error)
	Cancel(ctx context.Context, actor Actor, id int64, reason string) (*VendorRequestDetail, error)
	List(ctx context.Context, params ListVendorRequestParams) (*ListVendorRequestResult, error)
	Get(ctx context.Context, id int64) (*VendorRequestDetail, error)
	AuditLog(ctx context.Context, id int64) ([]AuditEntry, error)
}

// VendorRequestService implements VendorRequestServicer.
type VendorRequestService struct {
	pool VendorRequestPool
	read VendorRequestRepository // db.New(pool): read-only queries, no tx needed
}

// NewVendorRequestService creates a VendorRequestService wrapping the given
// pool (instantiate with the primary pool — Sec 6: writes, and read-after-
// write on detail/list, both go to primary). Unlike design.md's sketch
// (NewVendorRequestService(pool, auditWriter)), this takes only the pool: a
// pool-scoped audit.Writer would write outside the transition's transaction,
// breaking the atomicity design.md itself requires ("a failed audit write
// rolls back the transition"). audit.NewWriter(tx) is built fresh per
// mutating call instead — see transition()/Create()/UpdateItems() in
// vendor_request_actions.go.
func NewVendorRequestService(pool VendorRequestPool) *VendorRequestService {
	return &VendorRequestService{pool: pool, read: db.New(pool)}
}

// --- State machine -----------------------------------------------------

type action string

const (
	actionSubmit  action = "submit"
	actionApprove action = "approve"
	actionReject  action = "reject"
	actionRevise  action = "revise"
	actionCancel  action = "cancel"
)

// transitions is the valid-transition table (Req 2.1). approved->processing,
// processing->completed/failed are part of the DB CHECK constraint (migration
// 028, for state-machine consistency) but out of scope for this feature: no
// requirement or endpoint here triggers them — they belong to whatever
// downstream module executes the approved order.
var transitions = map[string]map[action]string{
	"draft":            {actionSubmit: "pending_approval", actionCancel: "cancelled"},
	"pending_approval": {actionApprove: "approved", actionReject: "rejected", actionCancel: "cancelled"},
	"rejected":         {actionRevise: "draft"},
	"approved":         {actionCancel: "cancelled"},
}

// nextState returns the target status for (cur, a), or ok=false if the
// transition doesn't exist (Req 2.2: ErrInvalidTransition, no state change).
func nextState(cur string, a action) (string, bool) {
	m, ok := transitions[cur]
	if !ok {
		return "", false
	}
	to, ok := m[a]
	return to, ok
}

// checkActor enforces Req 2.4 (Maker: submit/revise/cancel-from-draft),
// 2.5/2.7/2.8 (Checker != creator: approve/reject), the resolved cancel
// rule for pending_approval: Req 2.12 (Checker) and Req 10.4 (creator)
// directly contradict each other for that one case — resolved as a union
// (either may cancel a pending request) per team decision — and
// replenishment-request-enhancements Req 3.7/3.12: cancelling an already
// approved order is Checker-only (no four-eyes here, this is a direct
// cancellation rather than an approve/reject pair, so a Checker who is also
// the creator is still allowed).
func checkActor(actor Actor, req db.VendorRequest, a action) error {
	switch a {
	case actionSubmit, actionRevise:
		if actor.UserID != req.CreatedBy {
			return ErrNotCreator
		}
	case actionApprove, actionReject:
		if !isChecker(actor.Role) {
			return ErrNotChecker
		}
		if actor.UserID == req.CreatedBy {
			return ErrSelfApproval
		}
	case actionCancel:
		switch req.Status {
		case "draft":
			if actor.UserID != req.CreatedBy {
				return ErrNotCreator
			}
		case "approved":
			if !isChecker(actor.Role) {
				return ErrNotChecker
			}
		default:
			isCreator := actor.UserID == req.CreatedBy
			isCheckerNotCreator := isChecker(actor.Role) && actor.UserID != req.CreatedBy
			if !isCreator && !isCheckerNotCreator {
				return ErrNotAuthorized
			}
		}
	}
	return nil
}

// --- small conversion helpers ------------------------------------------

func toPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func timestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

// dateToPtr converts a nullable pgtype.Date to *time.Time (nil when NULL),
// same convention as timestamptzToPtr, for the new nullable replenish_date
// column (Req 2.9, 5.10).
func dateToPtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func notesOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// newAuditWriter builds a transaction-scoped audit.Writer so a write
// failure rolls back with the rest of the transition (see
// NewVendorRequestService's doc comment).
func newAuditWriter(tx pgx.Tx) *audit.Writer {
	return audit.NewWriter(tx)
}
