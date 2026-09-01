import { test, expect } from '@playwright/test';

/**
 * Authentication E2E Tests
 * Week 5 - Accountant CRM
 */

test.describe('Authentication', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
  });

  test('should display login page', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /login|sign in/i })).toBeVisible();
    await expect(page.getByLabel(/email/i)).toBeVisible();
    await expect(page.getByLabel(/password/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /login|sign in/i })).toBeVisible();
  });

  test('should show error for invalid credentials', async ({ page }) => {
    await page.getByLabel(/email/i).fill('invalid@example.com');
    await page.getByLabel(/password/i).fill('wrongpassword');
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page.getByText(/invalid|incorrect|error/i)).toBeVisible();
  });

  test('should show validation error for empty fields', async ({ page }) => {
    await page.getByRole('button', { name: /login|sign in/i }).click();

    // Should show validation errors
    await expect(page.locator('form')).toContainText(/required|email|password/i);
  });

  test('should navigate to forgot password page', async ({ page }) => {
    await page.getByRole('link', { name: /forgot|reset/i }).click();
    await expect(page).toHaveURL(/forgot-password|reset-password/);
  });

  test('should navigate to register page', async ({ page }) => {
    const registerLink = page.getByRole('link', { name: /register|sign up|create account/i });
    if (await registerLink.isVisible()) {
      await registerLink.click();
      await expect(page).toHaveURL(/register/);
    }
  });
});

test.describe('Authentication - Valid Login', () => {
  test('should login with valid credentials', async ({ page }) => {
    // This test requires a valid test user in the database
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    // Should redirect to dashboard on success
    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  });

  test('should persist session after page refresh', async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });

    // Refresh the page
    await page.reload();

    // Should still be on dashboard
    await expect(page).toHaveURL(/dashboard/);
  });
});

test.describe('Authentication - 2FA', () => {
  test.skip('should show 2FA prompt for 2FA-enabled users', async ({ page }) => {
    // This test requires a user with 2FA enabled
    const test2FAEmail = process.env.TEST_2FA_USER_EMAIL;
    const test2FAPassword = process.env.TEST_2FA_USER_PASSWORD;

    if (!test2FAEmail || !test2FAPassword) {
      test.skip();
      return;
    }

    await page.goto('/login');
    await page.getByLabel(/email/i).fill(test2FAEmail);
    await page.getByLabel(/password/i).fill(test2FAPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    // Should show 2FA code input
    await expect(page.getByLabel(/code|otp|2fa/i)).toBeVisible();
  });
});

test.describe('Authentication - Logout', () => {
  test('should logout successfully', async ({ page }) => {
    const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
    const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';

    // Login first
    await page.goto('/login');
    await page.getByLabel(/email/i).fill(testEmail);
    await page.getByLabel(/password/i).fill(testPassword);
    await page.getByRole('button', { name: /login|sign in/i }).click();

    await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });

    // Find and click logout
    const logoutButton = page.getByRole('button', { name: /logout|sign out/i });
    const logoutLink = page.getByRole('link', { name: /logout|sign out/i });

    if (await logoutButton.isVisible()) {
      await logoutButton.click();
    } else if (await logoutLink.isVisible()) {
      await logoutLink.click();
    } else {
      // Try opening user menu first
      await page.getByRole('button', { name: /user|profile|menu/i }).click();
      await page.getByText(/logout|sign out/i).click();
    }

    // Should redirect to login
    await expect(page).toHaveURL(/login/);
  });
});
