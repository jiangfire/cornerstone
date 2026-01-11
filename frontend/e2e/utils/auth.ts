import { Page, expect } from '@playwright/test';

// Test user credentials for different roles
export const TEST_USERS = {
  owner: {
    username: 'test_owner',
    email: 'owner@test.com',
    password: 'password123',
  },
  admin: {
    username: 'test_admin',
    email: 'admin@test.com',
    password: 'password123',
  },
  editor: {
    username: 'test_editor',
    email: 'editor@test.com',
    password: 'password123',
  },
  viewer: {
    username: 'test_viewer',
    email: 'viewer@test.com',
    password: 'password123',
  },
};

/**
 * Login with specified user credentials
 */
export async function login(page: Page, user: typeof TEST_USERS[keyof typeof TEST_USERS]) {
  await page.goto('/login');

  // Fill login form
  await page.fill('input[placeholder="用户名"]', user.username);
  await page.fill('input[placeholder="密码"]', user.password);

  // Click login button
  await page.click('button[type="submit"]');

  // Wait for navigation to dashboard
  await page.waitForURL('/');

  // Verify successful login
  await expect(page).toHaveURL('/');
}

/**
 * Logout current user
 */
export async function logout(page: Page) {
  // Click user profile dropdown (assuming it exists)
  try {
    await page.click('[class*="user-dropdown"] or .user-menu or [data-testid="user-menu"]');
    await page.click('text=退出登录');
  } catch (e) {
    // Fallback: clear localStorage and navigate to login
    await page.evaluate(() => localStorage.clear());
    await page.goto('/login');
  }

  await page.waitForURL('/login');
}

/**
 * Register a new user with specified role
 */
export async function registerUser(page: Page, role: string) {
  const user = TEST_USERS[role as keyof typeof TEST_USERS];

  await page.goto('/register');

  // Fill registration form
  await page.fill('input[placeholder="用户名"]', user.username);
  await page.fill('input[placeholder="邮箱"]', user.email);
  await page.fill('input[placeholder="密码"]', user.password);
  await page.fill('input[placeholder="确认密码"]', user.password);

  // Click register button
  await page.click('button[type="submit"]');

  // Wait for navigation to login or dashboard
  await page.waitForURL(/\/(login|dashboard)/);

  return user;
}

/**
 * Ensure user is logged in, login if not
 */
export async function ensureLoggedIn(page: Page, role: keyof typeof TEST_USERS = 'owner') {
  await page.goto('/');

  // Check if we're redirected to login
  const currentUrl = page.url();
  if (currentUrl.includes('/login')) {
    await login(page, TEST_USERS[role]);
  }

  // Verify we're on a protected page
  await expect(page).not.toHaveURL(/\/login/);
}