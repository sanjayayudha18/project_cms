package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// mockUserRepository is a test double for UserRepository.
type mockUserRepository struct {
	user *UserRecord
	err  error
}

func (m *mockUserRepository) FindByUsername(_ context.Context, _ string) (*UserRecord, error) {
	return m.user, m.err
}

func (m *mockUserRepository) UpdateLastLogin(_ context.Context, _ int64) error {
	return nil
}

func (m *mockUserRepository) GetUserProfile(_ context.Context, _ int64) (*UserRecord, error) {
	return m.user, m.err
}

// hashPassword is a test helper that generates a bcrypt hash.
func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

func TestLocalProvider_Authenticate_ValidPassword(t *testing.T) {
	password := "Password123!"
	hash := hashPassword(t, password)
	vendorID := int64(42)

	repo := &mockUserRepository{
		user: &UserRecord{
			ID:           1,
			Username:     "john.admin",
			FullName:     "John Admin",
			Email:        "john@example.com",
			PasswordHash: &hash,
			AuthSource:   "local_dev",
			RoleID:       1,
			Role:         "ADMIN",
			IsKaryawan:   true,
			VendorID:     &vendorID,
			IsActive:     true,
		},
	}

	provider := NewLocalProvider(repo)
	identity, err := provider.Authenticate(context.Background(), "john.admin", password)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if identity == nil {
		t.Fatal("expected identity, got nil")
	}
	if identity.UserID != 1 {
		t.Errorf("expected UserID=1, got %d", identity.UserID)
	}
	if identity.Username != "john.admin" {
		t.Errorf("expected Username=john.admin, got %s", identity.Username)
	}
	if identity.Role != "ADMIN" {
		t.Errorf("expected Role=ADMIN, got %s", identity.Role)
	}
	if !identity.IsKaryawan {
		t.Error("expected IsKaryawan=true")
	}
	if identity.VendorID == nil || *identity.VendorID != 42 {
		t.Errorf("expected VendorID=42, got %v", identity.VendorID)
	}
}

func TestLocalProvider_Authenticate_WrongPassword(t *testing.T) {
	hash := hashPassword(t, "CorrectPassword!")

	repo := &mockUserRepository{
		user: &UserRecord{
			ID:           1,
			Username:     "john.admin",
			PasswordHash: &hash,
			AuthSource:   "local_dev",
			Role:         "ADMIN",
			IsKaryawan:   true,
			IsActive:     true,
		},
	}

	provider := NewLocalProvider(repo)
	identity, err := provider.Authenticate(context.Background(), "john.admin", "WrongPassword!")

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity on wrong password")
	}
}

func TestLocalProvider_Authenticate_UserNotFound(t *testing.T) {
	// repo returns nil, nil — user not found
	repo := &mockUserRepository{
		user: nil,
		err:  nil,
	}

	provider := NewLocalProvider(repo)
	identity, err := provider.Authenticate(context.Background(), "nonexistent", "AnyPassword1")

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity when user not found")
	}
}

func TestLocalProvider_Authenticate_RepoError(t *testing.T) {
	repoErr := errors.New("database connection lost")
	repo := &mockUserRepository{
		user: nil,
		err:  repoErr,
	}

	provider := NewLocalProvider(repo)
	identity, err := provider.Authenticate(context.Background(), "john.admin", "Password123!")

	if !errors.Is(err, repoErr) {
		t.Errorf("expected repo error, got: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity on repo error")
	}
}

func TestLocalProvider_Authenticate_NilPasswordHash(t *testing.T) {
	// LDAP user with nil password_hash
	repo := &mockUserRepository{
		user: &UserRecord{
			ID:           2,
			Username:     "ldap.user",
			PasswordHash: nil,
			AuthSource:   "ldap",
			Role:         "ATM-USER",
			IsKaryawan:   true,
			IsActive:     true,
		},
	}

	provider := NewLocalProvider(repo)
	identity, err := provider.Authenticate(context.Background(), "ldap.user", "AnyPassword1")

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity when password_hash is nil")
	}
}

func TestLocalProvider_Supports(t *testing.T) {
	provider := NewLocalProvider(nil)

	tests := []struct {
		authSource string
		expected   bool
	}{
		{"local", true},
		{"local_dev", true},
		{"ldap", false},
		{"", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.authSource, func(t *testing.T) {
			got := provider.Supports(tt.authSource)
			if got != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.authSource, got, tt.expected)
			}
		})
	}
}
