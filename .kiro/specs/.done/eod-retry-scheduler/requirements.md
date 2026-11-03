# Requirements Document

## Introduction

The EOD Retry Scheduler is a standalone Python FastAPI microservice that detects failed and late ETL file processing for the CMS ATM & CIT system's End-of-Day batch pipeline. It provides filesystem-based detection of unprocessed files, automated and manual retry capabilities by re-running idempotent ETL scripts, and a REST API consumed by the Go backend and the frontend EOD Monitoring page.

The service handles three ETL pipelines: DMAA, ITM Cash Position, and ITM Replenishment. Each pipeline has configurable SLA deadlines. Files that miss their deadline are flagged as late; files that end up in `not_processed/` directories or remain in input after processing errors are flagged as failed.

## Glossary

- **Retry_Scheduler**: The Python FastAPI microservice responsible for detecting failed/late files and orchestrating retries
- **ETL_Script**: One of the three existing ETL processors — `dmaa_etl.py`, `itm_cashpos_etl.py`, or `itm_replenish_etl.py`
- **File_Type**: Classification of an inbound file — one of `dmaa`, `itm_cashpos`, or `itm_replenish`
- **SLA_Deadline**: The configurable time-of-day by which a file of a given File_Type must have arrived and been successfully processed
- **Late_File**: A file that has not arrived in the input directory or has not been successfully processed before the SLA_Deadline for its File_Type
- **Failed_File**: A file found in the `not_processed/` directory or remaining in the input directory after a processing error
- **Input_Directory**: The FTP landing directory for inbound files — `FTP_DATA/DMAA/` or `FTP_DATA/ITM/`
- **Not_Processed_Directory**: The error directory where ETL scripts move files that could not be processed
- **File_Checksum**: SHA-256 hash of a file's contents, used as the idempotency key for processing
- **Retry_Attempt**: A single execution of the appropriate ETL_Script against a Failed_File or Late_File
- **Processing_Status**: The state of a file in the DB — one of `pending`, `processing`, `completed`, or `failed`
- **Go_Backend**: The main CMS Go backend (`cmd/api`) that consumes the Retry_Scheduler REST API
- **EOD_Monitoring_Page**: The frontend admin page that displays EOD run status, retry history, and file processing state

## Requirements

### Requirement 1: File Failure Detection

**User Story:** As an operations support engineer, I want the system to automatically detect files that failed ETL processing, so that I can see which files need retry without manually checking directories.

#### Acceptance Criteria

1. WHEN the Retry_Scheduler performs a detection scan, THE Retry_Scheduler SHALL check the Not_Processed_Directory for each File_Type for files that have been moved there by the ETL_Script.
2. WHEN the Retry_Scheduler finds a file in the Not_Processed_Directory, THE Retry_Scheduler SHALL mark that file as `failed` in the corresponding database table using the File_Checksum as the idempotency key.
3. WHEN the Retry_Scheduler performs a detection scan, THE Retry_Scheduler SHALL check the Input_Directory for each File_Type for files that remain after the ETL_Script has completed its run.
4. WHEN the Retry_Scheduler finds a file remaining in the Input_Directory after the ETL_Script run window has closed, THE Retry_Scheduler SHALL mark that file as `failed` in the corresponding database table.
5. THE Retry_Scheduler SHALL record the file path, File_Checksum, File_Type, detection timestamp, and failure reason for each detected Failed_File.

### Requirement 2: Late File Detection

**User Story:** As an operations support engineer, I want the system to detect files that have not arrived or completed processing before the SLA deadline, so that I am alerted to potential upstream issues.

#### Acceptance Criteria

1. WHEN the current time passes the SLA_Deadline for a given File_Type AND no file with Processing_Status `completed` exists for the current processing_date, THE Retry_Scheduler SHALL flag the File_Type as late for that processing_date.
2. THE Retry_Scheduler SHALL store a late detection record containing the File_Type, processing_date, SLA_Deadline, and the actual detection timestamp.
3. WHILE a File_Type is flagged as late for a processing_date, THE Retry_Scheduler SHALL continue to report the late status until a file for that File_Type achieves Processing_Status `completed`.
4. WHEN a late File_Type subsequently achieves Processing_Status `completed`, THE Retry_Scheduler SHALL update the late detection record with the completion timestamp and mark the late status as resolved.

### Requirement 3: Configurable SLA Deadlines

**User Story:** As an admin, I want to configure the SLA deadlines per file type, so that the system adapts when business SLAs change without code deployment.

#### Acceptance Criteria

1. THE Retry_Scheduler SHALL support per-File_Type SLA_Deadline configuration via environment variables or a configuration file.
2. THE Retry_Scheduler SHALL use the following default SLA_Deadlines: DMAA before 06:00 WIB, ITM Cash Position before 07:00 WIB, ITM Replenishment before 07:00 WIB.
3. WHEN a SLA_Deadline configuration value is not a valid time format, THE Retry_Scheduler SHALL reject the configuration at startup and log the validation error.
4. THE Retry_Scheduler SHALL evaluate all SLA_Deadlines in Asia/Jakarta timezone (WIB).

### Requirement 4: Automated Retry Execution

**User Story:** As an operations support engineer, I want the system to automatically retry failed files on a configurable schedule, so that transient failures recover without manual intervention.

#### Acceptance Criteria

1. THE Retry_Scheduler SHALL execute automatic retries for Failed_Files on a configurable interval (default: every 30 minutes after initial failure detection).
2. WHEN the Retry_Scheduler initiates a Retry_Attempt, THE Retry_Scheduler SHALL invoke the appropriate ETL_Script for the File_Type of the Failed_File.
3. THE Retry_Scheduler SHALL enforce a configurable maximum number of automatic Retry_Attempts per file (default: 3).
4. WHEN a file has reached the maximum number of automatic Retry_Attempts AND Processing_Status is still `failed`, THE Retry_Scheduler SHALL mark the file as `max_retries_exhausted` and cease automatic retries.
5. WHEN a Retry_Attempt results in Processing_Status `completed`, THE Retry_Scheduler SHALL update the file record to `completed` and cease further retries for that file.
6. THE Retry_Scheduler SHALL ensure idempotency of retries by using the File_Checksum — re-running the ETL_Script against the same file content produces no duplicate data.

### Requirement 5: Manual Retry via REST API

**User Story:** As an operations support engineer, I want to manually trigger a retry for a specific failed file from the EOD Monitoring page, so that I can intervene when automated retries are exhausted or I want immediate action.

#### Acceptance Criteria

1. WHEN the Go_Backend sends a POST request to the manual retry endpoint with a valid file identifier, THE Retry_Scheduler SHALL queue a Retry_Attempt for the specified file.
2. WHEN a manual Retry_Attempt is requested for a file with Processing_Status `completed`, THE Retry_Scheduler SHALL reject the request with an appropriate error response.
3. WHEN a manual Retry_Attempt is requested, THE Retry_Scheduler SHALL bypass the maximum automatic retry count and execute the retry regardless of previous attempt count.
4. THE Retry_Scheduler SHALL return the retry job identifier and current Processing_Status in the response to a manual retry request.

### Requirement 6: REST API for Status and History

**User Story:** As an operations support engineer, I want a REST API that exposes the current status of all file processing and retry history, so that the EOD Monitoring page can display real-time information.

#### Acceptance Criteria

1. THE Retry_Scheduler SHALL expose a GET endpoint that returns the current Processing_Status of all files for a given processing_date, grouped by File_Type.
2. THE Retry_Scheduler SHALL expose a GET endpoint that returns the retry history for a specific file, including each Retry_Attempt timestamp, outcome, and error detail.
3. THE Retry_Scheduler SHALL expose a GET endpoint that returns all late detection records for a given processing_date.
4. THE Retry_Scheduler SHALL expose a GET endpoint that returns a summary of failed, late, completed, and pending file counts for a given processing_date.
5. THE Retry_Scheduler SHALL return all API responses in a consistent JSON envelope containing `status`, `data`, and `error` fields.
6. WHEN a request references a processing_date with no records, THE Retry_Scheduler SHALL return an empty data set with HTTP 200, not an error.

### Requirement 7: Audit Logging

**User Story:** As a compliance officer, I want every retry action to be audit-logged, so that there is a complete trail of who triggered what and when for regulatory review.

#### Acceptance Criteria

1. WHEN a Retry_Attempt is initiated (automatic or manual), THE Retry_Scheduler SHALL write an audit log entry containing: trigger type (auto/manual), file identifier, File_Type, File_Checksum, processing_date, initiated_by (system or user ID), and timestamp.
2. WHEN a Retry_Attempt completes, THE Retry_Scheduler SHALL write an audit log entry containing: file identifier, outcome (completed/failed), duration, and error detail if failed.
3. THE Retry_Scheduler SHALL store audit log entries in a dedicated `retry_audit_logs` database table.
4. THE Retry_Scheduler SHALL expose a GET endpoint that returns audit log entries filtered by processing_date, File_Type, or trigger type.
5. THE Retry_Scheduler SHALL never delete or modify existing audit log entries — audit records are append-only.

### Requirement 8: Detection Scheduling

**User Story:** As an operations support engineer, I want the detection scan to run on a configurable schedule, so that the system checks for failures and late files at appropriate intervals.

#### Acceptance Criteria

1. THE Retry_Scheduler SHALL run the file failure detection scan on a configurable cron-like schedule (default: every 15 minutes from 05:00 to 09:00 WIB).
2. THE Retry_Scheduler SHALL run the late file detection check immediately after each SLA_Deadline passes for the current processing_date.
3. WHEN a detection scan is already in progress AND a new scan is triggered, THE Retry_Scheduler SHALL skip the new scan and log a warning rather than running concurrent scans.
4. THE Retry_Scheduler SHALL log the start time, end time, and number of files detected for each scan execution.

### Requirement 9: Health and Connectivity

**User Story:** As a DevOps engineer, I want the service to expose health check endpoints, so that infrastructure monitoring can detect when the service is unhealthy.

#### Acceptance Criteria

1. THE Retry_Scheduler SHALL expose a GET `/health` endpoint that returns HTTP 200 when the service is running and can connect to the database.
2. IF the database connection fails during a health check, THEN THE Retry_Scheduler SHALL return HTTP 503 with a diagnostic message indicating database connectivity failure.
3. IF the configured Input_Directory or Not_Processed_Directory is not accessible, THEN THE Retry_Scheduler SHALL return HTTP 503 with a diagnostic message indicating filesystem access failure.
4. THE Retry_Scheduler SHALL include the service version, uptime, and last successful scan timestamp in the health check response.

### Requirement 10: Error Handling and Resilience

**User Story:** As an operations support engineer, I want the service to handle errors gracefully without crashing, so that partial failures do not prevent other files from being retried.

#### Acceptance Criteria

1. IF an ETL_Script invocation fails during a Retry_Attempt, THEN THE Retry_Scheduler SHALL capture the error output, update the file Processing_Status to `failed`, increment the retry count, and continue processing remaining files.
2. IF the filesystem becomes inaccessible during a detection scan, THEN THE Retry_Scheduler SHALL log the error, mark the scan as failed, and attempt the next scheduled scan normally.
3. IF a database write fails during retry processing, THEN THE Retry_Scheduler SHALL retry the database operation up to 3 times with exponential backoff before marking the operation as failed.
4. THE Retry_Scheduler SHALL isolate failures per file — a failure processing one file shall not prevent retry of other files in the same scan cycle.
5. WHEN an unrecoverable error occurs, THE Retry_Scheduler SHALL log the full error context including file path, File_Type, processing_date, and stack trace.

### Requirement 11: Security and Access Control

**User Story:** As a security engineer, I want the microservice endpoints to be protected, so that only authorized systems can trigger retries or access file status.

#### Acceptance Criteria

1. THE Retry_Scheduler SHALL require a valid API key or JWT token on all endpoints except `/health`.
2. WHEN a request is received without valid authentication credentials, THE Retry_Scheduler SHALL return HTTP 401 with an error message.
3. WHEN a manual retry is triggered, THE Retry_Scheduler SHALL extract the user identity from the authentication token and include the user ID in the audit log entry.
4. THE Retry_Scheduler SHALL never log or expose authentication tokens, database credentials, or file contents in API responses or log output.
