package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRow struct{ err error }

func (r fakeRow) Scan(...any) error { return r.err }

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

func TestWrite_RejectsInvalidWithoutQuery(t *testing.T) {
	for _, e := range []Entry{{Action: "x"}, {ActorID: 1}} {
		f := &fakeDB{}
		if err := NewWriter().Write(context.Background(), f, e); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("want ErrInvalidEntry, got %v", err)
		}
		if f.calls != 0 {
			t.Fatalf("query ran %d times", f.calls)
		}
	}
}

func TestWrite_PassesFields(t *testing.T) {
	f := &fakeDB{}
	err := NewWriter().Write(context.Background(), f, Entry{
		ActorID: 7, Action: "export", EntityType: "export_job", EntityID: 9,
		After: map[string]any{"rows": 3, "kind": "vendor"}, IP: "10.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 || len(f.args) != 7 {
		t.Fatalf("calls=%d args=%d", f.calls, len(f.args))
	}
	if f.args[0] != int64(7) || f.args[1] != "export" || f.args[2] != "export_job" || f.args[3] != int64(9) {
		t.Fatalf("args: %v", f.args)
	}
	if b, _ := f.args[4].([]byte); b != nil {
		t.Fatalf("before should be nil, got %s", b)
	}
	var got map[string]any
	if err := json.Unmarshal(f.args[5].([]byte), &got); err != nil || got["kind"] != "vendor" || got["rows"] != float64(3) {
		t.Fatalf("after: %v %v", got, err)
	}
	if ip, _ := f.args[6].(*string); ip == nil || *ip != "10.0.0.1" {
		t.Fatalf("ip: %v", f.args[6])
	}
}

func TestWrite_EmptyIPAndNilAfterAreNull(t *testing.T) {
	f := &fakeDB{}
	if err := NewWriter().Write(context.Background(), f, Entry{ActorID: 1, Action: "a"}); err != nil {
		t.Fatal(err)
	}
	if ip, _ := f.args[6].(*string); ip != nil {
		t.Fatalf("ip should be nil")
	}
	if b, _ := f.args[5].([]byte); b != nil {
		t.Fatalf("after should be nil")
	}
}

func TestWrite_WrapsDBError(t *testing.T) {
	boom := errors.New("boom")
	err := NewWriter().Write(context.Background(), &fakeDB{err: boom}, Entry{ActorID: 1, Action: "a"})
	if !errors.Is(err, boom) {
		t.Fatalf("want wrapped boom, got %v", err)
	}
}
