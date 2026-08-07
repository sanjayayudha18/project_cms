package auth

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the bcrypt cost factor used for hashing passwords.
const BcryptCost = 12

// LocalProvider verifies credentials against bcrypt password_hash in DB.
// Supports auth_source values: "local", "local_dev"
type LocalProvider struct {
	repo UserRepository
}

// NewLocalProvider creates a new LocalProvider with the given repository.
func NewLocalProvider(repo UserRepository) *LocalProvider {
	return &LocalProvider{repo: repo}
}

// Supports returns true if this provider handles the given auth source.
func (p *LocalProvider) Supports(authSource string) bool {
	return authSource == "local" || authSource == "local_dev"
}

// Authenticate verifies username/password against bcrypt hash from the database.
// Returns AuthIdentity on success or ErrInvalidCredentials on failure.
func (p *LocalProvider) Authenticate(ctx context.Context, username, password string) (*AuthIdentity, error) {
	user, err := p.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if user.PasswordHash == nil {
		return nil, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return &AuthIdentity{
		UserID:     user.ID,
		Username:   user.Username,
		Role:       user.Role,
		IsKaryawan: user.IsKaryawan,
		VendorID:   user.VendorID,
	}, nil
}
