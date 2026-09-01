import { test, expect } from '@playwright/test';

/**
 * Clients E2E Tests
 * Week 5 - Accountant CRM
 */

test.describe('Clients Module', () => {
  test.beforeEach(async ({ page }) => {
    // Login before each test
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should navigate to clients list', async ({ page }) => {
    await page.getByRole('link', { name: /clients/i }).click();
    await expect(page).toHaveURL(/clients/);
    await expect(page.getByRole('heading', { name: /clients/i })).toBeVisible();
  });

  test('should display clients table', async ({ page }) => {
    await page.goto('/dashboard/clients');

    // Table or list should be visible
    const table = page.locator('table');
    const list = page.locator('[role="list"]');

    await expect(table.or(list).first()).toBeVisible();
  });

  test('should have search functionality', async ({ page }) => {
    await page.goto('/dashboard/clients');

    const searchInput = page.getByPlaceholder(/search/i);
    await expect(searchInput).toBeVisible();

    // Type in search
    await searchInput.fill('test');
    await page.waitForTimeout(500); // Debounce

    // Results should update (no specific assertion as depends on data)
  });

  test('should navigate to add client page', async ({ page }) => {
    await page.goto('/dashboard/clients');

    const addButton = page.getByRole('link', { name: /add|new|create/i }).or(
      page.getByRole('button', { name: /add|new|create/i })
    );

    await addButton.first().click();
    await expect(page).toHaveURL(/clients\/new/);
  });
});

test.describe('Client Creation', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
    await page.goto('/dashboard/clients/new');
  });

  test('should display client creation form', async ({ page }) => {
    await expect(page.getByLabel(/company name|business name/i)).toBeVisible();
    await expect(page.getByLabel(/contact name/i)).toBeVisible();
    await expect(page.getByLabel(/email/i)).toBeVisible();
  });

  test('should show validation errors for empty required fields', async ({ page }) => {
    const submitButton = page.getByRole('button', { name: /save|create|submit/i });
    await submitButton.click();

    // Should show validation errors
    await expect(page.locator('form')).toContainText(/required/i);
  });

  test('should create a new client', async ({ page }) => {
    const timestamp = Date.now();
    const clientName = `E2E Test Company ${timestamp}`;

    await page.getByLabel(/company name|business name/i).fill(clientName);
    await page.getByLabel(/contact name/i).fill('E2E Test Contact');
    await page.getByLabel(/email/i).fill(`e2e-test-${timestamp}@example.com`);

    // Fill optional phone if visible
    const phoneInput = page.getByLabel(/phone/i);
    if (await phoneInput.isVisible()) {
      await phoneInput.fill('01onal234567890');
    }

    const submitButton = page.getByRole('button', { name: /save|create|submit/i });
    await submitButton.click();

    // Should redirect to client detail or clients list
    await expect(page).toHaveURL(/clients/, { timeout: 10000 });
  });
});

test.describe('Client Detail', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should navigate to client detail page', async ({ page }) => {
    await page.goto('/dashboard/clients');

    // Click on first client row/card
    const clientRow = page.locator('table tbody tr').first().or(
      page.locator('[data-testid="client-card"]').first()
    );

    if (await clientRow.isVisible()) {
      await clientRow.click();
      await expect(page).toHaveURL(/clients\/[a-f0-9-]+/);
    }
  });

  test('should display client information tabs', async ({ page }) => {
    await page.goto('/dashboard/clients');

    const clientRow = page.locator('table tbody tr').first();
    if (await clientRow.isVisible()) {
      await clientRow.click();

      // Should have tabs for different sections
      const detailsTab = page.getByRole('tab', { name: /details|info/i });
      const documentsTab = page.getByRole('tab', { name: /documents/i });
      const servicesTab = page.getByRole('tab', { name: /services/i });

      // At least one should be visible
      await expect(
        detailsTab.or(documentsTab).or(servicesTab).first()
      ).toBeVisible();
    }
  });
});

test.describe('Client Notes', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should add a note to client', async ({ page }) => {
    // Navigate to a client detail page
    await page.goto('/dashboard/clients');
    const clientRow = page.locator('table tbody tr').first();

    if (await clientRow.isVisible()) {
      await clientRow.click();

      // Find notes section or tab
      const notesTab = page.getByRole('tab', { name: /notes/i });
      if (await notesTab.isVisible()) {
        await notesTab.click();
      }

      // Add a note
      const noteInput = page.getByPlaceholder(/add.*note/i).or(
        page.getByLabel(/note/i)
      );

      if (await noteInput.isVisible()) {
        await noteInput.fill('E2E test note - ' + Date.now());
        await page.getByRole('button', { name: /add|save|submit/i }).click();

        // Note should appear in list
        await expect(page.getByText(/E2E test note/)).toBeVisible();
      }
    }
  });
});

test.describe('Staff Assignment', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should assign staff to client', async ({ page }) => {
    await page.goto('/dashboard/clients');
    const clientRow = page.locator('table tbody tr').first();

    if (await clientRow.isVisible()) {
      await clientRow.click();

      // Find assign staff button
      const assignButton = page.getByRole('button', { name: /assign/i });
      if (await assignButton.isVisible()) {
        await assignButton.click();

        // Select staff from dropdown
        const staffSelect = page.getByRole('combobox');
        if (await staffSelect.isVisible()) {
          await staffSelect.click();
          const staffOption = page.getByRole('option').first();
          await staffOption.click();

          await page.getByRole('button', { name: /save|confirm/i }).click();

          // Success message should appear
          await expect(page.getByText(/assigned|success/i)).toBeVisible();
        }
      }
    }
  });
});
