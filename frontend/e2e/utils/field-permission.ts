import { Page, expect } from '@playwright/test';
import { findDatabaseRow } from './database';

/**
 * Field permission utilities for testing
 */

/**
 * Create a table with fields
 */
export async function createTableWithFields(page: Page, dbName: string, tableName: string, fields: Array<{name: string, type: string}>) {
  // Navigate to database
  await page.goto('/databases');
  const dbRow = await findDatabaseRow(page, dbName);
  if (!dbRow) {
    throw new Error(`Database ${dbName} not found`);
  }
  await dbRow.click('button:has-text("查看")');
  await page.waitForURL(/\/databases\/.+/);

  // Create table
  await page.click('button:has-text("新建表")');
  await expect(page.locator('.el-dialog')).toBeVisible();
  await page.fill('input[placeholder="表名称"]', tableName);
  await page.click('.el-dialog__footer button:has-text("确定")');
  await expect(page.locator('.el-dialog')).not.toBeVisible();

  // Go to table fields
  const tableRow = await findTableRow(page, tableName);
  if (!tableRow) {
    throw new Error(`Table ${tableName} not found`);
  }
  await tableRow.click('button:has-text("字段管理")');
  await page.waitForURL(/\/tables\/.+\/fields/);

  // Add fields
  for (const field of fields) {
    await page.click('button:has-text("新建字段")');
    await expect(page.locator('.el-dialog')).toBeVisible();
    await page.fill('input[placeholder="字段名称"]', field.name);
    await page.click(`.el-select:has-text("字段类型")`);
    await page.click(`.el-select-dropdown__item:has-text("${field.type}")`);
    await page.click('.el-dialog__footer button:has-text("确定")');
    await expect(page.locator('.el-dialog')).not.toBeVisible();
    await page.waitForTimeout(200);
  }

  return tableName;
}

/**
 * Navigate to field permissions page
 */
export async function navigateToFieldPermissions(page: Page, tableName: string) {
  // Find table row and click field permissions
  const tableRow = await findTableRow(page, tableName);
  if (!tableRow) {
    throw new Error(`Table ${tableName} not found`);
  }
  await tableRow.click('button:has-text("权限配置")');
  await page.waitForURL(/\/tables\/.+\/field-permissions/);
}

/**
 * Set field permissions for a specific role
 */
export async function setFieldPermission(page: Page, tableName: string, fieldName: string, role: string, permissions: {read?: boolean, write?: boolean, delete?: boolean}) {
  await navigateToFieldPermissions(page, tableName);

  // Find the row for the field
  const rows = page.locator('table tbody tr');
  const count = await rows.count();

  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const text = await row.textContent();
    if (text?.includes(fieldName)) {
      // Find the role column
      const roleColumns = row.locator('.permission-checkboxes');
      const roleIndex = await getRoleColumnIndex(page, role);

      // Set permissions
      if (permissions.read !== undefined) {
        const checkbox = roleColumns.nth(roleIndex).locator('input[type="checkbox"]').first();
        if (permissions.read) {
          await checkbox.check();
        } else {
          await checkbox.uncheck();
        }
      }

      if (permissions.write !== undefined) {
        const checkbox = roleColumns.nth(roleIndex).locator('input[type="checkbox"]').nth(1);
        if (permissions.write) {
          await checkbox.check();
        } else {
          await checkbox.uncheck();
        }
      }

      if (permissions.delete !== undefined) {
        const checkbox = roleColumns.nth(roleIndex).locator('input[type="checkbox"]').nth(2);
        if (permissions.delete) {
          await checkbox.check();
        } else {
          await checkbox.uncheck();
        }
      }

      break;
    }
  }

  // Save permissions
  await page.click('button:has-text("保存配置")');
  await page.waitForTimeout(500);
}

/**
 * Apply permission template
 */
export async function applyPermissionTemplate(page: Page, tableName: string, template: string) {
  await navigateToFieldPermissions(page, tableName);

  // Click template dropdown
  await page.click('button:has-text("应用模板")');
  await page.click(`.el-dropdown-menu__item:has-text("${template}")`);

  // Save
  await page.click('button:has-text("保存配置")');
  await page.waitForTimeout(500);
}

/**
 * Batch select permissions
 */
export async function batchSelectPermissions(page: Page, tableName: string, permissionType: '读取' | '写入' | '删除') {
  await navigateToFieldPermissions(page, tableName);

  const buttonMap = {
    '读取': '全选读取',
    '写入': '全选写入',
    '删除': '全选删除'
  };

  await page.click(`button:has-text("${buttonMap[permissionType]}")`);
  await page.click('button:has-text("保存配置")');
  await page.waitForTimeout(500);
}

/**
 * Helper to find table row
 */
async function findTableRow(page: Page, tableName: string) {
  const rows = page.locator('table tbody tr');
  const count = await rows.count();

  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const text = await row.textContent();
    if (text?.includes(tableName)) {
      return row;
    }
  }

  return null;
}

/**
 * Helper to get role column index
 */
async function getRoleColumnIndex(page: Page, role: string) {
  const headers = page.locator('table thead th');
  const count = await headers.count();

  for (let i = 0; i < count; i++) {
    const text = await headers.nth(i).textContent();
    if (text?.toLowerCase().includes(role.toLowerCase())) {
      return i - 2; // Subtract 2 for "字段名称" and "字段类型" columns
    }
  }

  return 0;
}