import { Page, expect } from '@playwright/test';

/**
 * Database permission utilities for testing
 */

/**
 * Create a new database
 */
export async function createDatabase(page: Page, name: string, description?: string) {
  await page.goto('/databases');

  // Click "新建数据库" button
  await page.click('button:has-text("新建数据库")');

  // Wait for dialog
  await expect(page.locator('.el-dialog')).toBeVisible();

  // Fill form
  await page.fill('input[placeholder="数据库名称"]', name);
  if (description) {
    await page.fill('textarea[placeholder="描述"]', description);
  }

  // Click confirm
  await page.click('.el-dialog__footer button:has-text("确定")');

  // Wait for dialog to close
  await expect(page.locator('.el-dialog')).not.toBeVisible();
  await page.waitForTimeout(500);

  return name;
}

/**
 * Find database row by name
 */
export async function findDatabaseRow(page: Page, name: string) {
  const rows = page.locator('table tbody tr');
  const count = await rows.count();

  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const text = await row.textContent();
    if (text?.includes(name)) {
      return row;
    }
  }

  return null;
}

/**
 * Share database with user and assign role
 */
export async function shareDatabase(page: Page, dbName: string, username: string, role: string) {
  await page.goto('/databases');

  const dbRow = await findDatabaseRow(page, dbName);
  if (!dbRow) {
    throw new Error(`Database ${dbName} not found`);
  }

  // Click "分享" button
  await dbRow.click('button:has-text("分享")');

  // Wait for dialog
  await expect(page.locator('.el-dialog')).toBeVisible();

  // Fill user and select role
  await page.fill('input[placeholder*="用户名"]', username);
  await page.click(`.el-select:has-text("角色")`);
  await page.click(`.el-select-dropdown__item:has-text("${role}")`);

  // Confirm
  await page.click('.el-dialog__footer button:has-text("确定")');

  // Wait for success
  await page.waitForTimeout(500);
}

/**
 * Update database user role
 */
export async function updateDatabaseUserRole(page: Page, dbName: string, username: string, newRole: string) {
  await page.goto('/databases');

  const dbRow = await findDatabaseRow(page, dbName);
  if (!dbRow) {
    throw new Error(`Database ${dbName} not found`);
  }

  // Click "管理用户" or similar
  await dbRow.click('button:has-text("管理用户")');
  await page.waitForURL(/\/databases\/.+/);

  // Find user row
  const userRows = page.locator('table tbody tr');
  const count = await userRows.count();

  for (let i = 0; i < count; i++) {
    const row = userRows.nth(i);
    const text = await row.textContent();
    if (text?.includes(username)) {
      // Update role
      await row.click('.el-select');
      await page.click(`.el-select-dropdown__item:has-text("${newRole}")`);
      break;
    }
  }
}

/**
 * Remove user from database
 */
export async function removeDatabaseUser(page: Page, dbName: string, username: string) {
  await page.goto('/databases');

  const dbRow = await findDatabaseRow(page, dbName);
  if (!dbRow) {
    throw new Error(`Database ${dbName} not found`);
  }

  await dbRow.click('button:has-text("管理用户")');
  await page.waitForURL(/\/databases\/.+/);

  // Find user row and delete
  const userRows = page.locator('table tbody tr');
  const count = await userRows.count();

  for (let i = 0; i < count; i++) {
    const row = userRows.nth(i);
    const text = await row.textContent();
    if (text?.includes(username)) {
      await row.click('button:has-text("删除")');
      await page.click('button:has-text("确定")');
      break;
    }
  }
}