package extauth

import (
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestNewClientID(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		id, err := NewClientID()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidClientIDFormat(id) {
			t.Fatalf("NewClientID() = %q does not match format", id)
		}
		if seen[id] {
			t.Fatalf("duplicate client id %q", id)
		}
		seen[id] = true
	}
}

func TestNewSecret(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		s, err := NewSecret()
		if err != nil {
			t.Fatal(err)
		}
		if len(s) != 43 {
			t.Fatalf("len = %d, want 43", len(s))
		}
		raw, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil || len(raw) != 32 {
			t.Fatalf("secret %q is not 32 bytes base64url: %v", s, err)
		}
		if seen[s] {
			t.Fatal("duplicate secret")
		}
		seen[s] = true
	}
}

func TestHashAndVerifySecret(t *testing.T) {
	s, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	h, err := HashSecret(s)
	if err != nil {
		t.Fatal(err)
	}
	if cost, err := bcrypt.Cost([]byte(h)); err != nil || cost != 12 {
		t.Fatalf("bcrypt cost = %d (%v), want 12", cost, err)
	}
	if strings.Contains(h, s) {
		t.Fatal("hash contains the secret")
	}
	if !VerifySecret(h, s) {
		t.Error("correct secret rejected")
	}
	if VerifySecret(h, s+"x") || VerifySecret(h, "") {
		t.Error("wrong secret accepted")
	}
	if VerifySecret("not-a-hash", s) {
		t.Error("malformed hash accepted")
	}
	// Unknown client: empty hash still does a bcrypt compare (dummy) and is always false.
	if VerifySecret("", s) || VerifySecret("", "") {
		t.Error("empty hash accepted")
	}
}

func TestDummyHashCost(t *testing.T) {
	if cost, err := bcrypt.Cost([]byte(dummyHash())); err != nil || cost != bcryptCost {
		t.Fatalf("dummy hash cost = %d (%v), want %d", cost, err, bcryptCost)
	}
	if a, b := dummyHash(), dummyHash(); a != b {
		t.Error("dummy hash must be computed once")
	}
}

func TestHashSecretTooLong(t *testing.T) {
	if _, err := HashSecret(strings.Repeat("a", 73)); err == nil {
		t.Error("expected error for secret over 72 bytes")
	}
}

func TestValidClientIDFormat(t *testing.T) {
	ok := "crown_" + strings.Repeat("a", 26)
	for s, want := range map[string]bool{
		ok:                                 true,
		"crown_" + strings.Repeat("2", 26): true,
		"crown_" + strings.Repeat("a", 25): false,
		"crown_" + strings.Repeat("a", 27): false,
		"crown_" + strings.Repeat("A", 26): false,
		"crown_" + strings.Repeat("1", 26): false, // 0,1,8,9 not in base32
		"CROWN_" + strings.Repeat("a", 26): false,
		"x" + ok:                           false,
		ok + "\n":                          false,
		"":                                 false,
		"crown_aaaaaaaaaaaaa:aaaaaaaaaaaa": false,
	} {
		if got := ValidClientIDFormat(s); got != want {
			t.Errorf("ValidClientIDFormat(%q) = %v, want %v", s, got, want)
		}
	}
}
