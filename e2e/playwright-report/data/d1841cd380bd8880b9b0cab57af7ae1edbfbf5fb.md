# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: documents.spec.ts >> Document Detail >> should have download button for uploaded documents
- Location: tests/documents.spec.ts:207:7

# Error details

```
Error: expect(page).toHaveURL(expected) failed

Expected pattern: /dashboard/
Received string:  "https://crm.irislondonshoes.com/login/"
Timeout: 10000ms

Call log:
  - Expect "toHaveURL" with timeout 10000ms
    24 × locator resolved to <html lang="en" class="light">…</html>
       - unexpected value "https://crm.irislondonshoes.com/login/"

```

```yaml
- heading "Sign in to your account" [level=2]
- paragraph:
  - text: Or
  - link "create a new account":
    - /url: /register/
- paragraph: Too many requests. Please try again later.
- link "Forgot your password?":
  - /url: /forgot-password/
- text: Email address
- textbox "Email address": admin@test.com
- text: Password
- textbox "Password": TestPassword123!
- button "Sign in"
- alert
```

# Test source

```ts
  74  |     await page.goto('/login');
  75  |     await page.getByLabel(/email/i).fill(testEmail);
  76  |     await page.getByLabel(/password/i).fill(testPassword);
  77  |     await page.getByRole('button', { name: /login|sign in/i }).click();
  78  | 
  79  |     await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  80  |   });
  81  | 
  82  |   test('should show upload button or area', async ({ page }) => {
  83  |     await page.goto('/dashboard/documents');
  84  | 
  85  |     const uploadButton = page.getByRole('button', { name: /upload/i });
  86  |     const uploadArea = page.locator('[data-testid="upload-area"]');
  87  |     const uploadInput = page.locator('input[type="file"]');
  88  | 
  89  |     await expect(uploadButton.or(uploadArea).or(uploadInput).first()).toBeVisible();
  90  |   });
  91  | 
  92  |   test('should open upload modal when clicking upload', async ({ page }) => {
  93  |     await page.goto('/dashboard/documents');
  94  | 
  95  |     const uploadButton = page.getByRole('button', { name: /upload/i });
  96  | 
  97  |     if (await uploadButton.isVisible()) {
  98  |       await uploadButton.click();
  99  | 
  100 |       // Modal or upload form should appear
  101 |       const modal = page.getByRole('dialog');
  102 |       const uploadForm = page.locator('form').filter({ hasText: /upload/i });
  103 | 
  104 |       await expect(modal.or(uploadForm).first()).toBeVisible();
  105 |     }
  106 |   });
  107 | 
  108 |   test('should require client selection for document upload', async ({ page }) => {
  109 |     await page.goto('/dashboard/documents');
  110 | 
  111 |     const uploadButton = page.getByRole('button', { name: /upload/i });
  112 | 
  113 |     if (await uploadButton.isVisible()) {
  114 |       await uploadButton.click();
  115 | 
  116 |       // Client dropdown should be visible
  117 |       const clientSelect = page.getByLabel(/client/i).or(
  118 |         page.getByRole('combobox', { name: /client/i })
  119 |       );
  120 | 
  121 |       await expect(clientSelect).toBeVisible();
  122 |     }
  123 |   });
  124 | 
  125 |   test.skip('should upload a document successfully', async ({ page }) => {
  126 |     await page.goto('/dashboard/documents');
  127 | 
  128 |     const uploadButton = page.getByRole('button', { name: /upload/i });
  129 | 
  130 |     if (await uploadButton.isVisible()) {
  131 |       await uploadButton.click();
  132 | 
  133 |       // Select client
  134 |       const clientSelect = page.getByLabel(/client/i);
  135 |       if (await clientSelect.isVisible()) {
  136 |         await clientSelect.click();
  137 |         await page.getByRole('option').first().click();
  138 |       }
  139 | 
  140 |       // Select document type
  141 |       const typeSelect = page.getByLabel(/type/i);
  142 |       if (await typeSelect.isVisible()) {
  143 |         await typeSelect.click();
  144 |         await page.getByRole('option').first().click();
  145 |       }
  146 | 
  147 |       // Upload file
  148 |       const fileInput = page.locator('input[type="file"]');
  149 |       await fileInput.setInputFiles({
  150 |         name: 'test-document.pdf',
  151 |         mimeType: 'application/pdf',
  152 |         buffer: Buffer.from('Test PDF content'),
  153 |       });
  154 | 
  155 |       // Submit
  156 |       await page.getByRole('button', { name: /upload|submit/i }).click();
  157 | 
  158 |       // Success message
  159 |       await expect(page.getByText(/success|uploaded/i)).toBeVisible();
  160 |     }
  161 |   });
  162 | });
  163 | 
  164 | test.describe('Document Detail', () => {
  165 |   test.beforeEach(async ({ page }) => {
  166 |     const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
  167 |     const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';
  168 | 
  169 |     await page.goto('/login');
  170 |     await page.getByLabel(/email/i).fill(testEmail);
  171 |     await page.getByLabel(/password/i).fill(testPassword);
  172 |     await page.getByRole('button', { name: /login|sign in/i }).click();
  173 | 
> 174 |     await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
      |                        ^ Error: expect(page).toHaveURL(expected) failed
  175 |   });
  176 | 
  177 |   test('should navigate to document detail page', async ({ page }) => {
  178 |     await page.goto('/dashboard/documents');
  179 | 
  180 |     const documentRow = page.locator('table tbody tr').first().or(
  181 |       page.locator('[data-testid="document-card"]').first()
  182 |     );
  183 | 
  184 |     if (await documentRow.isVisible()) {
  185 |       await documentRow.click();
  186 |       await expect(page).toHaveURL(/documents\/[a-f0-9-]+/);
  187 |     }
  188 |   });
  189 | 
  190 |   test('should display document information', async ({ page }) => {
  191 |     await page.goto('/dashboard/documents');
  192 | 
  193 |     const documentRow = page.locator('table tbody tr').first();
  194 | 
  195 |     if (await documentRow.isVisible()) {
  196 |       await documentRow.click();
  197 | 
  198 |       // Should show document details
  199 |       const docName = page.locator('h1, h2').filter({ hasText: /.+/ });
  200 |       const docStatus = page.getByText(/pending|approved|rejected|requested/i);
  201 | 
  202 |       await expect(docName.first()).toBeVisible();
  203 |       await expect(docStatus.first()).toBeVisible();
  204 |     }
  205 |   });
  206 | 
  207 |   test('should have download button for uploaded documents', async ({ page }) => {
  208 |     await page.goto('/dashboard/documents');
  209 | 
  210 |     const documentRow = page.locator('table tbody tr').first();
  211 | 
  212 |     if (await documentRow.isVisible()) {
  213 |       await documentRow.click();
  214 | 
  215 |       const downloadButton = page.getByRole('button', { name: /download/i }).or(
  216 |         page.getByRole('link', { name: /download/i })
  217 |       );
  218 | 
  219 |       // Download button may or may not be visible depending on document status
  220 |       if (await downloadButton.isVisible()) {
  221 |         await expect(downloadButton).toBeEnabled();
  222 |       }
  223 |     }
  224 |   });
  225 | });
  226 | 
  227 | test.describe('Document Versions', () => {
  228 |   test.beforeEach(async ({ page }) => {
  229 |     const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
  230 |     const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';
  231 | 
  232 |     await page.goto('/login');
  233 |     await page.getByLabel(/email/i).fill(testEmail);
  234 |     await page.getByLabel(/password/i).fill(testPassword);
  235 |     await page.getByRole('button', { name: /login|sign in/i }).click();
  236 | 
  237 |     await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  238 |   });
  239 | 
  240 |   test.skip('should display version history', async ({ page }) => {
  241 |     await page.goto('/dashboard/documents');
  242 | 
  243 |     const documentRow = page.locator('table tbody tr').first();
  244 | 
  245 |     if (await documentRow.isVisible()) {
  246 |       await documentRow.click();
  247 | 
  248 |       // Look for versions tab or section
  249 |       const versionsTab = page.getByRole('tab', { name: /versions|history/i });
  250 |       const versionsSection = page.getByText(/version history/i);
  251 | 
  252 |       if (await versionsTab.isVisible()) {
  253 |         await versionsTab.click();
  254 |         await expect(page.getByText(/version/i)).toBeVisible();
  255 |       } else if (await versionsSection.isVisible()) {
  256 |         await expect(versionsSection).toBeVisible();
  257 |       }
  258 |     }
  259 |   });
  260 | });
  261 | 
  262 | test.describe('Document Expiry & Renewal', () => {
  263 |   test.beforeEach(async ({ page }) => {
  264 |     const testEmail = process.env.TEST_USER_EMAIL || 'admin@test.com';
  265 |     const testPassword = process.env.TEST_USER_PASSWORD || 'TestPassword123!';
  266 | 
  267 |     await page.goto('/login');
  268 |     await page.getByLabel(/email/i).fill(testEmail);
  269 |     await page.getByLabel(/password/i).fill(testPassword);
  270 |     await page.getByRole('button', { name: /login|sign in/i }).click();
  271 | 
  272 |     await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
  273 |   });
  274 | 
```