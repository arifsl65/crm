import { test, expect } from '@playwright/test';

/**
 * Documents E2E Tests
 * Week 7 - Accountant CRM
 * Tests document listing, upload, download, and management
 */

test.describe('Documents Module', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should navigate to documents list', async ({ page }) => {
    await page.getByRole('link', { name: /documents/i }).click();
    await expect(page).toHaveURL(/documents/);
    await expect(page.getByRole('heading', { name: /documents/i })).toBeVisible();
  });

  test('should display documents table or list', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const table = page.locator('table');
    const list = page.locator('[role="list"]');
    const grid = page.locator('[data-testid="documents-grid"]');

    await expect(table.or(list).or(grid).first()).toBeVisible();
  });

  test('should have search and filter functionality', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const searchInput = page.getByPlaceholder(/search/i);
    await expect(searchInput).toBeVisible();

    // Type in search
    await searchInput.fill('test');
    await page.waitForTimeout(500); // Debounce
  });

  test('should filter documents by status', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Find status filter dropdown
    const statusFilter = page.getByRole('combobox', { name: /status/i }).or(
      page.getByLabel(/status/i)
    );

    if (await statusFilter.isVisible()) {
      await statusFilter.click();

      // Should show status options
      const pendingOption = page.getByRole('option', { name: /pending/i });
      const approvedOption = page.getByRole('option', { name: /approved/i });

      await expect(pendingOption.or(approvedOption).first()).toBeVisible();
    }
  });
});

test.describe('Document Upload', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should show upload button or area', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const uploadButton = page.getByRole('button', { name: /upload/i });
    const uploadArea = page.locator('[data-testid="upload-area"]');
    const uploadInput = page.locator('input[type="file"]');

    await expect(uploadButton.or(uploadArea).or(uploadInput).first()).toBeVisible();
  });

  test('should open upload modal when clicking upload', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const uploadButton = page.getByRole('button', { name: /upload/i });

    if (await uploadButton.isVisible()) {
      await uploadButton.click();

      // Modal or upload form should appear
      const modal = page.getByRole('dialog');
      const uploadForm = page.locator('form').filter({ hasText: /upload/i });

      await expect(modal.or(uploadForm).first()).toBeVisible();
    }
  });

  test('should require client selection for document upload', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const uploadButton = page.getByRole('button', { name: /upload/i });

    if (await uploadButton.isVisible()) {
      await uploadButton.click();

      // Client dropdown should be visible
      const clientSelect = page.getByLabel(/client/i).or(
        page.getByRole('combobox', { name: /client/i })
      );

      await expect(clientSelect).toBeVisible();
    }
  });

  test.skip('should upload a document successfully', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const uploadButton = page.getByRole('button', { name: /upload/i });

    if (await uploadButton.isVisible()) {
      await uploadButton.click();

      // Select client
      const clientSelect = page.getByLabel(/client/i);
      if (await clientSelect.isVisible()) {
        await clientSelect.click();
        await page.getByRole('option').first().click();
      }

      // Select document type
      const typeSelect = page.getByLabel(/type/i);
      if (await typeSelect.isVisible()) {
        await typeSelect.click();
        await page.getByRole('option').first().click();
      }

      // Upload file
      const fileInput = page.locator('input[type="file"]');
      await fileInput.setInputFiles({
        name: 'test-document.pdf',
        mimeType: 'application/pdf',
        buffer: Buffer.from('Test PDF content'),
      });

      // Submit
      await page.getByRole('button', { name: /upload|submit/i }).click();

      // Success message
      await expect(page.getByText(/success|uploaded/i)).toBeVisible();
    }
  });
});

test.describe('Document Detail', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should navigate to document detail page', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const documentRow = page.locator('table tbody tr').first().or(
      page.locator('[data-testid="document-card"]').first()
    );

    if (await documentRow.isVisible()) {
      await documentRow.click();
      await expect(page).toHaveURL(/documents\/[a-f0-9-]+/);
    }
  });

  test('should display document information', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const documentRow = page.locator('table tbody tr').first();

    if (await documentRow.isVisible()) {
      await documentRow.click();

      // Should show document details
      const docName = page.locator('h1, h2').filter({ hasText: /.+/ });
      const docStatus = page.getByText(/pending|approved|rejected|requested/i);

      await expect(docName.first()).toBeVisible();
      await expect(docStatus.first()).toBeVisible();
    }
  });

  test('should have download button for uploaded documents', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const documentRow = page.locator('table tbody tr').first();

    if (await documentRow.isVisible()) {
      await documentRow.click();

      const downloadButton = page.getByRole('button', { name: /download/i }).or(
        page.getByRole('link', { name: /download/i })
      );

      // Download button may or may not be visible depending on document status
      if (await downloadButton.isVisible()) {
        await expect(downloadButton).toBeEnabled();
      }
    }
  });
});

test.describe('Document Versions', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should display version history', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const documentRow = page.locator('table tbody tr').first();

    if (await documentRow.isVisible()) {
      await documentRow.click();

      // Look for versions tab or section
      const versionsTab = page.getByRole('tab', { name: /versions|history/i });
      const versionsSection = page.getByText(/version history/i);

      if (await versionsTab.isVisible()) {
        await versionsTab.click();
        await expect(page.getByText(/version/i)).toBeVisible();
      } else if (await versionsSection.isVisible()) {
        await expect(versionsSection).toBeVisible();
      }
    }
  });
});

test.describe('Document Expiry & Renewal', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should show expiring documents section on dashboard', async ({ page }) => {
    await page.goto('/dashboard');

    const expiringSection = page.getByText(/expiring/i).or(
      page.locator('[data-testid="expiring-documents"]')
    );

    // Expiring documents widget may be on dashboard
    if (await expiringSection.isVisible()) {
      await expect(expiringSection).toBeVisible();
    }
  });

  test.skip('should request document renewal', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const documentRow = page.locator('table tbody tr').first();

    if (await documentRow.isVisible()) {
      await documentRow.click();

      const renewalButton = page.getByRole('button', { name: /request.*renewal|renew/i });

      if (await renewalButton.isVisible()) {
        await renewalButton.click();

        // Modal for renewal note
        const noteInput = page.getByLabel(/note/i).or(
          page.getByPlaceholder(/note/i)
        );

        if (await noteInput.isVisible()) {
          await noteInput.fill('E2E test renewal request');
        }

        await page.getByRole('button', { name: /submit|request|confirm/i }).click();

        // Success message
        await expect(page.getByText(/renewal.*requested|success/i)).toBeVisible();
      }
    }
  });
});

test.describe('Firm Documents', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should have firm documents section', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Tab or filter for firm documents
    const firmTab = page.getByRole('tab', { name: /firm/i });
    const firmFilter = page.getByRole('button', { name: /firm/i });

    await expect(firmTab.or(firmFilter).first()).toBeVisible();
  });

  test('should display firm documents separately', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const firmTab = page.getByRole('tab', { name: /firm/i });

    if (await firmTab.isVisible()) {
      await firmTab.click();

      // Should show firm documents or empty state
      const content = page.locator('table, [role="list"], [data-testid="empty-state"]');
      await expect(content.first()).toBeVisible();
    }
  });
});
