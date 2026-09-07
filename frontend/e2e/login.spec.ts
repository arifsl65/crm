import { test, expect } from '@playwright/test';

test.describe('Login Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to login page
    await page.goto('/login');
  });

  test('should display login form', async ({ page }) => {
    // Check that login form elements are present
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test('should show error for invalid credentials', async ({ page }) => {
    // Fill in invalid credentials
    await page.fill('input[type="email"]', 'invalid@test.com');
    await page.fill('input[type="password"]', 'wrongpassword');

    // Submit the form
    await page.click('button[type="submit"]');

    // Wait for error message
    await expect(page.locator('text=Invalid email or password')).toBeVisible({ timeout: 10000 });
  });

  test('should login successfully with valid credentials', async ({ page, context }) => {
    // Capture network responses for debugging
    const responses: { url: string; status: number; body: string }[] = [];

    // Capture console messages
    const consoleLogs: string[] = [];
    page.on('console', msg => {
      consoleLogs.push(`[${msg.type()}] ${msg.text()}`);
    });

    page.on('response', async (response) => {
      if (response.url().includes('/api/v1/auth/login')) {
        try {
          const body = await response.text();
          responses.push({
            url: response.url(),
            status: response.status(),
            body: body.substring(0, 500),
          });
          console.log('Login API Response:', response.status(), body.substring(0, 200));
        } catch (e) {
          console.log('Could not read response body');
        }
      }
    });

    page.on('request', (request) => {
      if (request.url().includes('/api/v1/auth/login')) {
        console.log('Login API Request:', request.method(), request.url());
        console.log('Request Headers:', JSON.stringify(request.headers(), null, 2));
        try {
          console.log('Request Body:', request.postData());
        } catch (e) {
          console.log('Could not read request body');
        }
      }
    });

    // Fill in valid credentials
    await page.fill('input[type="email"]', 'admin@test.com');
    await page.fill('input[type="password"]', 'Test123!');

    // Submit the form
    await page.click('button[type="submit"]');

    // Wait a bit for the API call
    await page.waitForTimeout(3000);

    // Log responses for debugging
    console.log('Captured responses:', JSON.stringify(responses, null, 2));

    // Check localStorage immediately after login
    const localStorage = await page.evaluate(() => {
      return {
        user: window.localStorage.getItem('user'),
        allKeys: Object.keys(window.localStorage),
      };
    });
    console.log('LocalStorage:', JSON.stringify(localStorage, null, 2));

    // Check current URL
    console.log('Current URL after login:', page.url());

    // Log console messages
    if (consoleLogs.length > 0) {
      console.log('Console logs:', consoleLogs.join('\n'));
    }

    // Check cookies (debug)
    const debugCookies = await context.cookies();
    console.log('Cookies:', JSON.stringify(debugCookies.map(c => ({ name: c.name, domain: c.domain })), null, 2));

    // Check if there's an error displayed
    const errorVisible = await page.locator('.bg-red-50, .bg-red-900\\/50').isVisible();
    if (errorVisible) {
      const errorText = await page.locator('.bg-red-50 p, .bg-red-900\\/50 p').textContent();
      console.log('Error displayed on page:', errorText);

      // Log network info for debugging
      if (responses.length > 0) {
        console.log('API returned:', responses[0].status, responses[0].body);
      }
    }

    // Wait for redirect to dashboard
    await page.waitForURL('**/dashboard**', { timeout: 15000 });

    // Verify we're on the dashboard
    expect(page.url()).toContain('/dashboard');

    // Check that cookies are set
    const cookies = await context.cookies();
    const accessToken = cookies.find(c => c.name === 'access_token');
    const refreshToken = cookies.find(c => c.name === 'refresh_token');

    // Cookies should be set after successful login
    expect(accessToken).toBeDefined();
    expect(refreshToken).toBeDefined();
  });

  test('should persist authentication after page reload', async ({ page, context }) => {
    // First, login
    await page.fill('input[type="email"]', 'admin@test.com');
    await page.fill('input[type="password"]', 'Test123!');
    await page.click('button[type="submit"]');

    // Wait for dashboard
    await page.waitForURL('**/dashboard**', { timeout: 15000 });

    // Reload the page
    await page.reload();

    // Should still be on dashboard (not redirected to login)
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('/dashboard');
  });

  test('should redirect unauthenticated users to login', async ({ page }) => {
    // Try to access dashboard directly without login
    await page.goto('/dashboard');

    // Should be redirected to login
    await page.waitForURL('**/login**', { timeout: 10000 });
  });
});
