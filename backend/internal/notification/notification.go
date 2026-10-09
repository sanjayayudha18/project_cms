// Package notification sends in-app notifications and queues their emails
// (Phase 0.3, .claude/sdlc/notification/spec.md). Send writes inside the
// caller's transaction, so a notification commits or rolls back together
// with the business event that triggered it; emails go out later through
// Worker (outbox), so an SMTP outage never fails the business action.
package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ErrInvalidMessage is returned by Send for a malformed Message. Callers pass
// constants, so this signals a programming error, not user input (FR1.5).
var ErrInvalidMessage = errors.New("notification: invalid message")

const (
	maxTitleLen = 120
	maxBodyLen  = 1000

	emailStatusPending = "pending"
	emailStatusSkipped = "skipped"

	emailFooter = "\n\n--\nPesan otomatis CROWN Cash Management System. Mohon tidak membalas email ini."
)

// Message is one notification. Type is a stable code (e.g.
// "visit_quota.over_quota"); Link is a path in the recipient's portal.
type Message struct {
	Type       string
	Title      string
	Body       string
	Link       string
	EntityType string
	EntityID   *int64
	Email      bool
}

// Recipients is the union of specific users, every active user of a role
// (expanded at send time), and vendors (their active users in-app + email,
// plus email-only to vendor_pics flagged is_notification_recipient), and
// vendor branches (cit-send-vendor FR7.1: same, limited to users/PICs pinned
// to that branch or vendor-wide).
type Recipients struct {
	UserIDs         []int64
	Roles           []string
	VendorIDs       []int64
	VendorBranchIDs []int64
}

// Querier is the subset of *db.Queries Send needs; pass the caller's
// tx-bound db.New(tx).
type Querier interface {
	ListActiveUserRecipientsByIDs(ctx context.Context, ids []int64) ([]db.ListActiveUserRecipientsByIDsRow, error)
	ListActiveUserRecipientsByRoles(ctx context.Context, roles []string) ([]db.ListActiveUserRecipientsByRolesRow, error)
	ListActiveUserRecipientsByVendors(ctx context.Context, vendorIds []int64) ([]db.ListActiveUserRecipientsByVendorsRow, error)
	ListVendorNotificationPicEmails(ctx context.Context, vendorIds []int64) ([]string, error)
	ListActiveUserRecipientsByVendorBranches(ctx context.Context, branchIds []int64) ([]db.ListActiveUserRecipientsByVendorBranchesRow, error)
	ListVendorBranchNotificationPicEmails(ctx context.Context, branchIds []int64) ([]string, error)
	InsertNotification(ctx context.Context, arg db.InsertNotificationParams) (int64, error)
	InsertNotificationEmail(ctx context.Context, arg db.InsertNotificationEmailParams) error
}

// Service writes notifications. smtpEnabled=false (no SMTP_HOST) stores
// outbox rows as "skipped" so dev never builds a backlog that would be sent
// in a burst once SMTP is configured (FR1.3).
type Service struct {
	smtpEnabled bool
}

func NewService(smtpEnabled bool) *Service {
	return &Service{smtpEnabled: smtpEnabled}
}

type userRecipient struct {
	id    int64
	email string
}

// Send fans msg out to every recipient: one notifications row per unique
// user and, when msg.Email, one notification_emails row per unique address.
func (s *Service) Send(ctx context.Context, q Querier, msg Message, to Recipients) error {
	if err := validate(msg); err != nil {
		return err
	}
	users, err := resolveUsers(ctx, q, to)
	if err != nil {
		return err
	}
	var picEmails []string
	if msg.Email && len(to.VendorIDs) > 0 {
		if picEmails, err = q.ListVendorNotificationPicEmails(ctx, to.VendorIDs); err != nil {
			return fmt.Errorf("list vendor pic emails: %w", err)
		}
	}
	if msg.Email && len(to.VendorBranchIDs) > 0 {
		branchPics, err := q.ListVendorBranchNotificationPicEmails(ctx, to.VendorBranchIDs)
		if err != nil {
			return fmt.Errorf("list vendor branch pic emails: %w", err)
		}
		picEmails = append(picEmails, branchPics...)
	}
	if len(users) == 0 && len(picEmails) == 0 {
		slog.InfoContext(ctx, "notification has no recipients", "type", msg.Type)
		return nil
	}

	seenEmail := map[string]bool{}
	for _, u := range users {
		id, err := q.InsertNotification(ctx, db.InsertNotificationParams{
			RecipientUserID: u.id,
			Type:            msg.Type,
			Title:           msg.Title,
			Body:            msg.Body,
			Link:            optional(msg.Link),
			EntityType:      optional(msg.EntityType),
			EntityID:        msg.EntityID,
		})
		if err != nil {
			return fmt.Errorf("insert notification: %w", err)
		}
		if msg.Email {
			if err := s.queueEmail(ctx, q, &id, u.email, msg, seenEmail); err != nil {
				return err
			}
		}
	}
	for _, addr := range picEmails {
		if err := s.queueEmail(ctx, q, nil, addr, msg, seenEmail); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) queueEmail(ctx context.Context, q Querier, notificationID *int64, raw string, msg Message, seen map[string]bool) error {
	addr, ok := normalizeEmail(raw)
	if !ok {
		slog.WarnContext(ctx, "notification email skipped: invalid address", "type", msg.Type)
		return nil
	}
	if seen[addr] {
		return nil
	}
	seen[addr] = true
	status := emailStatusSkipped
	if s.smtpEnabled {
		status = emailStatusPending
	}
	if err := q.InsertNotificationEmail(ctx, db.InsertNotificationEmailParams{
		NotificationID: notificationID,
		ToAddress:      addr,
		Subject:        msg.Title,
		Body:           msg.Body + emailFooter,
		Status:         status,
	}); err != nil {
		return fmt.Errorf("insert notification email: %w", err)
	}
	return nil
}

// resolveUsers expands Recipients into unique active users, ordered by id.
// Queries for an empty field are skipped.
func resolveUsers(ctx context.Context, q Querier, to Recipients) ([]userRecipient, error) {
	byID := map[int64]string{}
	add := func(id int64, email string) {
		if _, ok := byID[id]; !ok {
			byID[id] = email
		}
	}
	if len(to.UserIDs) > 0 {
		rows, err := q.ListActiveUserRecipientsByIDs(ctx, to.UserIDs)
		if err != nil {
			return nil, fmt.Errorf("list recipients by id: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Email)
		}
	}
	if len(to.Roles) > 0 {
		rows, err := q.ListActiveUserRecipientsByRoles(ctx, to.Roles)
		if err != nil {
			return nil, fmt.Errorf("list recipients by role: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Email)
		}
	}
	if len(to.VendorIDs) > 0 {
		rows, err := q.ListActiveUserRecipientsByVendors(ctx, to.VendorIDs)
		if err != nil {
			return nil, fmt.Errorf("list recipients by vendor: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Email)
		}
	}
	if len(to.VendorBranchIDs) > 0 {
		rows, err := q.ListActiveUserRecipientsByVendorBranches(ctx, to.VendorBranchIDs)
		if err != nil {
			return nil, fmt.Errorf("list recipients by vendor branch: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Email)
		}
	}
	users := make([]userRecipient, 0, len(byID))
	for id, email := range byID {
		users = append(users, userRecipient{id: id, email: email})
	}
	sort.Slice(users, func(i, j int) bool { return users[i].id < users[j].id })
	return users, nil
}

func validate(msg Message) error {
	switch {
	case strings.TrimSpace(msg.Type) == "":
		return fmt.Errorf("%w: empty type", ErrInvalidMessage)
	case strings.TrimSpace(msg.Title) == "":
		return fmt.Errorf("%w: empty title", ErrInvalidMessage)
	case utf8.RuneCountInString(msg.Title) > maxTitleLen:
		return fmt.Errorf("%w: title longer than %d", ErrInvalidMessage, maxTitleLen)
	case utf8.RuneCountInString(msg.Body) > maxBodyLen:
		return fmt.Errorf("%w: body longer than %d", ErrInvalidMessage, maxBodyLen)
	case msg.Link != "" && (!strings.HasPrefix(msg.Link, "/") || strings.HasPrefix(msg.Link, "//")):
		// Both portals navigate to Link as-is: only an in-app path is allowed,
		// never an absolute or protocol-relative URL (review R3).
		return fmt.Errorf("%w: link must be an in-app path", ErrInvalidMessage)
	}
	return nil
}

// normalizeEmail lowercases/trims an address and rejects anything that is
// not a bare RFC 5322 address (also blocks header injection via CR/LF).
func normalizeEmail(raw string) (string, bool) {
	addr := strings.ToLower(strings.TrimSpace(raw))
	parsed, err := mail.ParseAddress(addr)
	if err != nil || parsed.Address != addr {
		return "", false
	}
	return addr, true
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
