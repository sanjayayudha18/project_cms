import type { ReconciliationException } from './types';

/**
 * Combined AND filter for reconciliation exceptions.
 *
 * Severity filter:
 *   - "All severity" | "All" → no constraint
 *   - "High" → only severity === "high"
 *   - "Medium" → only severity === "medium"
 *
 * Exception type filter:
 *   - "Open exceptions" → only records where owner is null (unassigned)
 *   - "All records" | "All" → no constraint
 *   - "Resolved" → only records where owner is not null (assigned)
 */
export function filterExceptions(
  records: ReconciliationException[],
  severity: string,
  exceptionType: string,
): ReconciliationException[] {
  return records.filter((record) => {
    const passesSeverity = matchesSeverity(record, severity);
    const passesType = matchesExceptionType(record, exceptionType);
    return passesSeverity && passesType;
  });
}

function matchesSeverity(
  record: ReconciliationException,
  severity: string,
): boolean {
  const normalized = severity.toLowerCase();
  if (normalized === 'all severity' || normalized === 'all') return true;
  if (normalized === 'high') return record.severity === 'high';
  if (normalized === 'medium') return record.severity === 'medium';
  return true;
}

function matchesExceptionType(
  record: ReconciliationException,
  exceptionType: string,
): boolean {
  const normalized = exceptionType.toLowerCase();
  if (normalized === 'open exceptions') return record.owner === null;
  if (normalized === 'all records' || normalized === 'all') return true;
  if (normalized === 'resolved') return record.owner !== null;
  return true;
}
