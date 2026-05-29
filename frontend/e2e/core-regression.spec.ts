import { expect, test, type Page } from '@playwright/test'

function json(body: unknown) {
  return {
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
  }
}

type MockState = ReturnType<typeof createMockState>

function createMockState() {
  return {
    tokens: [
      {
        id: 'tok_existing',
        name: 'Existing Token',
        is_master: false,
        scopes: '{}',
        created_at: '2026-05-28T00:00:00Z',
      },
    ],
    createdToken: 'cs_new_token',
    databases: [
      {
        id: 'db_1',
        name: 'CRM',
        description: 'crm',
        created_at: '',
        updated_at: '',
      },
    ],
    tables: [
      {
        id: 'tbl_1',
        database_id: 'db_1',
        name: 'customers',
        description: '',
        created_at: '',
        updated_at: '',
      },
    ],
    fields: [
      {
        id: 'fld_1',
        table_id: 'tbl_1',
        name: 'name',
        type: 'string',
        description: '',
        required: true,
        options: '',
        created_at: '',
        updated_at: '',
      },
    ],
    records: [
      {
        id: 'rec_1',
        table_id: 'tbl_1',
        data: { name: 'Alice' },
        version: 1,
        created_at: '',
        updated_at: '',
      },
    ],
    aiReply: '已创建 orders 表',
    createTokenCalls: 0,
    chatCalls: 0,
  }
}

async function installCoreMocks(page: Page, state: MockState) {
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const key = `${request.method()} ${url.pathname}`

    switch (key) {
      case 'GET /api/tokens':
        await route.fulfill(
          json({
            code: 0,
            message: 'ok',
            data: {
              tokens: state.tokens,
              total: state.tokens.length,
            },
          }),
        )
        return

      case 'POST /api/tokens':
        state.createTokenCalls += 1
        state.tokens = [
          ...state.tokens,
          {
            id: 'tok_created',
            name: 'Script Token',
            is_master: false,
            scopes: '',
            created_at: '2026-05-28T01:00:00Z',
          },
        ]
        await route.fulfill(
          json({
            code: 0,
            message: 'ok',
            data: {
              id: 'tok_created',
              name: 'Script Token',
              is_master: false,
              scopes: '',
              created_at: '2026-05-28T01:00:00Z',
              token: state.createdToken,
            },
          }),
        )
        return

      case 'POST /api/ai/chat':
        state.chatCalls += 1
        await route.fulfill(
          json({
            code: 0,
            message: 'ok',
            data: {
              type: 'result',
              reply: state.aiReply,
              context: {},
            },
          }),
        )
        return

      case 'GET /api/databases':
        await route.fulfill(
          json({
            code: 0,
            message: 'ok',
            data: {
              databases: state.databases,
              total: state.databases.length,
            },
          }),
        )
        return

      case 'GET /api/databases/db_1/tables':
        await route.fulfill(
          json({
            code: 0,
            message: 'ok',
            data: {
              tables: state.tables,
              total: state.tables.length,
            },
          }),
        )
        return

      case 'GET /api/tables/tbl_1/fields':
        await route.fulfill(
          json({
            code: 0,
            message: 'ok',
            data: {
              items: state.fields,
              total: state.fields.length,
            },
          }),
        )
        return

      case 'GET /api/records':
        await route.fulfill(
          json({
            code: 0,
            message: 'ok',
            data: {
              items: state.records,
              total: state.records.length,
              has_more: false,
            },
          }),
        )
        return

      default:
        await route.fulfill(
          json({
            code: 1,
            message: `Unexpected mocked request: ${key}`,
            data: null,
          }),
        )
    }
  })
}

test.describe('Core UI regression', () => {
  test('AI assistant renders reply from DTO success payload', async ({ page }) => {
    const state = createMockState()
    await installCoreMocks(page, state)

    await page.goto('/ai')
    await page.getByPlaceholder('输入消息... (Enter 发送)').fill('创建订单表')
    await page.getByRole('button', { name: '发送' }).click()

    await expect.poll(() => state.chatCalls).toBe(1)
    await expect(page.getByText('已创建 orders 表')).toBeVisible()
  })

  test('tokens page only shows newly created token secret and does not expose persisted token ids', async ({ page }) => {
    const state = createMockState()
    await installCoreMocks(page, state)

    await page.goto('/tokens')
    await expect(page.getByText('Existing Token')).toBeVisible()
    await expect(page.getByText('tok_existing')).toHaveCount(0)
    await expect(page.getByText('📋')).toHaveCount(0)

    await page.getByRole('button', { name: '新建令牌' }).click()
    await page.getByPlaceholder('令牌名称').fill('Script Token')
    await page.getByRole('button', { name: '创建' }).click()

    await expect.poll(() => state.createTokenCalls).toBe(1)
    await expect(page.getByText('令牌创建成功')).toBeVisible()
    await expect(page.getByText(state.createdToken)).toBeVisible()
    const apiKey = await page.evaluate(() => window.localStorage.getItem('api_key'))
    expect(apiKey).toBeNull()
  })

  test('data browser loads records and shows selected record detail', async ({ page }) => {
    const state = createMockState()
    await installCoreMocks(page, state)

    await page.goto('/')
    await page.locator('.db-list li').filter({ hasText: 'CRM' }).click()
    await page.locator('.table-list li').filter({ hasText: 'customers' }).click()

    await expect(page.locator('.records-table')).toBeVisible()
    await expect(page.getByText('name: Alice')).toBeVisible()

    await page.locator('.records-table tbody tr').first().click()
    await expect(page.locator('.record-detail pre')).toContainText('"name": "Alice"')
  })
})
