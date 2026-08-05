import { test, expect } from '@playwright/test';

/**
 * E2E: Invoice approval
 * Manager + Validated invoice → Approve → status updates.
 * @validates Requirements 6.7
 */
test.describe('Invoice approval flow', () => {
  test('Manager can approve a Validated invoice', async ({ page }) => {
    await page.goto('/invoice');

    // Switch role to Manager
    const roleSelect = page.locator('select').first();
    await roleSelect.selectOption('Manager');

    // Click on a Validated invoice row (INV-2024-002 is "Validated")
    const validatedRow = page.locator('tr', { hasText: 'INV-2024-002' });
    await validatedRow.click();

    // Approve button should be visible
    const approveButton = page.getByRole('button', { name: /approve/i });
    await expect(approveButton).toBeVisible();

    // Click Approve
    await approveButton.click();

    // Confirmation message should appear
    await expect(
      page.getByText(/INV-2024-002.*approved/i),
    ).toBeVisible();
  });

  test('Approve button not visible for non-Manager roles', async ({ page }) => {
    await page.goto('/invoice');

    // Default is Admin — click on Validated invoice
    const validatedRow = page.locator('tr', { hasText: 'INV-2024-002' });
    await validatedRow.click();

    // Approve button should NOT be visible for Admin
    const approveButton = page.getByRole('button', { name: /approve/i });
    await expect(approveButton).toBeHidden();
  });

  test('Approve button not visible for non-Validated invoices', async ({ page }) => {
    await page.goto('/invoice');

    // Switch to Manager
    const roleSelect = page.locator('select').first();
    await roleSelect.selectOption('Manager');

    // Click on an Uploaded invoice (INV-2024-005)
    const uploadedRow = page.locator('tr', { hasText: 'INV-2024-005' });
    await uploadedRow.click();

    // Approve button should NOT be visible
    const approveButton = page.getByRole('button', { name: /approve/i });
    await expect(approveButton).toBeHidden();
  });

  test('approval updates invoice status in the list', async ({ page }) => {
    await page.goto('/invoice');

    // Switch to Manager
    const roleSelect = page.locator('select').first();
    await roleSelect.selectOption('Manager');

    // Click on Validated invoice INV-2024-007
    const validatedRow = page.locator('tr', { hasText: 'INV-2024-007' });
    await validatedRow.click();

    // Approve it
    const approveButton = page.getByRole('button', { name: /approve/i });
    await approveButton.click();

    // The invoice list should now show "Approved" badge for this invoice
    const updatedRow = page.locator('tr', { hasText: 'INV-2024-007' });
    await expect(updatedRow.getByText('Approved')).toBeVisible();
  });
});
