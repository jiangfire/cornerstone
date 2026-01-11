import { test, expect } from '@playwright/test';
import { login, logout, registerUser, TEST_USERS } from './utils/auth';
import { createDatabase } from './utils/database';
import { createTableWithFields, setFieldPermission } from './utils/field-permission';

/**
 * Test permission enforcement - ensuring users can only perform allowed actions
 */
test.describe('Permission Enforcement Tests', () => {
  const dbName = 'Enforcement Test DB';
  const tableName = 'Enforcement Table';
  const fields = [
    { name: 'field1', type: '字符串' },
    { name: 'field2', type: '字符串' },
    { name: 'field3', type: '字符串' }
  ];

  test.beforeAll(async ({ browser }) => {
    // Setup: Create database, table, and set specific permissions
    const page = await browser.newPage();
    await login(page, TEST_USERS.owner);

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Set specific permissions:
    // - Editor: read+write on field1, read-only on field2, no access to field3
    // - Viewer: read-only on field1, no access to field2 and field3
    await setFieldPermission(page, tableName, 'field1', 'editor', { read: true, write: true, delete: false });
    await setFieldPermission(page, tableName, 'field2', 'editor', { read: true, write: false, delete: false });
    await setFieldPermission(page, tableName, 'field1', 'viewer', { read: true, write: false, delete: false });

    await page.close();
  });

  test('editor can read and write field1', async ({ page }) => {
    const editorUser = await registerUser(page, 'editor');
    await logout(page);
    await login(page, TEST_USERS.editor);

    // Navigate to table records
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: dbName });
    await dbRow.click('button:has-text("查看")');
    await page.waitForURL(/\/databases\/.+/);

    const tableRow = await page.locator('table tbody tr').filter({ hasText: tableName });
    await tableRow.click('button:has-text("数据管理")');
    await page.waitForURL(/\/tables\/.+\/records/);

    // Create new record
    await page.click('button:has-text("新建记录")');
    await expect(page.locator('.el-dialog')).toBeVisible();

    // field1 should be editable
    const field1Input = page.locator('input[placeholder*="field1"]');
    await expect(field1Input).toBeVisible();
    await field1Input.fill('test value');

    // field2 should be visible but read-only (check if disabled)
    const field2Input = page.locator('input[placeholder*="field2"]');
    await expect(field2Input).toBeVisible();

    // field3 should not be visible (no permission)
    const field3Input = page.locator('input[placeholder*="field3"]');
    await expect(field3Input).not.toBeVisible();
  });

  test('editor cannot write to read-only field2', async ({ page }) => {
    const editorUser = await registerUser(page, 'editor');
    await logout(page);
    await login(page, TEST_USERS.editor);

    // Navigate to records
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: dbName });
    await dbRow.click('button:has-text("查看")');

    const tableRow = await page.locator('table tbody tr').filter({ hasText: tableName });
    await tableRow.click('button:has-text("数据管理")');

    // Create record
    await page.click('button:has-text("新建记录")');
    await expect(page.locator('.el-dialog')).toBeVisible();

    // Try to interact with field2 - it should be disabled or readonly
    const field2Input = page.locator('input[placeholder*="field2"]');
    const isDisabled = await field2Input.isDisabled();
    const isReadOnly = await field2Input.getAttribute('readonly');

    // Either disabled or readonly should be true
    expect(isDisabled || isReadOnly !== null).toBe(true);
  });

  test('viewer can only read field1', async ({ page }) => {
    const viewerUser = await registerUser(page, 'viewer');
    await logout(page);
    await login(page, TEST_USERS.viewer);

    // Navigate to records
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: dbName });
    await dbRow.click('button:has-text("查看")');

    const tableRow = await page.locator('table tbody tr').filter({ hasText: tableName });
    await tableRow.click('button:has-text("数据管理")');

    // Create record
    await page.click('button:has-text("新建记录")');
    await expect(page.locator('.el-dialog')).toBeVisible();

    // field1 should be visible but read-only
    const field1Input = page.locator('input[placeholder*="field1"]');
    await expect(field1Input).toBeVisible();
    const isReadOnly1 = await field1Input.getAttribute('readonly');
    expect(isReadOnly1).not.toBeNull();

    // field2 should not be visible (no permission)
    const field2Input = page.locator('input[placeholder*="field2"]');
    await expect(field2Input).not.toBeVisible();

    // field3 should not be visible (no permission)
    const field3Input = page.locator('input[placeholder*="field3"]');
    await expect(field3Input).not.toBeVisible();
  });

  test('owner has full access to all fields', async ({ page }) => {
    await login(page, TEST_USERS.owner);

    // Navigate to records
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: dbName });
    await dbRow.click('button:has-text("查看")');

    const tableRow = await page.locator('table tbody tr').filter({ hasText: tableName });
    await tableRow.click('button:has-text("数据管理")');

    // Create record
    await page.click('button:has-text("新建记录")');
    await expect(page.locator('.el-dialog')).toBeVisible();

    // All fields should be visible and editable
    const field1Input = page.locator('input[placeholder*="field1"]');
    const field2Input = page.locator('input[placeholder*="field2"]');
    const field3Input = page.locator('input[placeholder*="field3"]');

    await expect(field1Input).toBeVisible();
    await expect(field2Input).toBeVisible();
    await expect(field3Input).toBeVisible();

    // All should be editable
    await field1Input.fill('owner value 1');
    await field2Input.fill('owner value 2');
    await field3Input.fill('owner value 3');
  });

  test('admin has full access to all fields', async ({ page }) => {
    const adminUser = await registerUser(page, 'admin');
    await logout(page);
    await login(page, TEST_USERS.admin);

    // Navigate to records
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: dbName });
    await dbRow.click('button:has-text("查看")');

    const tableRow = await page.locator('table tbody tr').filter({ hasText: tableName });
    await tableRow.click('button:has-text("数据管理")');

    // Create record
    await page.click('button:has-text("新建记录")');
    await expect(page.locator('.el-dialog')).toBeVisible();

    // All fields should be visible and editable
    const field1Input = page.locator('input[placeholder*="field1"]');
    const field2Input = page.locator('input[placeholder*="field2"]');
    const field3Input = page.locator('input[placeholder*="field3"]');

    await expect(field1Input).toBeVisible();
    await expect(field2Input).toBeVisible();
    await expect(field3Input).toBeVisible();
  });

  test('user without database access cannot view tables', async ({ page }) => {
    // Register a user who has no access to the test database
    const randomUser = {
      username: 'random_user_' + Date.now(),
      email: 'random@test.com',
      password: 'password123'
    };

    await page.goto('/register');
    await page.fill('input[placeholder="用户名"]', randomUser.username);
    await page.fill('input[placeholder="邮箱"]', randomUser.email);
    await page.fill('input[placeholder="密码"]', randomUser.password);
    await page.fill('input[placeholder="确认密码"]', randomUser.password);
    await page.click('button[type="submit"]');

    await page.waitForURL(/\/(login|dashboard)/);
    await login(page, randomUser);

    // Try to navigate directly to the test database tables
    // Get the database ID from the URL pattern or navigate to databases list
    await page.goto('/databases');

    // The test database should not be visible to this user
    await expect(page.locator('text=' + dbName)).not.toBeVisible();
  });

  test('user with no field permissions sees empty field list', async ({ page }) => {
    // Create a table with no permissions for viewer
    const noPermDb = 'No Perm DB';
    const noPermTable = 'No Perm Table';
    const noPermFields = [{ name: 'restricted', type: '字符串' }];

    await createDatabase(page, noPermDb);
    await createTableWithFields(page, noPermDb, noPermTable, noPermFields);

    // Set no permissions for viewer
    await setFieldPermission(page, noPermTable, 'restricted', 'viewer', {
      read: false,
      write: false,
      delete: false
    });

    // Login as viewer
    const viewerUser = await registerUser(page, 'viewer');
    await logout(page);
    await login(page, TEST_USERS.viewer);

    // Navigate to records
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: noPermDb });
    await dbRow.click('button:has-text("查看")');

    const tableRow = await page.locator('table tbody tr').filter({ hasText: noPermTable });
    await tableRow.click('button:has-text("数据管理")');

    // Try to create record - should see no fields or empty form
    await page.click('button:has-text("新建记录")');
    await expect(page.locator('.el-dialog')).toBeVisible();

    // Should not see the restricted field
    const restrictedInput = page.locator('input[placeholder*="restricted"]');
    await expect(restrictedInput).not.toBeVisible();

    // Or the form might be empty
    const formContent = await page.locator('.el-dialog__body').textContent();
    expect(formContent?.trim() === '' || formContent?.includes('暂无字段')).toBeTruthy();
  });
});