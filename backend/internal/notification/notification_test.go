package notification

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeQuerier records inserts and serves canned recipient rows.
type fakeQuerier struct {
	byIDs     []db.ListActiveUserRecipientsByIDsRow
	byRoles   []db.ListActiveUserRecipientsByRolesRow
	byVendors []db.ListActiveUserRecipientsByVendorsRow
	pics      []string
	byBranch  []db.ListActiveUserRecipientsByVendorBranchesRow
	branchPic []string

	calls         []string
	notifications []db.InsertNotificationParams
	emails        []db.InsertNotificationEmailParams
	nextID        int64
	insertErr     error
}

func (f *fakeQuerier) ListActiveUserRecipientsByIDs(_ context.Context, _ []int64) ([]db.ListActiveUserRecipientsByIDsRow, error) {
	f.calls = append(f.calls, "byIDs")
	return f.byIDs, nil
}

func (f *fakeQuerier) ListActiveUserRecipientsByRoles(_ context.Context, _ []string) ([]db.ListActiveUserRecipientsByRolesRow, error) {
	f.calls = append(f.calls, "byRoles")
	return f.byRoles, nil
}

func (f *fakeQuerier) ListActiveUserRecipientsByVendors(_ context.Context, _ []int64) ([]db.ListActiveUserRecipientsByVendorsRow, error) {
	f.calls = append(f.calls, "byVendors")
	return f.byVendors, nil
}

func (f *fakeQuerier) ListVendorNotificationPicEmails(_ context.Context, _ []int64) ([]string, error) {
	f.calls = append(f.calls, "pics")
	return f.pics, nil
}

func (f *fakeQuerier) ListActiveUserRecipientsByVendorBranches(_ context.Context, _ []int64) ([]db.ListActiveUserRecipientsByVendorBranchesRow, error) {
	f.calls = append(f.calls, "byBranch")
	return f.byBranch, nil
}

func (f *fakeQuerier) ListVendorBranchNotificationPicEmails(_ context.Context, _ []int64) ([]string, error) {
	f.calls = append(f.calls, "branchPics")
	return f.branchPic, nil
}

func (f *fakeQuerier) InsertNotification(_ context.Context, arg db.InsertNotificationParams) (int64, error) {
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	f.nextID++
	f.notifications = append(f.notifications, arg)
	return f.nextID, nil
}

func (f *fakeQuerier) InsertNotificationEmail(_ context.Context, arg db.InsertNotificationEmailParams) error {
	f.emails = append(f.emails, arg)
	return nil
}

func validMsg() Message {
	return Message{Type: "visit_quota.over_quota", Title: "Kelebihan kuota kunjungan", Body: "isi", Email: true}
}

func TestSend_ValidatesMessage(t *testing.T) {
	cases := []struct {
		name string
		msg  Message
	}{
		{"empty type", Message{Title: "t"}},
		{"empty title", Message{Type: "x"}},
		{"title too long", Message{Type: "x", Title: strings.Repeat("a", 121)}},
		{"body too long", Message{Type: "x", Title: "t", Body: strings.Repeat("é", 1001)}},
		{"absolute link", Message{Type: "x", Title: "t", Link: "https://evil.example/x"}},
		{"protocol-relative link", Message{Type: "x", Title: "t", Link: "//evil.example/x"}},
		{"javascript link", Message{Type: "x", Title: "t", Link: "javascript:alert(1)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakeQuerier{}
			err := NewService(true).Send(context.Background(), q, tc.msg, Recipients{UserIDs: []int64{1}})
			if !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v, want ErrInvalidMessage", err)
			}
			if len(q.calls) != 0 {
				t.Fatalf("queried %v before validating", q.calls)
			}
		})
	}
}

func TestSend_DedupesUsersAndEmails(t *testing.T) {
	q := &fakeQuerier{
		byIDs: []db.ListActiveUserRecipientsByIDsRow{{ID: 7, Email: "Maker@Bank.co.id"}},
		byRoles: []db.ListActiveUserRecipientsByRolesRow{
			{ID: 7, Email: "maker@bank.co.id"}, // same user via role
			{ID: 9, Email: " spv@bank.co.id "},
			{ID: 10, Email: "MAKER@bank.co.id"}, // different user, same address
		},
		byVendors: []db.ListActiveUserRecipientsByVendorsRow{{ID: 20, Email: "user@vendor.co.id"}},
		pics:      []string{"USER@vendor.co.id", "pic@vendor.co.id", "not-an-email"},
	}
	err := NewService(true).Send(context.Background(), q, validMsg(),
		Recipients{UserIDs: []int64{7}, Roles: []string{"ATM-SPV"}, VendorIDs: []int64{3}})
	if err != nil {
		t.Fatal(err)
	}

	gotUsers := []int64{}
	for _, n := range q.notifications {
		gotUsers = append(gotUsers, n.RecipientUserID)
	}
	if want := []int64{7, 9, 10, 20}; !equalInts(gotUsers, want) {
		t.Fatalf("in-app recipients = %v, want %v", gotUsers, want)
	}

	gotEmails := map[string]bool{}
	for _, e := range q.emails {
		if gotEmails[e.ToAddress] {
			t.Fatalf("duplicate email %q", e.ToAddress)
		}
		gotEmails[e.ToAddress] = true
		if e.Status != "pending" {
			t.Fatalf("status = %q, want pending", e.Status)
		}
	}
	for _, want := range []string{"maker@bank.co.id", "spv@bank.co.id", "user@vendor.co.id", "pic@vendor.co.id"} {
		if !gotEmails[want] {
			t.Fatalf("missing email %q in %v", want, gotEmails)
		}
	}
	if len(q.emails) != 4 {
		t.Fatalf("emails = %d, want 4 (invalid PIC address skipped)", len(q.emails))
	}
	for _, e := range q.emails {
		if e.ToAddress == "pic@vendor.co.id" && e.NotificationID != nil {
			t.Fatal("PIC email must not link to an in-app notification")
		}
		if e.ToAddress == "user@vendor.co.id" && e.NotificationID == nil {
			t.Fatal("user email must link to the user's notification")
		}
	}
}

func TestSend_SkipsUnusedRecipientQueries(t *testing.T) {
	q := &fakeQuerier{byIDs: []db.ListActiveUserRecipientsByIDsRow{{ID: 1, Email: "a@b.co"}}}
	if err := NewService(true).Send(context.Background(), q, validMsg(), Recipients{UserIDs: []int64{1}}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(q.calls, ",") != "byIDs" {
		t.Fatalf("calls = %v, want only byIDs", q.calls)
	}
}

func TestSend_NoRecipientsIsNotAnError(t *testing.T) {
	q := &fakeQuerier{}
	if err := NewService(true).Send(context.Background(), q, validMsg(), Recipients{Roles: []string{"ATM-SPV"}}); err != nil {
		t.Fatal(err)
	}
	if len(q.notifications)+len(q.emails) != 0 {
		t.Fatal("expected no writes")
	}
}

func TestSend_SMTPDisabledWritesSkipped(t *testing.T) {
	q := &fakeQuerier{byIDs: []db.ListActiveUserRecipientsByIDsRow{{ID: 1, Email: "a@b.co"}}}
	if err := NewService(false).Send(context.Background(), q, validMsg(), Recipients{UserIDs: []int64{1}}); err != nil {
		t.Fatal(err)
	}
	if len(q.emails) != 1 || q.emails[0].Status != "skipped" {
		t.Fatalf("emails = %+v, want one skipped", q.emails)
	}
}

func TestSend_EmailFalseWritesNoOutbox(t *testing.T) {
	q := &fakeQuerier{byIDs: []db.ListActiveUserRecipientsByIDsRow{{ID: 1, Email: "a@b.co"}}}
	msg := validMsg()
	msg.Email = false
	if err := NewService(true).Send(context.Background(), q, msg, Recipients{UserIDs: []int64{1}}); err != nil {
		t.Fatal(err)
	}
	if len(q.notifications) != 1 || len(q.emails) != 0 {
		t.Fatalf("notifications=%d emails=%d, want 1/0", len(q.notifications), len(q.emails))
	}
}

func TestSend_InsertErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	q := &fakeQuerier{byIDs: []db.ListActiveUserRecipientsByIDsRow{{ID: 1, Email: "a@b.co"}}, insertErr: boom}
	if err := NewService(true).Send(context.Background(), q, validMsg(), Recipients{UserIDs: []int64{1}}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func equalInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cit-send-vendor FR7.1: branch recipients merge with the others and dedupe
// like vendor recipients; PIC lookups only run when the message is emailed.
func TestSend_VendorBranchRecipients(t *testing.T) {
	q := &fakeQuerier{
		byIDs:     []db.ListActiveUserRecipientsByIDsRow{{ID: 30, Email: "ops@vendor.co.id"}},
		byBranch:  []db.ListActiveUserRecipientsByVendorBranchesRow{{ID: 30, Email: "ops@vendor.co.id"}, {ID: 31, Email: "branch@vendor.co.id"}},
		branchPic: []string{"pic.branch@vendor.co.id", "BRANCH@vendor.co.id"},
	}
	if err := NewService(true).Send(context.Background(), q, validMsg(),
		Recipients{UserIDs: []int64{30}, VendorBranchIDs: []int64{5}}); err != nil {
		t.Fatal(err)
	}
	gotUsers := []int64{}
	for _, n := range q.notifications {
		gotUsers = append(gotUsers, n.RecipientUserID)
	}
	if want := []int64{30, 31}; !equalInts(gotUsers, want) {
		t.Fatalf("in-app recipients = %v, want %v", gotUsers, want)
	}
	if len(q.emails) != 3 {
		t.Fatalf("emails = %d (%v), want 3 (2 users + 1 new PIC)", len(q.emails), q.emails)
	}

	noMail := &fakeQuerier{byBranch: []db.ListActiveUserRecipientsByVendorBranchesRow{{ID: 31, Email: "branch@vendor.co.id"}}}
	msg := validMsg()
	msg.Email = false
	if err := NewService(true).Send(context.Background(), noMail, msg, Recipients{VendorBranchIDs: []int64{5}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range noMail.calls {
		if c == "branchPics" {
			t.Fatal("PIC emails queried for an in-app-only message")
		}
	}
}
