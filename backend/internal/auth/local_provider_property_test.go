package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
	"pgregory.net/rapid"
)

// printableASCIIRune returns a generator for printable ASCII runes (0x20–0x7E).
func printableASCIIRune() *rapid.Generator[rune] {
	return rapid.Map(rapid.Int32Range(0x20, 0x7E), func(i int32) rune {
		return rune(i)
	})
}

// Feature: user-login, Property 1: Bcrypt Round-Trip Verification
// **Validates: Requirements 2.2, 6.1, 6.3, 6.4**
//
// For any password string between 8 and 72 bytes, hashing it with bcrypt and then
// comparing the original password against the resulting hash SHALL succeed, and
// comparing any different password against the same hash SHALL fail.
//
// Note: We use bcrypt cost 4 (bcrypt.MinCost) in this property test for performance,
// since rapid runs many iterations. The BcryptCost constant (12) is used in production.
// The round-trip correctness property is independent of the cost factor.
func TestProperty_BcryptRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random password between 8 and 72 runes (printable ASCII)
		password := rapid.StringOfN(printableASCIIRune(), 8, 72, -1).Draw(t, "password")

		// Hash it with min cost for test performance
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("bcrypt hash failed: %v", err)
		}

		// Verify original password matches the hash
		err = bcrypt.CompareHashAndPassword(hash, []byte(password))
		if err != nil {
			t.Fatalf("original password should match hash, got: %v", err)
		}

		// Generate a different password and verify it does NOT match
		otherPassword := rapid.StringOfN(printableASCIIRune(), 8, 72, -1).Draw(t, "other_password")
		if otherPassword == password {
			return // skip if identical (astronomically unlikely but valid)
		}

		err = bcrypt.CompareHashAndPassword(hash, []byte(otherPassword))
		if err == nil {
			t.Fatalf("different password %q should NOT match hash of %q", otherPassword, password)
		}
	})
}
