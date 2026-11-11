// Package extauth authenticates external API clients (OAuth2 client
// credentials): client records, secrets, tokens, rate limits, middleware.
package extauth

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/cimb-niaga/cms/pkg/userrules"
)

const (
	ScopeUsersRead  = "users:read"
	ScopeUsersWrite = "users:write"
	Audience        = "crown-ext"
)

// ValidScope reports whether s is a scope this API knows (exact match).
func ValidScope(s string) bool {
	return s == ScopeUsersRead || s == ScopeUsersWrite
}

// ParseScopes splits on spaces or commas, drops empties and duplicates (first
// occurrence keeps its order) and rejects any unknown scope. Empty input → nil.
func ParseScopes(s string) ([]string, error) {
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if !ValidScope(f) {
			return nil, fmt.Errorf("unknown scope %q", f)
		}
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out, nil
}

// Client is one api_clients row.
type Client struct {
	ID                int64
	ClientID          string
	Name              string
	SecretHash        string
	CredentialVersion int32
	Scopes            []string
	AssignableRoles   []string
	AllowedCIDRs      []netip.Prefix
	ServiceUserID     int64
	IsActive          bool
}

// AllowsIP reports whether ip is inside one of the client's CIDRs. An empty
// list allows nothing (Req 1.7).
func (c Client) AllowsIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, p := range c.AllowedCIDRs {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// CanAssign reports whether the client may give role to a user: never a
// privileged role, even if the allowlist somehow holds one.
func (c Client) CanAssign(role string) bool {
	return !userrules.IsPrivileged(role) && slices.Contains(c.AssignableRoles, role)
}

// HasScope reports whether the client is allowed scope s.
func (c Client) HasScope(s string) bool {
	return slices.Contains(c.Scopes, s)
}
