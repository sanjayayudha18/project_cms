package auth

import "time"

// Local-password lockout policy constants (auth_source=local|local_dev only;
// distinct from the per-IP/username rate limiter in pkg/middleware — that
// throttles request volume, this locks one specific account after repeated
// bad credentials). See .kiro/specs/Auth-Local-Lifecycle/task.md Task 4.
const (
	MaxFailedLogins = 3
	LockoutDuration = 30 * time.Minute
)

// IsLocked reports whether an account is currently locked out, given its
// locked_until value. nil, or a timestamp that has already passed, means
// not locked — the lock auto-expires; nothing needs to actively clear it.
func IsLocked(lockedUntil *time.Time, now time.Time) bool {
	if lockedUntil == nil {
		return false
	}
	return now.Before(*lockedUntil)
}
