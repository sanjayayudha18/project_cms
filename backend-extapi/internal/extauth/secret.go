package extauth

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost     = 12
	clientIDPrefix = "crown_"
)

var (
	clientIDRe = regexp.MustCompile(`^crown_[a-z2-7]{26}$`)
	b32        = base32.StdEncoding.WithPadding(base32.NoPadding)

	// dummyHash is compared against when the client is unknown, so the
	// response time does not tell whether a client_id exists (Req 2.4).
	dummyHash = sync.OnceValue(func() string {
		s, err := NewSecret()
		if err != nil {
			panic(fmt.Sprintf("extauth: dummy secret: %v", err))
		}
		h, err := HashSecret(s)
		if err != nil {
			panic(fmt.Sprintf("extauth: dummy hash: %v", err))
		}
		return h
	})
)

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("reading crypto/rand: %w", err)
	}
	return b, nil
}

// NewClientID returns "crown_" + 16 random bytes in lowercase unpadded base32.
func NewClientID() (string, error) {
	b, err := randomBytes(16)
	if err != nil {
		return "", err
	}
	return clientIDPrefix + strings.ToLower(b32.EncodeToString(b)), nil
}

// NewSecret returns 32 random bytes in unpadded base64url (43 chars).
func NewSecret() (string, error) {
	b, err := randomBytes(32)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashSecret hashes a client secret with bcrypt cost 12.
func HashSecret(s string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(s), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing secret: %w", err)
	}
	return string(h), nil
}

// VerifySecret compares s to hash in constant time. An empty hash (unknown
// client) is compared against the dummy hash and always returns false.
func VerifySecret(hash, s string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash()), []byte(s))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(s)) == nil
}

// ValidClientIDFormat checks ^crown_[a-z2-7]{26}$ before the value reaches a
// Redis key or a query.
func ValidClientIDFormat(s string) bool {
	return clientIDRe.MatchString(s)
}
