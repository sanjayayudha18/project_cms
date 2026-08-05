/**
 * Generic filter utilities for mock data tables.
 * Used by Forecast View (priority filter) and CIT Tracker (status + vendor compound filter).
 */

/**
 * Filters an array of records by a single field value.
 * Returns all records unchanged when value is null (no filter applied).
 */
export function filterByField<T>(
  records: T[],
  field: keyof T,
  value: T[keyof T] | null,
): T[] {
  if (value === null) {
    return records;
  }
  return records.filter((record) => record[field] === value);
}

/**
 * Applies multiple filters simultaneously. Only non-null filter values are applied.
 * Returns records matching ALL active filter criteria (logical AND).
 *
 * Example:
 *   compoundFilter(orders, { status: 'Completed', vendorId: null })
 *   → filters by status only (vendorId is null so ignored)
 */
export function compoundFilter<T>(
  records: T[],
  filters: Partial<Record<keyof T, unknown>>,
): T[] {
  const activeFilters = Object.entries(filters).filter(
    ([, value]) => value !== null && value !== undefined,
  ) as [string, unknown][];

  if (activeFilters.length === 0) {
    return records;
  }

  return records.filter((record) =>
    activeFilters.every(
      ([key, value]) => record[key as keyof T] === value,
    ),
  );
}
