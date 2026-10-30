package exportjob

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// Preview page size and time limit (Req 4.3).
const (
	DefaultPreviewLimit = 50
	MaxPreviewLimit     = 100
	PreviewTimeout      = 10 * time.Second
)

// Preview is one page of a master-data table as the export file would hold it (Req 4.2).
type Preview struct {
	Columns   []string   `json:"columns"`
	Rows      [][]string `json:"rows"`
	Total     int64      `json:"total"`
	NextAfter *int64     `json:"next_after"` // null on the last page
	Limit     int        `json:"limit"`
}

// ParsePaging validates after (>= 0, default 0) and limit (1..100, default 50); else ErrValidation (Req 4.3).
func ParsePaging(rawAfter, rawLimit string) (after int64, limit int, err error) {
	limit = DefaultPreviewLimit
	if rawAfter != "" {
		after, err = strconv.ParseInt(rawAfter, 10, 64)
		if err != nil || after < 0 {
			return 0, 0, ErrValidation
		}
	}
	if rawLimit != "" {
		limit, err = strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > MaxPreviewLimit {
			return 0, 0, ErrValidation
		}
	}
	return after, limit, nil
}

// Previewer serves preview pages: scope on the primary, rows on the replica. Writes nothing,
// audit included (D5, Req 4.11).
type Previewer struct {
	scope     createQueries  // primary: vendor / cabang lookup, same as Create (Req 4.1)
	reader    snapshotReader // replica snapshot (Req 4.6)
	producers map[Kind]map[string]Producer
	now       func() time.Time
}

// NewPreviewer wires the previewer (pass primary twice when there is no replica).
func NewPreviewer(primary, replica *pgxpool.Pool, producers map[Kind]map[string]Producer) *Previewer {
	return &Previewer{
		scope:     repository.New(primary),
		reader:    pgReader{pool: replica},
		producers: producers,
		now:       time.Now,
	}
}

// Preview returns one page of p's rows with id > after. p must come from ParseParams,
// after/limit from ParsePaging. Check order: actor, role (before any DB), producer,
// scope (primary), then Count + Page in one replica snapshot.
// Req 4.1-4.3, 4.6, 4.8, 4.9, 4.11, 7.1.
func (v *Previewer) Preview(ctx context.Context, a Actor, p Params, after int64, limit int) (Preview, error) {
	if a.UserID == 0 {
		return Preview{}, ErrUnauthenticated
	}
	if !allowed(p.Kind, a.Role) { // same rule as export (Req 4.8, 7.1)
		return Preview{}, ErrForbidden
	}
	pp, ok := v.producers[p.Kind][p.Entity].(PreviewProducer)
	if !ok { // report kinds and unknown entities
		return Preview{}, ErrUnknownEntity
	}
	if after < 0 || limit < 1 || limit > MaxPreviewLimit { // ParsePaging guards this; fail closed anyway
		return Preview{}, ErrValidation
	}

	ctx, cancel := context.WithTimeout(ctx, PreviewTimeout)
	defer cancel()
	if _, _, err := resolveScope(ctx, v.scope, p); err != nil {
		return Preview{}, err
	}

	asOf := v.now() // Harga Paket as_of = WIB date of the request (Req 4.9)
	q, rollback, err := v.reader.Begin(ctx)
	if err != nil {
		return Preview{}, replicaErr("begin snapshot", err)
	}
	defer rollback()

	total, err := pp.Count(ctx, q, p, asOf)
	if err != nil {
		return Preview{}, replicaErr("count", err)
	}
	rows, last, err := pp.Page(ctx, q, p, asOf, after, limit)
	if err != nil {
		return Preview{}, replicaErr("page", err)
	}
	// ponytail: Page returns only the last id, so a full page probes one row past it
	// instead of reading limit+1; fold into one query if Page ever returns per-row ids.
	var next *int64
	if len(rows) == limit {
		more, _, err := pp.Page(ctx, q, p, asOf, last, 1)
		if err != nil {
			return Preview{}, replicaErr("probe next page", err)
		}
		if len(more) > 0 {
			next = &last
		}
	}
	if rows == nil {
		rows = [][]string{} // JSON [] rather than null
	}
	return Preview{Columns: pp.Header(), Rows: rows, Total: total, NextAfter: next, Limit: limit}, nil
}

// replicaErr maps a replica recovery conflict to ErrUnavailable, anything else to ErrInternal.
func replicaErr(op string, err error) error {
	if isRecoveryConflict(err) {
		return fmt.Errorf("%w: %s: %w", ErrUnavailable, op, err)
	}
	return wrapInternal(op, err)
}
