package extauth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType is the JOSE "typ" header of every Client_Token (RFC 9068).
const TokenType = "at+jwt"

// ErrInvalidToken wraps every reason Validate rejects a token; callers map it
// to 401 invalid_token and never echo the detail to the caller.
var ErrInvalidToken = errors.New("invalid client token")

// Claims is the payload of a Client_Token (Req 3.1).
type Claims struct {
	Scope string `json:"scope"`
	Ver   int32  `json:"ver"`
	jwt.RegisteredClaims
}

// Scopes returns the space-separated scope claim as a slice.
func (c *Claims) Scopes() []string { return strings.Fields(c.Scope) }

// TokenService issues and validates Client_Tokens with EXT_JWT_SECRET (never
// JWT_SECRET, Req 3.1/3.4).
type TokenService struct {
	key    []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenService expects key/issuer/ttl already checked by config.Load
// (key ≥ 32 bytes, ttl 1m–60m).
func NewTokenService(key []byte, issuer string, ttl time.Duration) *TokenService {
	return &TokenService{key: key, issuer: issuer, ttl: ttl, now: time.Now}
}

// Issue signs a token for client c with the given scopes. Every scope must be
// known and allowed for c; the caller (token handler) already narrowed them.
func (t *TokenService) Issue(c Client, scopes []string) (token string, expiresIn int, err error) {
	if c.ClientID == "" || c.CredentialVersion <= 0 {
		return "", 0, errors.New("issue token: client id and credential version required")
	}
	if len(scopes) == 0 {
		return "", 0, errors.New("issue token: no scope")
	}
	for _, s := range scopes {
		if !ValidScope(s) || !c.HasScope(s) {
			return "", 0, fmt.Errorf("issue token: scope %q not allowed", s)
		}
	}

	now := t.now()
	claims := Claims{
		Scope: strings.Join(scopes, " "),
		Ver:   c.CredentialVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   c.ClientID,
			Audience:  jwt.ClaimStrings{Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["typ"] = TokenType
	signed, err := tok.SignedString(t.key)
	if err != nil {
		return "", 0, fmt.Errorf("issue token: sign: %w", err)
	}
	return signed, int(t.ttl / time.Second), nil
}

// Validate accepts only an HS256 at+jwt token signed with this service's key,
// with the configured iss, aud crown-ext, exp, iat not in the future, sub, jti,
// ver > 0 and at least one scope, all known (Req 3.1, 3.3). A User_Token has no
// aud/typ/ver, so it fails here even if signed with the same key.
func (t *TokenService) Validate(token string) (*Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(tok *jwt.Token) (any, error) {
			if typ, _ := tok.Header["typ"].(string); typ != TokenType {
				return nil, fmt.Errorf("typ %q", typ)
			}
			return t.key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if claims.Subject == "" || claims.ID == "" || claims.Ver <= 0 || claims.IssuedAt == nil {
		return nil, fmt.Errorf("%w: missing sub, jti, iat or ver", ErrInvalidToken)
	}
	scopes := claims.Scopes()
	if len(scopes) == 0 {
		return nil, fmt.Errorf("%w: no scope", ErrInvalidToken)
	}
	for _, s := range scopes {
		if !ValidScope(s) {
			return nil, fmt.Errorf("%w: unknown scope", ErrInvalidToken)
		}
	}
	return &claims, nil
}
