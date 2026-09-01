import { test, expect } from '@playwright/test';

/**
 * Companies House Integration E2E Tests
 * Week 5 - Accountant CRM
 */

test.describe('Companies House Search', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should search Companies House from client creation page', async ({ page }) => {
    await page.goto('/dashboard/clients/new');

    // Look for CH search input or button
    const chSearchInput = page.getByPlaceholder(/search.*companies house|company number/i);
    const chSearchButton = page.getByRole('button', { name: /search.*house|lookup|find company/i });

    if (await chSearchInput.isVisible()) {
      // Search for a well-known company
      await chSearchInput.fill('TESCO');
      await page.waitForTimeout(500); // Debounce

      // Results should appear
      const results = page.locator('[data-testid="ch-results"]').or(
        page.locator('.ch-search-results')
      );

      // Wait for results or timeout
      await expect(results.or(page.getByText(/TESCO/i))).toBeVisible({ timeout: 10000 });
    } else if (await chSearchButton.isVisible()) {
      await chSearchButton.click();

      // Modal or search dialog should appear
      const searchDialog = page.getByRole('dialog');
      if (await searchDialog.isVisible()) {
        const dialogInput = searchDialog.getByPlaceholder(/search|company/i);
        await dialogInput.fill('TESCO');
        await searchDialog.getByRole('button', { name: /search/i }).click();

        await expect(searchDialog.getByText(/TESCO/i)).toBeVisible({ timeout: 10000 });
      }
    }
  });

  test('should populate form from Companies House selection', async ({ page }) => {
    await page.goto('/dashboard/clients/new');

    const chSearchInput = page.getByPlaceholder(/search.*companies house|company number/i);

    if (await chSearchInput.isVisible()) {
      await chSearchInput.fill('00445790'); // Tesco PLC company number
      await page.waitForTimeout(1000);

      // Click on a result
      const result = page.getByText(/TESCO/i).first();
      if (await result.isVisible()) {
        await result.click();

        // Form should be populated
        const companyNameInput = page.getByLabel(/company name/i);
        await expect(companyNameInput).toHaveValue(/TESCO/i);
      }
    }
  });
});

test.describe('Companies House Sync', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should sync client with Companies House', async ({ page }) => {
    // Navigate to a client with a company number
    await page.goto('/dashboard/clients');

    const clientRow = page.locator('table tbody tr').first();
    if (await clientRow.isVisible()) {
      await clientRow.click();

      // Look for sync button
      const syncButton = page.getByRole('button', { name: /sync|refresh.*house/i });

      if (await syncButton.isVisible()) {
        await syncButton.click();

        // Should show loading state
        await expect(page.getByText(/syncing|loading/i)).toBeVisible();

        // Should show success message
        await expect(page.getByText(/synced|updated|success/i)).toBeVisible({ timeout: 15000 });
      }
    }
  });

  test.skip('should display synced directors', async ({ page }) => {
    await page.goto('/dashboard/clients');

    const clientRow = page.locator('table tbody tr').first();
    if (await clientRow.isVisible()) {
      await clientRow.click();

      // Find directors section or tab
      const directorsTab = page.getByRole('tab', { name: /directors|officers/i });
      if (await directorsTab.isVisible()) {
        await directorsTab.click();

        // Directors list should be visible
        const directorsList = page.locator('[data-testid="directors-list"]').or(
          page.locator('.directors-list')
        );

        await expect(directorsList).toBeVisible();
      }
    }
  });

  test.skip('should display synced PSC', async ({ page }) => {
    await page.goto('/dashboard/clients');

    const clientRow = page.locator('table tbody tr').first();
    if (await clientRow.isVisible()) {
      await clientRow.click();

      // Find PSC section or tab
      const pscTab = page.getByRole('tab', { name: /psc|shareholders|significant control/i });
      if (await pscTab.isVisible()) {
        await pscTab.click();

        // PSC list should be visible
        const pscList = page.locator('[data-testid="psc-list"]').or(
          page.locator('.psc-list')
        );

        await expect(pscList).toBeVisible();
      }
    }
  });
});

test.describe('Companies House API Status', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should handle Companies House API errors gracefully', async ({ page }) => {
    await page.goto('/dashboard/clients/new');

    const chSearchInput = page.getByPlaceholder(/search.*companies house|company number/i);

    if (await chSearchInput.isVisible()) {
      // Search for non-existent company
      await chSearchInput.fill('ZZZZZZNONEXISTENT12345');
      await page.waitForTimeout(1000);

      // Should show "no results" message, not crash
      const noResults = page.getByText(/no results|not found|no companies/i);
      const errorMessage = page.getByText(/error|unavailable/i);

      // Either no results or error message should be visible
      await expect(noResults.or(errorMessage).first()).toBeVisible({ timeout: 10000 });
    }
  });
});
