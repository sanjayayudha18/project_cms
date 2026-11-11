package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRow struct{ err error }

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*int64)) = 7
	return nil
}

// fakeDB records the InsertAuditLog args.
type fakeDB struct {
	calls int
	args  []any
	err   error
}

func (f *fakeDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeDB) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (f *fakeDB) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	f.calls++
	f.args = args
	return fakeRow{err: f.err}
}

func TestWrite_RejectsMissingActorOrAction(t *testing.T) {
	for name, e := range map[string]Entry{
		"no actor":  {Action: "user_created"},
		"no action": {ActorID: 1},
	} {
		db := &fakeDB{}
		if err := NewWriter().Write(context.Background(), db, e); !errors.Is(err, ErrInvalidEntry) {
			t.Errorf("%s: err = %v, want ErrInvalidEntry", name, err)
		}
		if db.calls != 0 {
			t.Errorf("%s: DB touched before validation", name)
		}
	}
}

func TestWrite_MarshalFailureNeverReachesDB(t *testing.T) {
	db := &fakeDB{}
	err := NewWriter().Write(context.Background(), db, Entry{ActorID: 1, Action: "a", Before: make(chan int)})
	if err == nil || db.calls != 0 {
		t.Fatalf("err = %v, calls = %d; want error and no DB call", err, db.calls)
	}
}

func TestWrite_PassesFieldsAndNullsEmptyValues(t *testing.T) {
	db := &fakeDB{}
	e := Entry{ActorID: 9, Action: "user_updated", EntityType: "user", EntityID: 5,
		Before: map[string]string{"a": "1"}, After: nil, IP: "10.0.0.1"}
	if err := NewWriter().Write(context.Background(), db, e); err != nil {
		t.Fatal(err)
	}
	if db.calls != 1 || len(db.args) != 7 {
		t.Fatalf("calls=%d args=%d", db.calls, len(db.args))
	}
	if db.args[0] != int64(9) || db.args[1] != "user_updated" || db.args[2] != "user" || db.args[3] != int64(5) {
		t.Errorf("scalar args = %v", db.args[:4])
	}
	if string(db.args[4].([]byte)) != `{"a":"1"}` {
		t.Errorf("before = %s", db.args[4])
	}
	if b := db.args[5].([]byte); b != nil {
		t.Errorf("after should be NULL, got %s", b)
	}
	if ip := db.args[6].(*string); ip == nil || *ip != "10.0.0.1" {
		t.Errorf("ip = %v", ip)
	}

	db2 := &fakeDB{}
	_ = NewWriter().Write(context.Background(), db2, Entry{ActorID: 1, Action: "a"})
	if ip := db2.args[6].(*string); ip != nil {
		t.Errorf("empty IP must be NULL, got %v", *ip)
	}
}

func TestWrite_WrapsInsertError(t *testing.T) {
	boom := errors.New("boom")
	err := NewWriter().Write(context.Background(), &fakeDB{err: boom}, Entry{ActorID: 1, Action: "a"})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want wrapping boom", err)
	}
}
