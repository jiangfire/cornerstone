import { Page, expect } from '@playwright/test';

/**
 * Organization management utilities for testing
 */

/**
 * Create a new organization
 */
export async function createOrganization(page: Page, name: string, description?: string) {
  await page.goto('/organizations');

  // Click "新建组织" button
  await page.click('button:has-text("新建组织")');

  // Wait for dialog to appear
  await expect(page.locator('.el-dialog')).toBeVisible();

  // Fill form
  await page.fill('input[placeholder="组织名称"]', name);
  if (description) {
    await page.fill('textarea[placeholder="描述"]', description);
  }

  // Click confirm button
  await page.click('.el-dialog__footer button:has-text("确定")');

  // Wait for dialog to close and success message
  await expect(page.locator('.el-dialog')).not.toBeVisible();
  await page.waitForTimeout(500); // Brief wait for UI update

  return name;
}

/**
 * Find organization by name in the list
 */
export async function findOrganizationRow(page: Page, name: string) {
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
 * Add member to organization
 */
export async function addOrganizationMember(page: Page, orgName: string, username: string, role: string) {
  // Navigate to organization page first
  await page.goto('/organizations');

  const orgRow = await findOrganizationRow(page, orgName);
  if (!orgRow) {
    throw new Error(`Organization ${orgName} not found`);
  }

  // Click "查看" or "编辑" to go to organization detail
  await orgRow.click('button:has-text("查看")');
  await page.waitForURL(/\/organizations\/.+/);

  // Find and click member management section/tab
  await page.click('text=成员管理');

  // Click "添加成员" button
  await page.click('button:has-text("添加成员")');

  // Wait for dialog
  await expect(page.locator('.el-dialog')).toBeVisible();

  // Fill member form
  await page.fill('input[placeholder*="用户名"]', username);
  await page.click(`.el-select:has-text("角色")`);
  await page.click(`.el-select-dropdown__item:has-text("${role}")`);

  // Confirm
  await page.click('.el-dialog__footer button:has-text("确定")');

  // Wait for success
  await page.waitForTimeout(500);
}

/**
 * Update organization member role
 */
export async function updateOrganizationMemberRole(page: Page, orgName: string, username: string, newRole: string) {
  await page.goto('/organizations');

  const orgRow = await findOrganizationRow(page, orgName);
  if (!orgRow) {
    throw new Error(`Organization ${orgName} not found`);
  }

  await orgRow.click('button:has-text("查看")');
  await page.waitForURL(/\/organizations\/.+/);
  await page.click('text=成员管理');

  // Find member row
  const memberRows = page.locator('table tbody tr');
  const count = await memberRows.count();

  for (let i = 0; i < count; i++) {
    const row = memberRows.nth(i);
    const text = await row.textContent();
    if (text?.includes(username)) {
      // Click role dropdown
      await row.click('.el-select');
      await page.click(`.el-select-dropdown__item:has-text("${newRole}")`);
      break;
    }
  }
}

/**
 * Remove member from organization
 */
export async function removeOrganizationMember(page: Page, orgName: string, username: string) {
  await page.goto('/organizations');

  const orgRow = await findOrganizationRow(page, orgName);
  if (!orgRow) {
    throw new Error(`Organization ${orgName} not found`);
  }

  await orgRow.click('button:has-text("查看")');
  await page.waitForURL(/\/organizations\/.+/);
  await page.click('text=成员管理');

  // Find member row and click delete
  const memberRows = page.locator('table tbody tr');
  const count = await memberRows.count();

  for (let i = 0; i < count; i++) {
    const row = memberRows.nth(i);
    const text = await row.textContent();
    if (text?.includes(username)) {
      await row.click('button:has-text("删除")');
      // Confirm deletion
      await page.click('button:has-text("确定")');
      break;
    }
  }
}