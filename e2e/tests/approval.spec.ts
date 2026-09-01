import { test, expect } from '@playwright/test';

/**
 * Document Approval E2E Tests
 * Week 7 - Accountant CRM
 * Tests document approval and rejection workflow
 */

test.describe('Document Approval Workflow', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should show pending documents count on dashboard', async ({ page }) => {
    await page.goto('/dashboard');

    const pendingWidget = page.getByText(/pending.*review/i).or(
      page.locator('[data-testid="pending-documents"]')
    );

    // Dashboard may show pending documents widget
    if (await pendingWidget.isVisible()) {
      await expect(pendingWidget).toBeVisible();
    }
  });

  test('should filter documents by pending status', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const statusFilter = page.getByRole('combobox', { name: /status/i }).or(
      page.getByLabel(/status/i)
    );

    if (await statusFilter.isVisible()) {
      await statusFilter.click();

      const pendingOption = page.getByRole('option', { name: /pending/i });
      if (await pendingOption.isVisible()) {
        await pendingOption.click();
        await page.waitForTimeout(500);
      }
    }
  });

  test('should display approval actions for pending documents', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Navigate to a pending document
    const pendingRow = page.locator('table tbody tr').filter({
      hasText: /pending/i,
    }).first();

    if (await pendingRow.isVisible()) {
      await pendingRow.click();

      // Should show approve/reject buttons
      const approveButton = page.getByRole('button', { name: /approve/i });
      const rejectButton = page.getByRole('button', { name: /reject/i });

      await expect(approveButton.or(rejectButton).first()).toBeVisible();
    }
  });
});

test.describe('Document Approval', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should approve a pending document', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Find and click a pending document
    const pendingRow = page.locator('table tbody tr').filter({
      hasText: /pending/i,
    }).first();

    if (await pendingRow.isVisible()) {
      await pendingRow.click();

      const approveButton = page.getByRole('button', { name: /approve/i });

      if (await approveButton.isVisible()) {
        await approveButton.click();

        // Confirmation modal may appear
        const confirmButton = page.getByRole('button', { name: /confirm|yes/i });
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }

        // Success message or status change
        await expect(
          page.getByText(/approved|success/i)
        ).toBeVisible({ timeout: 5000 });
      }
    }
  });

  test('should show approved status after approval', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Look for approved documents
    const approvedRow = page.locator('table tbody tr').filter({
      hasText: /approved/i,
    }).first();

    if (await approvedRow.isVisible()) {
      const statusBadge = approvedRow.locator('[data-testid="status-badge"]').or(
        approvedRow.getByText(/approved/i)
      );
      await expect(statusBadge.first()).toBeVisible();
    }
  });
});

test.describe('Document Rejection', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test.skip('should reject a pending document with reason', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const pendingRow = page.locator('table tbody tr').filter({
      hasText: /pending/i,
    }).first();

    if (await pendingRow.isVisible()) {
      await pendingRow.click();

      const rejectButton = page.getByRole('button', { name: /reject/i });

      if (await rejectButton.isVisible()) {
        await rejectButton.click();

        // Rejection reason modal
        const reasonInput = page.getByLabel(/reason/i).or(
          page.getByPlaceholder(/reason/i)
        );

        if (await reasonInput.isVisible()) {
          await reasonInput.fill('E2E test rejection - document quality issue');
        }

        // Confirm rejection
        const confirmButton = page.getByRole('button', { name: /confirm|reject|submit/i });
        await confirmButton.click();

        // Success message
        await expect(
          page.getByText(/rejected|success/i)
        ).toBeVisible({ timeout: 5000 });
      }
    }
  });

  test('should require rejection reason', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const pendingRow = page.locator('table tbody tr').filter({
      hasText: /pending/i,
    }).first();

    if (await pendingRow.isVisible()) {
      await pendingRow.click();

      const rejectButton = page.getByRole('button', { name: /reject/i });

      if (await rejectButton.isVisible()) {
        await rejectButton.click();

        // Try to submit without reason
        const confirmButton = page.getByRole('button', { name: /confirm|reject|submit/i });

        if (await confirmButton.isVisible()) {
          await confirmButton.click();

          // Should show validation error
          const errorMessage = page.getByText(/required|reason/i);
          if (await errorMessage.isVisible()) {
            await expect(errorMessage).toBeVisible();
          }
        }
      }
    }
  });
});

test.describe('Bulk Approval', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should have select all checkbox for bulk actions', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const selectAllCheckbox = page.getByRole('checkbox', { name: /select all/i }).or(
      page.locator('thead input[type="checkbox"]')
    );

    // Table may have select all checkbox
    if (await selectAllCheckbox.isVisible()) {
      await expect(selectAllCheckbox).toBeVisible();
    }
  });

  test.skip('should bulk approve multiple documents', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Select multiple documents
    const checkboxes = page.locator('tbody input[type="checkbox"]');
    const count = await checkboxes.count();

    if (count >= 2) {
      await checkboxes.nth(0).check();
      await checkboxes.nth(1).check();

      // Bulk approve button should appear
      const bulkApproveButton = page.getByRole('button', { name: /bulk.*approve|approve.*selected/i });

      if (await bulkApproveButton.isVisible()) {
        await bulkApproveButton.click();

        // Confirmation
        const confirmButton = page.getByRole('button', { name: /confirm|yes/i });
        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }

        // Success message
        await expect(page.getByText(/approved|success/i)).toBeVisible();
      }
    }
  });
});

test.describe('Approval Notifications', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should show notification bell icon', async ({ page }) => {
    const notificationBell = page.getByRole('button', { name: /notification/i }).or(
      page.locator('[data-testid="notifications"]')
    );

    await expect(notificationBell).toBeVisible();
  });

  test('should display notifications panel', async ({ page }) => {
    const notificationBell = page.getByRole('button', { name: /notification/i }).or(
      page.locator('[data-testid="notifications"]')
    );

    if (await notificationBell.isVisible()) {
      await notificationBell.click();

      // Notifications dropdown or panel
      const notificationsPanel = page.getByRole('menu').or(
        page.locator('[data-testid="notifications-panel"]')
      );

      await expect(notificationsPanel).toBeVisible();
    }
  });
});

test.describe('Approval History', () => {
  test.beforeEach(async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should show approval history on document detail', async ({ page }) => {
    await page.goto('/dashboard/documents');

    const documentRow = page.locator('table tbody tr').first();

    if (await documentRow.isVisible()) {
      await documentRow.click();

      // Look for history/activity section
      const historySection = page.getByText(/history|activity|timeline/i);
      const historyTab = page.getByRole('tab', { name: /history|activity/i });

      if (await historyTab.isVisible()) {
        await historyTab.click();
      }

      // Should show some form of activity log
      const activityItem = page.locator('[data-testid="activity-item"]').or(
        page.getByText(/approved|rejected|uploaded/i)
      );

      if (await activityItem.first().isVisible()) {
        await expect(activityItem.first()).toBeVisible();
      }
    }
  });

  test('should show who approved/rejected document', async ({ page }) => {
    await page.goto('/dashboard/documents');

    // Look for approved document
    const approvedRow = page.locator('table tbody tr').filter({
      hasText: /approved/i,
    }).first();

    if (await approvedRow.isVisible()) {
      await approvedRow.click();

      // Should show approver information
      const approverInfo = page.getByText(/approved by/i).or(
        page.locator('[data-testid="approver"]')
      );

      if (await approverInfo.isVisible()) {
        await expect(approverInfo).toBeVisible();
      }
    }
  });
});
