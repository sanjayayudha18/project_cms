package auth

import "time"

// Local-password policy constants (auth_source=local|local_dev only; LDAP/Entra
// manage their own policy). See .kiro/specs/Auth-Local-Lifecycle/task.md Task 3.
const (
	PasswordMaxAgeDays = 90
	PasswordWarnDays   = 7
)

// PasswordExpiry evaluates the local-password expiry policy given when the
// password was last changed. changedAt=nil means the policy has never been
// evaluated for this user (e.g. a row that predates this column) and is
// treated as not expired, outside the warning window — daysLeft reports the
// full policy window since there is no age to measure against.
//
// daysLeft counts down from PasswordMaxAgeDays; it goes negative once expired.
// expired is true once the password is more than PasswordMaxAgeDays old
// (day 90 itself is still valid — expiry is exclusive of the boundary).
func PasswordExpiry(changedAt *time.Time, now time.Time) (expired bool, daysLeft int) {
	if changedAt == nil {
		return false, PasswordMaxAgeDays
	}

	ageDays := int(now.Sub(*changedAt) / (24 * time.Hour))
	daysLeft = PasswordMaxAgeDays - ageDays
	expired = daysLeft < 0
	return expired, daysLeft
}

// InPasswordWarningWindow reports whether daysLeft falls in the "expiring
// soon" window: not yet expired, but within PasswordWarnDays of expiring.
func InPasswordWarningWindow(daysLeft int) bool {
	return daysLeft >= 0 && daysLeft <= PasswordWarnDays
}
