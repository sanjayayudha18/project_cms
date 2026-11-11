import { test, expect } from '@playwright/test';

/**
 * E2E: 404 handling
 * Unknown path shows NotFound with link to /dashboard.
 * @validates Requirements 9.3
 */
test.describe('404 Not Found handling', () => {
  test('unknown route shows NotFound page', async ({ page }) => {
    await page.goto('/some-nonexistent-page');

    await expect(page.getByText('Page not found')).toBeVisible();
  });

  test('NotFound page has a link back to /dashboard', async ({ page }) => {
    await page.goto('/unknown-route');

    const link = page.getByRole('link', { name: /dashboard/i });
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute('href', '/dashboard');
  });

  test('clicking the link navigates to Dashboard', async ({ page }) => {
    await page.goto('/does-not-exist');

    const link = page.getByRole('link', { name: /dashboard/i });
    await link.click();

    await expect(page).toHaveURL(/\/dashboard$/);
  });
});
