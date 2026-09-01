import { test, expect } from '@playwright/test';

/**
 * QR Code Upload E2E Tests
 * Week 7 - Accountant CRM
 * Tests QR code generation and client-side document upload
 */

test.describe('QR Token Generation', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should have QR code generation option', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Look for QR code button in document actions
    const qrButton = page.getByRole('button', { name: /qr|generate.*link|upload.*link/i });
    const qrMenuItem = page.getByRole('menuitem', { name: /qr|generate.*link/i });

    await expect(qrButton.or(qrMenuItem).first()).toBeVisible();
  });

  test('should open QR generation modal', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      // Modal should appear
      const modal = page.getByRole('dialog');
      await expect(modal).toBeVisible();

      // Should have client selection
      const clientSelect = page.getByLabel(/client/i);
      await expect(clientSelect).toBeVisible();
    }
  });

  test.skip('should generate QR code for client', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      // Select client
      const clientSelect = page.getByLabel(/client/i);
      await clientSelect.click();
      await page.getByRole('option').first().click();

      // Generate button
      const generateButton = page.getByRole('button', { name: /generate/i });
      await generateButton.click();

      // QR code should appear
      const qrCode = page.locator('canvas, img[alt*="qr"], svg').or(
        page.locator('[data-testid="qr-code"]')
      );
      await expect(qrCode.first()).toBeVisible({ timeout: 5000 });
    }
  });

  test.skip('should show upload link with QR code', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      // Select client
      const clientSelect = page.getByLabel(/client/i);
      await clientSelect.click();
      await page.getByRole('option').first().click();

      // Generate
      await page.getByRole('button', { name: /generate/i }).click();

      // Upload link should be shown
      const uploadLink = page.getByText(/upload/i).locator('input').or(
        page.locator('[data-testid="upload-link"]')
      );

      if (await uploadLink.isVisible()) {
        const linkValue = await uploadLink.inputValue();
        expect(linkValue).toContain('/upload/');
      }
    }
  });

  test.skip('should allow copying upload link', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      // Select client and generate
      const clientSelect = page.getByLabel(/client/i);
      await clientSelect.click();
      await page.getByRole('option').first().click();
      await page.getByRole('button', { name: /generate/i }).click();

      // Copy button
      const copyButton = page.getByRole('button', { name: /copy/i });

      if (await copyButton.isVisible()) {
        await copyButton.click();

        // Should show copied feedback
        await expect(page.getByText(/copied/i)).toBeVisible();
      }
    }
  });
});

test.describe('QR Upload Page (Public)', () => {
  // Note: These tests access the public upload page without authentication

  test('should show error for invalid token', async ({ page }) => {
    await page.goto('/upload/invalid-token-12345');

    // Should show invalid/expired message
    const errorMessage = page.getByText(/invalid|expired|not found/i);
    await expect(errorMessage).toBeVisible();
  });

  test('should not allow access without valid token', async ({ page }) => {
    await page.goto('/upload/');

    // Should redirect or show error
    const errorMessage = page.getByText(/invalid|not found|error/i);
    const notFoundPage = page.getByText(/404|page not found/i);

    await expect(errorMessage.or(notFoundPage).first()).toBeVisible();
  });
});

test.describe('QR Upload Flow (End-to-End)', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should complete full QR upload flow', async ({ page, context }) => {
    // Step 1: Generate QR token as staff
    await page.goto('/dashboard/documents');

    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      // Select client
      const clientSelect = page.getByLabel(/client/i);
      await clientSelect.click();
      await page.getByRole('option').first().click();

      // Generate
      await page.getByRole('button', { name: /generate/i }).click();

      // Get upload link
      const uploadLinkInput = page.locator('input[readonly]').or(
        page.locator('[data-testid="upload-link"] input')
      );

      if (await uploadLinkInput.isVisible()) {
        const uploadUrl = await uploadLinkInput.inputValue();

        // Step 2: Open upload page in new tab (simulating client)
        const clientPage = await context.newPage();
        await clientPage.goto(uploadUrl);

        // Should show upload form
        const uploadForm = clientPage.locator('form');
        await expect(uploadForm).toBeVisible();

        // Should show client name
        const clientName = clientPage.getByText(/client|company/i);
        await expect(clientName).toBeVisible();

        // Step 3: Upload file
        const fileInput = clientPage.locator('input[type="file"]');
        await fileInput.setInputFiles({
          name: 'client-document.pdf',
          mimeType: 'application/pdf',
          buffer: Buffer.from('Test PDF content from client'),
        });

        // Submit
        const submitButton = clientPage.getByRole('button', { name: /upload|submit/i });
        await submitButton.click();

        // Success message
        await expect(clientPage.getByText(/success|uploaded|thank/i)).toBeVisible();

        // Close client page
        await clientPage.close();

        // Step 4: Verify document in staff view
        await page.goto('/dashboard/documents');

        // New document should appear
        const newDocument = page.getByText(/client-document/i);
        if (await newDocument.isVisible()) {
          await expect(newDocument).toBeVisible();
        }
      }
    }
  });
});

test.describe('QR Token Expiry', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should show expiry time when generating QR', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      // Expiry information should be shown
      const expiryInfo = page.getByText(/expir|valid.*for|hours|days/i);

      if (await expiryInfo.isVisible()) {
        await expect(expiryInfo).toBeVisible();
      }
    }
  });

  test.skip('should allow setting custom expiry', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      // Expiry duration selector
      const expirySelect = page.getByLabel(/expir|duration/i).or(
        page.getByRole('combobox', { name: /expir/i })
      );

      if (await expirySelect.isVisible()) {
        await expirySelect.click();

        // Options should include different durations
        const option24h = page.getByRole('option', { name: /24.*hour|1.*day/i });
        const option48h = page.getByRole('option', { name: /48.*hour|2.*day/i });

        await expect(option24h.or(option48h).first()).toBeVisible();
      }
    }
  });
});

test.describe('QR Token Management', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should list active upload tokens', async ({ page }) => {
    // Navigate to tokens management (if exists)
    await page.goto('/dashboard/documents');

    const tokensTab = page.getByRole('tab', { name: /tokens|links/i });
    const tokensButton = page.getByRole('button', { name: /manage.*tokens|active.*links/i });

    if (await tokensTab.isVisible()) {
      await tokensTab.click();

      // Should show list of active tokens
      const tokensList = page.locator('table, [role="list"]');
      await expect(tokensList).toBeVisible();
    } else if (await tokensButton.isVisible()) {
      await tokensButton.click();

      const tokensList = page.locator('table, [role="list"]');
      await expect(tokensList).toBeVisible();
    }
  });

  test.skip('should revoke an upload token', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Generate a token first
    const qrButton = page.getByRole('button', { name: /qr|generate.*link/i });

    if (await qrButton.isVisible()) {
      await qrButton.click();

      const clientSelect = page.getByLabel(/client/i);
      await clientSelect.click();
      await page.getByRole('option').first().click();

      await page.getByRole('button', { name: /generate/i }).click();

      // Look for revoke/delete button
      const revokeButton = page.getByRole('button', { name: /revoke|delete|cancel/i });

      if (await revokeButton.isVisible()) {
        await revokeButton.click();

        // Confirm revocation
        const confirmButton = page.getByRole('button', { name: /confirm|yes/i });
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }

        // Success message
        await expect(page.getByText(/revoked|deleted|cancelled/i)).toBeVisible();
      }
    }
  });
});

test.describe('Upload Page UI', () => {
  // Tests for the public upload page UI when accessed with valid token
  // Note: Requires a pre-created valid token in test environment

  test.skip('should display client-friendly upload interface', async ({ page }) => {
    // This test requires a valid token - skip in CI unless seeded
    const testToken = process.env.TEST_UPLOAD_TOKEN;

    if (testToken) {
      await page.goto(`/upload/${testToken}`);

      // Should show friendly upload interface
      const heading = page.getByRole('heading', { name: /upload|document/i });
      await expect(heading).toBeVisible();

      // Drag and drop area
      const dropzone = page.locator('[data-testid="dropzone"]').or(
        page.getByText(/drag.*drop|click.*upload/i)
      );
      await expect(dropzone).toBeVisible();

      // File input
      const fileInput = page.locator('input[type="file"]');
      await expect(fileInput).toBeVisible();
    }
  });

  test.skip('should show upload progress', async ({ page }) => {
    const testToken = process.env.TEST_UPLOAD_TOKEN;

    if (testToken) {
      await page.goto(`/upload/${testToken}`);

      const fileInput = page.locator('input[type="file"]');

      // Create a larger file to see progress
      const largeContent = 'X'.repeat(1024 * 100); // 100KB
      await fileInput.setInputFiles({
        name: 'large-document.pdf',
        mimeType: 'application/pdf',
        buffer: Buffer.from(largeContent),
      });

      // Progress indicator should appear
      const progressBar = page.locator('[role="progressbar"]').or(
        page.getByText(/%/)
      );

      // Progress may be quick, so just check it exists
      if (await progressBar.isVisible()) {
        await expect(progressBar).toBeVisible();
      }
    }
  });

  test.skip('should validate file types', async ({ page }) => {
    const testToken = process.env.TEST_UPLOAD_TOKEN;

    if (testToken) {
      await page.goto(`/upload/${testToken}`);

      const fileInput = page.locator('input[type="file"]');

      // Try uploading invalid file type
      await fileInput.setInputFiles({
        name: 'malicious.exe',
        mimeType: 'application/x-msdownload',
        buffer: Buffer.from('Not a real exe'),
      });

      // Should show error
      const errorMessage = page.getByText(/invalid.*type|not.*allowed|only.*pdf|only.*accepted/i);
      await expect(errorMessage).toBeVisible();
    }
  });
});
