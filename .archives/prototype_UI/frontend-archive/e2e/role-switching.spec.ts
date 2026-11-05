import { test, expect } from '@playwright/test';

/**
 * E2E: Navigation visibility
 * All navigation items are visible to all users (role-based filtering removed).
 * The RoleSwitcher component has been replaced by a static profile section.
 * @validates Requirements 1.9
 */
test.describe('Navigation visibility', () => {
  test('all navigation items are visible on the sidebar', async ({ page }) => {
    await page.goto('/dashboard');

    // All nav items should be visible (no role-based filtering)
    await expect(page.locator('nav a[href="/dashboard"]')).toBeVisible();
    await expect(page.locator('nav a[href="/replenishment"]')).toBeVisible();
    await expect(page.locator('nav a[href="/cash-count"]')).toBeVisible();
    await expect(page.locator('nav a[href="/reconciliation"]')).toBeVisible();
    await expect(page.locator('nav a[href="/invoices"]')).toBeVisible();
    await expect(page.locator('nav a[href="/reports"]')).toBeVisible();
    await expect(page.locator('nav a[href="/forecast"]')).toBeVisible();
    await expect(page.locator('nav a[href="/settings"]')).toBeVisible();
  });

  test('no role switcher select element exists', async ({ page }) => {
    await page.goto('/dashboard');

    // The RoleSwitcher select should no longer be present
    await expect(page.locator('select')).toHaveCount(0);
  });
});
