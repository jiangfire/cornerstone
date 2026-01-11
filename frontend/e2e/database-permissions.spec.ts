import { test, expect } from '@playwright/test';
import { login, logout, registerUser, TEST_USERS } from './utils/auth';
import { createDatabase, shareDatabase, updateDatabaseUserRole, removeDatabaseUser } from './utils/database';

test.describe('Database Permission Management', () => {
  test.beforeEach(async ({ page }) => {
    // Login as owner before each test
    await login(page, TEST_USERS.owner);
  });

  test('owner can create database', async ({ page }) => {
    const dbName = 'Test DB ' + Date.now();

    await createDatabase(page, dbName);

    // Verify database appears in list
    await expect(page.locator('text=' + dbName)).toBeVisible();
  });

  test('owner can share database with admin role', async ({ page }) => {
    const dbName = 'DB for Admin';
    await createDatabase(page, dbName);

    // Register admin user
    const adminUser = await registerUser(page, 'admin');
    await login(page, TEST_USERS.owner);

    // Share database
    await shareDatabase(page, dbName, adminUser.username, 'admin');

    // Verify sharing success (check user list)
    await page.click('button:has-text("管理用户")');
    await expect(page.locator('text=' + adminUser.username)).toBeVisible();
    await expect(page.locator('text=admin')).toBeVisible();
  });

  test('owner can share database with editor role', async ({ page }) => {
    const dbName = 'DB for Editor';
    await createDatabase(page, dbName);

    const editorUser = await registerUser(page, 'editor');
    await login(page, TEST_USERS.owner);

    await shareDatabase(page, dbName, editorUser.username, 'editor');

    await page.click('button:has-text("管理用户")');
    await expect(page.locator('text=' + editorUser.username)).toBeVisible();
    await expect(page.locator('text=editor')).toBeVisible();
  });

  test('owner can share database with viewer role', async ({ page }) => {
    const dbName = 'DB for Viewer';
    await createDatabase(page, dbName);

    const viewerUser = await registerUser(page, 'viewer');
    await login(page, TEST_USERS.owner);

    await shareDatabase(page, dbName, viewerUser.username, 'viewer');

    await page.click('button:has-text("管理用户")');
    await expect(page.locator('text=' + viewerUser.username)).toBeVisible();
    await expect(page.locator('text=viewer')).toBeVisible();
  });

  test('owner can update database user role', async ({ page }) => {
    const dbName = 'DB Role Update';
    await createDatabase(page, dbName);

    const editorUser = await registerUser(page, 'editor');
    await login(page, TEST_USERS.owner);

    await shareDatabase(page, dbName, editorUser.username, 'editor');

    // Update role to viewer
    await updateDatabaseUserRole(page, dbName, editorUser.username, 'viewer');

    // Verify update
    await page.click('button:has-text("管理用户")');
    await expect(page.locator('text=viewer')).toBeVisible();
  });

  test('owner can remove user from database', async ({ page }) => {
    const dbName = 'DB Removal Test';
    await createDatabase(page, dbName);

    const editorUser = await registerUser(page, 'editor');
    await login(page, TEST_USERS.owner);

    await shareDatabase(page, dbName, editorUser.username, 'editor');

    // Remove user
    await removeDatabaseUser(page, dbName, editorUser.username);

    // Verify removal
    await page.click('button:has-text("管理用户")');
    await expect(page.locator('text=' + editorUser.username)).not.toBeVisible();
  });

  test('admin can share database with other users', async ({ page }) => {
    const dbName = 'Admin Shared DB';
    await createDatabase(page, dbName);

    const adminUser = await registerUser(page, 'admin');
    await login(page, TEST_USERS.owner);
    await shareDatabase(page, dbName, adminUser.username, 'admin');

    // Login as admin
    await logout(page);
    await login(page, TEST_USERS.admin);

    // Share with another user
    const editorUser = await registerUser(page, 'editor');
    await logout(page);
    await login(page, TEST_USERS.admin);

    await shareDatabase(page, dbName, editorUser.username, 'editor');

    // Verify
    await page.click('button:has-text("管理用户")');
    await expect(page.locator('text=' + editorUser.username)).toBeVisible();
  });

  test('editor cannot share database', async ({ page }) => {
    const dbName = 'Editor No Share DB';
    await createDatabase(page, dbName);

    const editorUser = await registerUser(page, 'editor');
    await login(page, TEST_USERS.owner);
    await shareDatabase(page, dbName, editorUser.username, 'editor');

    // Login as editor
    await logout(page);
    await login(page, TEST_USERS.editor);

    // Navigate to database
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: dbName });

    // Should not see share button
    await expect(dbRow.locator('button:has-text("分享")')).not.toBeVisible();
  });

  test('viewer cannot share database', async ({ page }) => {
    const dbName = 'Viewer No Share DB';
    await createDatabase(page, dbName);

    const viewerUser = await registerUser(page, 'viewer');
    await login(page, TEST_USERS.owner);
    await shareDatabase(page, dbName, viewerUser.username, 'viewer');

    // Login as viewer
    await logout(page);
    await login(page, TEST_USERS.viewer);

    // Navigate to database
    await page.goto('/databases');
    const dbRow = await page.locator('table tbody tr').filter({ hasText: dbName });

    // Should not see share button
    await expect(dbRow.locator('button:has-text("分享")')).not.toBeVisible();
  });
});