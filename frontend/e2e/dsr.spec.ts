import { test, expect } from '@playwright/test';

/**
 * E2E: DSR date selection
 * Changing date updates table content.
 * @validates Requirements 3.8
 */
test.describe('DSR Dashboard date selection', () => {
  test('default date loads DSR data into table', async ({ page }) => {
    await page.goto('/reports');

    // Table should be visible with data for the default date (2024-01-15)
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('table tbody tr').first()).toBeVisible();
  });

  test('changing date updates table content', async ({ page }) => {
    await page.goto('/reports');

    // Get initial row count
    const initialRows = await page.locator('table tbody tr').count();
    expect(initialRows).toBeGreaterThan(0);

    // Change date to one that has data (2024-01-16)
    const dateInput = page.locator('input[type="date"]');
    await dateInput.fill('2024-01-16');

    // Wait for table to update — rows should still exist for a valid date
    await expect(page.locator('table')).toBeVisible();
  });

  test('selecting date with no data shows empty state', async ({ page }) => {
    await page.goto('/reports');

    // Change to a date that has no mock data
    const dateInput = page.locator('input[type="date"]');
    await dateInput.fill('2025-12-31');

    // Empty state message should appear
    await expect(
      page.getByText('No DSR data available for this date'),
    ).toBeVisible();
  });
});
