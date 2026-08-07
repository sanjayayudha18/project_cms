//go:build tools

package tools

// This file ensures all required dependencies remain in go.mod
// as direct dependencies. It is excluded from production builds
// via the "tools" build tag.

import (
	_ "github.com/go-chi/chi/v5"
	_ "github.com/go-playground/validator/v10"
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/google/uuid"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/redis/go-redis/v9"
	_ "golang.org/x/crypto/bcrypt"
	_ "pgregory.net/rapid"
)
