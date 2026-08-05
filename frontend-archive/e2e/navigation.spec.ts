import { test, expect } from '@playwright/test';

/**
 * E2E: Navigation flow
 * Sidebar links route correctly without full page reload.
 * @validates Requirements 1.5, 1.9
 */
test.describe('Navigation flow', () => {
  test('sidebar links navigate to correct routes', async ({ page }) => {
    await page.goto('/');

    // Default redirect should land on /dashboard
    await expect(page).toHaveURL(/\/dashboard$/);

    // Navigate to Reports (formerly /dsr)
    await page.locator('nav a[href="/reports"]').click();
    await expect(page).toHaveURL(/\/reports$/);

    // Navigate to Replenishment (formerly /cit)
    await page.locator('nav a[href="/replenishment"]').click();
    await expect(page).toHaveURL(/\/replenishment$/);

    // Navigate to Invoices
    await page.locator('nav a[href="/invoices"]').click();
    await expect(page).toHaveURL(/\/invoices$/);

    // Navigate to Forecast View
    await page.locator('nav a[href="/forecast"]').click();
    await expect(page).toHaveURL(/\/forecast$/);
  });

  test('navigation does not trigger a full page reload', async ({ page }) => {
    await page.goto('/dashboard');

    // Set a marker in window to detect full reload
    await page.evaluate(() => {
      (window as unknown as Record<string, boolean>).__nav_marker = true;
    });

    // Navigate to another page via sidebar
    await page.locator('nav a[href="/reports"]').click();
    await expect(page).toHaveURL(/\/reports$/);

    // Marker should still exist if no full reload occurred
    const markerExists = await page.evaluate(
      () => (window as unknown as Record<string, boolean>).__nav_marker === true,
    );
    expect(markerExists).toBe(true);
  });
});
