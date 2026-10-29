package exportjob

import "time"

// Export limits (design.md "Batas waktu dan konstanta").
const (
	JobTimeout       = 10 * time.Minute // Req 9.9: max run time of one job
	DownloadLease    = 5 * time.Minute  // Req 10.5: lease while a download streams
	Retention        = 10 * time.Minute // Req 10.6 (D18c): file kept this long after finish
	MaxBytes         = 100 << 20        // Req 10.11: max file size (100 MiB)
	MaxRows          = 1_000_000        // Req 10.11: max data rows per file
	ChunkSize        = 1 << 20          // Req 6.4, 10.1: size of one stored chunk
	PageSize         = 1000             // Req 6.1: keyset page size of producers (same as the old export)
	MaxActivePerUser = 3                // Req 9.5 (D21): queued+running jobs per user
)
