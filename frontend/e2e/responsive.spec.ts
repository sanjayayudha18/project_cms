import { test, expect } from '@playwright/test';

/**
 * E2E: Responsive sidebar
 * Viewport < 1024px collapses sidebar to icon-only.
 * @validates Requirements 10.1
 */
test.describe('Responsive sidebar', () => {
  test('sidebar collapses to icon-only rail on narrow viewport', async ({ page }) => {
    // Start with a wide viewport
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/dashboard');

    // Sidebar should be expanded (256px width)
    const sidebar = page.locator('aside');
    await expect(sidebar).toBeVisible();

    // Nav link labels should be visible when expanded
    const navLabel = sidebar.locator('nav a span').first();
    await expect(navLabel).toBeVisible();

    // Shrink viewport below 1024px
    await page.setViewportSize({ width: 768, height: 1024 });

    // Wait for sidebar to collapse — labels should become hidden (opacity: 0)
    await expect(navLabel).toHaveCSS('opacity', '0');
  });

  test('sidebar width narrows on small viewport', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto('/dashboard');

    const sidebar = page.locator('aside');
    const box = await sidebar.boundingBox();

    // Collapsed sidebar should be ~64px wide
    expect(box).not.toBeNull();
    expect(box!.width).toBeLessThanOrEqual(72);
  });

  test('sidebar expands back when user toggles', async ({ page }) => {
    // Start narrow
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto('/dashboard');

    const sidebar = page.locator('aside');
    const expandButton = page.getByLabel('Expand sidebar');

    // Click expand toggle
    await expandButton.click();

    // Should expand to full width
    const box = await sidebar.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.width).toBeGreaterThanOrEqual(200);
  });
});
