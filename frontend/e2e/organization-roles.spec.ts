import { test, expect } from '@playwright/test';
import { login, logout, registerUser, TEST_USERS } from './utils/auth';
import { createOrganization, addOrganizationMember, updateOrganizationMemberRole, removeOrganizationMember } from './utils/organization';

test.describe('Organization Role Management', () => {
  test.beforeEach(async ({ page }) => {
    // Login as owner before each test
    await login(page, TEST_USERS.owner);
  });

  test('owner can create organization and become owner', async ({ page }) => {
    const orgName = 'Test Organization ' + Date.now();

    await createOrganization(page, orgName);

    // Verify organization appears in list
    await expect(page.locator('text=' + orgName)).toBeVisible();
    await expect(page.locator('text=owner')).toBeVisible();
  });

  test('owner can add admin member to organization', async ({ page }) => {
    const orgName = 'Test Org for Admin';
    await createOrganization(page, orgName);

    // Register admin user first
    const adminUser = await registerUser(page, 'admin');
    await login(page, TEST_USERS.owner);

    // Add admin member
    await addOrganizationMember(page, orgName, adminUser.username, 'admin');

    // Verify member appears
    await page.click('text=成员管理');
    await expect(page.locator('text=' + adminUser.username)).toBeVisible();
    await expect(page.locator('text=admin')).toBeVisible();
  });

  test('owner can add editor member to organization', async ({ page }) => {
    const orgName = 'Test Org for Editor';
    await createOrganization(page, orgName);

    // Register editor user first
    const editorUser = await registerUser(page, 'editor');
    await login(page, TEST_USERS.owner);

    // Add editor member
    await addOrganizationMember(page, orgName, editorUser.username, 'editor');

    // Verify member appears
    await page.click('text=成员管理');
    await expect(page.locator('text=' + editorUser.username)).toBeVisible();
    await expect(page.locator('text=editor')).toBeVisible();
  });

  test('owner can add viewer member to organization', async ({ page }) => {
    const orgName = 'Test Org for Viewer';
    await createOrganization(page, orgName);

    // Register viewer user first
    const viewerUser = await registerUser(page, 'viewer');
    await login(page, TEST_USERS.owner);

    // Add viewer member
    await addOrganizationMember(page, orgName, viewerUser.username, 'viewer');

    // Verify member appears
    await page.click('text=成员管理');
    await expect(page.locator('text=' + viewerUser.username)).toBeVisible();
    await expect(page.locator('text=viewer')).toBeVisible();
  });

  test('owner can update member role', async ({ page }) => {
    const orgName = 'Test Org for Role Update';
    await createOrganization(page, orgName);

    const editorUser = await registerUser(page, 'editor');
    await login(page, TEST_USERS.owner);

    await addOrganizationMember(page, orgName, editorUser.username, 'editor');

    // Update role to viewer
    await updateOrganizationMemberRole(page, orgName, editorUser.username, 'viewer');

    // Verify role updated
    await page.click('text=成员管理');
    await expect(page.locator('text=' + editorUser.username)).toBeVisible();
    await expect(page.locator('text=viewer')).toBeVisible();
  });

  test('owner can remove member from organization', async ({ page }) => {
    const orgName = 'Test Org for Removal';
    await createOrganization(page, orgName);

    const editorUser = await registerUser(page, 'editor');
    await login(page, TEST_USERS.owner);

    await addOrganizationMember(page, orgName, editorUser.username, 'editor');

    // Remove member
    await removeOrganizationMember(page, orgName, editorUser.username);

    // Verify member removed
    await page.click('text=成员管理');
    await expect(page.locator('text=' + editorUser.username)).not.toBeVisible();
  });

  test('admin can add members to organization', async ({ page }) => {
    // Setup: Create org with admin user
    const orgName = 'Admin Test Org';
    await createOrganization(page, orgName);

    const adminUser = await registerUser(page, 'admin');
    await login(page, TEST_USERS.owner);
    await addOrganizationMember(page, orgName, adminUser.username, 'admin');

    // Login as admin
    await logout(page);
    await login(page, TEST_USERS.admin);

    // Add another member as admin
    const editorUser = await registerUser(page, 'editor');
    await logout(page);
    await login(page, TEST_USERS.admin);

    await addOrganizationMember(page, orgName, editorUser.username, 'editor');

    // Verify
    await page.click('text=成员管理');
    await expect(page.locator('text=' + editorUser.username)).toBeVisible();
  });

  test('viewer cannot add members to organization', async ({ page }) => {
    const orgName = 'Viewer Test Org';
    await createOrganization(page, orgName);

    const viewerUser = await registerUser(page, 'viewer');
    await login(page, TEST_USERS.owner);
    await addOrganizationMember(page, orgName, viewerUser.username, 'viewer');

    // Login as viewer
    await logout(page);
    await login(page, TEST_USERS.viewer);

    // Navigate to organization
    await page.goto('/organizations');
    const orgRow = await page.locator('table tbody tr').filter({ hasText: orgName });
    await orgRow.click('button:has-text("查看")');

    // Should not see member management or add member options
    await expect(page.locator('text=成员管理')).not.toBeVisible();
    await expect(page.locator('button:has-text("添加成员")')).not.toBeVisible();
  });
});