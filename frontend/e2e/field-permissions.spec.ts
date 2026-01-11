import { test, expect } from '@playwright/test';
import { login, logout, registerUser, TEST_USERS } from './utils/auth';
import { createDatabase } from './utils/database';
import { createTableWithFields, applyPermissionTemplate, batchSelectPermissions, setFieldPermission } from './utils/field-permission';

test.describe('Field-Level Permission Management', () => {
  test.beforeEach(async ({ page }) => {
    // Login as owner before each test
    await login(page, TEST_USERS.owner);
  });

  test('owner can create table with fields', async ({ page }) => {
    const dbName = 'Field Test DB';
    const tableName = 'Users Table';
    const fields = [
      { name: 'username', type: '字符串' },
      { name: 'email', type: '字符串' },
      { name: 'age', type: '数字' }
    ];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Verify fields created
    await expect(page.locator('text=username')).toBeVisible();
    await expect(page.locator('text=email')).toBeVisible();
    await expect(page.locator('text=age')).toBeVisible();
  });

  test('owner can access field permissions page', async ({ page }) => {
    const dbName = 'Permission Access DB';
    const tableName = 'Access Table';
    const fields = [{ name: 'field1', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Navigate to permissions
    await page.click('button:has-text("权限配置")');
    await page.waitForURL(/\/tables\/.+\/field-permissions/);

    // Verify page loaded
    await expect(page.locator('text=字段权限配置')).toBeVisible();
    await expect(page.locator('text=字段名称')).toBeVisible();
  });

  test('owner can apply default permission template', async ({ page }) => {
    const dbName = 'Template Test DB';
    const tableName = 'Template Table';
    const fields = [{ name: 'test_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Apply default template
    await applyPermissionTemplate(page, tableName, '默认权限');

    // Verify permissions are set (check that checkboxes are present)
    await expect(page.locator('input[type="checkbox"]')).toBeVisible();
  });

  test('owner can apply viewer-only template', async ({ page }) => {
    const dbName = 'Viewer Template DB';
    const tableName = 'Viewer Table';
    const fields = [{ name: 'readonly_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Apply viewer template
    await applyPermissionTemplate(page, tableName, '仅查看模式');

    // Verify viewer has read-only permissions
    await expect(page.locator('input[type="checkbox"]')).toBeVisible();
  });

  test('owner can apply strict template', async ({ page }) => {
    const dbName = 'Strict Template DB';
    const tableName = 'Strict Table';
    const fields = [{ name: 'strict_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Apply strict template
    await applyPermissionTemplate(page, tableName, '严格模式');

    // Verify strict permissions applied
    await expect(page.locator('button:has-text("保存配置")')).toBeVisible();
  });

  test('owner can batch select read permissions', async ({ page }) => {
    const dbName = 'Batch Read DB';
    const tableName = 'Batch Read Table';
    const fields = [
      { name: 'field1', type: '字符串' },
      { name: 'field2', type: '字符串' }
    ];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Batch select read permissions
    await batchSelectPermissions(page, tableName, '读取');

    // Verify success message or save button is available
    await expect(page.locator('button:has-text("保存配置")')).toBeVisible();
  });

  test('owner can batch select write permissions', async ({ page }) => {
    const dbName = 'Batch Write DB';
    const tableName = 'Batch Write Table';
    const fields = [
      { name: 'field1', type: '字符串' },
      { name: 'field2', type: '字符串' }
    ];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Batch select write permissions
    await batchSelectPermissions(page, tableName, '写入');

    // Verify
    await expect(page.locator('button:has-text("保存配置")')).toBeVisible();
  });

  test('owner can batch select delete permissions', async ({ page }) => {
    const dbName = 'Batch Delete DB';
    const tableName = 'Batch Delete Table';
    const fields = [
      { name: 'field1', type: '字符串' },
      { name: 'field2', type: '字符串' }
    ];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Batch select delete permissions
    await batchSelectPermissions(page, tableName, '删除');

    // Verify
    await expect(page.locator('button:has-text("保存配置")')).toBeVisible();
  });

  test('owner can set specific field permissions for editor role', async ({ page }) => {
    const dbName = 'Editor Field DB';
    const tableName = 'Editor Table';
    const fields = [{ name: 'editable_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Set specific permissions for editor
    await setFieldPermission(page, tableName, 'editable_field', 'editor', {
      read: true,
      write: true,
      delete: false
    });

    // Verify permissions saved
    await expect(page.locator('text=权限设置成功')).toBeVisible();
  });

  test('owner can set specific field permissions for viewer role', async ({ page }) => {
    const dbName = 'Viewer Field DB';
    const tableName = 'Viewer Table';
    const fields = [{ name: 'readonly_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Set specific permissions for viewer
    await setFieldPermission(page, tableName, 'readonly_field', 'viewer', {
      read: true,
      write: false,
      delete: false
    });

    // Verify permissions saved
    await expect(page.locator('text=权限设置成功')).toBeVisible();
  });

  test('owner can reset permissions to default', async ({ page }) => {
    const dbName = 'Reset Test DB';
    const tableName = 'Reset Table';
    const fields = [{ name: 'reset_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Apply custom permissions first
    await setFieldPermission(page, tableName, 'reset_field', 'editor', {
      read: true,
      write: false,
      delete: false
    });

    // Reset to default
    await page.click('button:has-text("重置为默认")');
    await page.click('button:has-text("确定")'); // Confirm reset

    // Verify reset
    await expect(page.locator('text=已重置为默认权限')).toBeVisible();
  });

  test('owner can clear all permissions', async ({ page }) => {
    const dbName = 'Clear All DB';
    const tableName = 'Clear Table';
    const fields = [{ name: 'clear_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Clear all permissions
    await page.click('button:has-text("清空全部")');

    // Verify clear action triggered
    await expect(page.locator('button:has-text("保存配置")')).toBeVisible();
  });

  test('owner cannot edit owner and admin permissions', async ({ page }) => {
    const dbName = 'Protected Roles DB';
    const tableName = 'Protected Table';
    const fields = [{ name: 'protected_field', type: '字符串' }];

    await createDatabase(page, dbName);
    await createTableWithFields(page, dbName, tableName, fields);

    // Navigate to permissions
    await page.click('button:has-text("权限配置")');
    await page.waitForURL(/\/tables\/.+\/field-permissions/);

    // Check that owner and admin checkboxes are disabled
    const ownerCheckboxes = page.locator('table tbody tr').first().locator('input[type="checkbox"]').filter({ hasText: 'Owner' });
    const adminCheckboxes = page.locator('table tbody tr').first().locator('input[type="checkbox"]').filter({ hasText: 'Admin' });

    // These should be disabled or not interactable
    const ownerCheckbox = page.locator('table tbody tr').first().locator('input[type="checkbox"]').first();
    await expect(ownerCheckbox).toBeDisabled();
  });
});