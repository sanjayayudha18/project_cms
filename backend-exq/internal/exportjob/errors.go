package exportjob

import "errors"

var (
	// ErrValidation marks bad request parameters (HTTP 400).
	ErrValidation = errors.New("exportjob: invalid parameters")
	// ErrUnknownEntity marks an entity that does not exist for the kind (HTTP 404).
	ErrUnknownEntity = errors.New("exportjob: unknown entity")
	// ErrUnauthenticated marks an actor without a user id (HTTP 401, Req 8.7).
	ErrUnauthenticated = errors.New("exportjob: unauthenticated")
	// ErrForbidden marks a role not allowed for the export kind (HTTP 403, Req 7.2, 7.3, 7.6).
	ErrForbidden = errors.New("exportjob: forbidden")
	// ErrNotFound marks a missing vendor / cabang / job; one error for every case (HTTP 404, Req 2.4, 3.2, 7.5).
	ErrNotFound = errors.New("exportjob: not found")
	// ErrTooManyActive marks a user already at MaxActivePerUser queued+running jobs (HTTP 429, Req 9.5).
	ErrTooManyActive = errors.New("exportjob: too many active exports")
	// ErrNotReady marks a download of a job that is not succeeded yet (HTTP 409, Req 9.11).
	ErrNotReady = errors.New("exportjob: export not ready")
	// ErrGone marks a download of a job already downloaded or expired (HTTP 410, Req 9.11).
	ErrGone = errors.New("exportjob: export gone")
	// ErrNotDownloading marks a Complete/Abort of a job no longer downloading
	// (janitor recovered it); nothing changed. Logged only, never sent to the client.
	ErrNotDownloading = errors.New("exportjob: job not downloading")
	// ErrInternal wraps DB and audit failures (HTTP 500). The cause stays in the chain for logs only.
	ErrInternal = errors.New("exportjob: internal error")
)
