import { expect, test, type Page } from '@playwright/test'

type GovernanceMockState = ReturnType<typeof createGovernanceMockState>

function json(body: unknown) {
  return {
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
  }
}

function createGovernanceMockState() {
  const user = {
    id: 'usr_reviewer_1',
    username: 'governance.reviewer',
    email: 'reviewer@example.com',
  }

  const users = [
    user,
    {
      id: 'usr_owner_1',
      username: 'task.owner',
      email: 'owner@example.com',
    },
  ]

  const task = {
    id: 'gvt_term_review_1',
    title: '处理 panel_id 术语归一',
    description: '需要确认 panel_id 字段的术语绑定与分类建议。',
    task_type: 'term_review',
    status: 'in_review',
    priority: 'high',
    source_system: 'fuckcmdb',
    resource_type: 'column',
    resource_id: 'col_panel_id',
    assignee_id: user.id,
    created_by: 'system:integration',
    created_at: '2026-03-24T08:00:00Z',
    updated_at: '2026-03-24T08:00:00Z',
    due_at: '2026-03-26T08:00:00Z',
  }

  const reviews = [
    {
      id: 'gvr_term_binding_1',
      task_id: task.id,
      review_type: 'term_binding',
      status: 'approved',
      proposal_source: 'llm-governor',
      proposal_payload: JSON.stringify({
        summary: '确认 panel_id 的术语绑定建议',
        recommendation_type: 'term_binding',
        resource_type: 'column',
        resource_id: 'col_panel_id',
        candidate_term: 'Panel Identifier',
        reason: '命名与现有术语表一致',
      }),
      decision_payload: JSON.stringify({
        decision: 'approved',
        note: '确认建议可用',
      }),
      apply_status: 'failed',
      apply_error: 'timeout waiting for downstream',
      apply_result: '',
      apply_target: 'fuckcmdb',
      reviewer_id: user.id,
      created_by: 'usr_owner_1',
      reviewed_at: '2026-03-24T08:30:00Z',
      applied_at: '',
      created_at: '2026-03-24T08:10:00Z',
      updated_at: '2026-03-24T08:30:00Z',
    },
  ]

  const externalLinks = [
    {
      id: 'gxl_panel_1',
      task_id: task.id,
      source_system: 'fuckcmdb',
      resource_type: 'column',
      resource_id: 'col_panel_id',
      display_name: 'panel_id',
      target_url: 'https://cmdb.example.com/columns/col_panel_id',
      created_at: '2026-03-24T08:00:00Z',
    },
  ]

  const comments = [
    {
      id: 'gcm_seed_1',
      task_id: task.id,
      content: '等待审核人确认提案后回写。',
      created_by: 'system:integration',
      created_at: '2026-03-24T08:05:00Z',
    },
  ]

  return {
    user,
    users,
    task,
    reviews,
    externalLinks,
    comments,
    applyCalls: 0,
    updateCalls: 0,
  }
}

async function installGovernanceMocks(page: Page, state: GovernanceMockState) {
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const key = `${request.method()} ${url.pathname}`

    switch (key) {
      case 'POST /api/auth/login':
        await route.fulfill(
          json({
            success: true,
            message: 'ok',
            data: {
              token: 'mock-token',
            },
          }),
        )
        return

      case 'GET /api/users/me':
        await route.fulfill(
          json({
            success: true,
            message: 'ok',
            data: state.user,
          }),
        )
        return

      case 'GET /api/users':
        await route.fulfill(
          json({
            success: true,
            message: 'ok',
            data: {
              users: state.users,
            },
          }),
        )
        return

      case 'GET /api/governance/tasks':
        await route.fulfill(
          json({
            success: true,
            message: 'ok',
            data: {
              tasks: [state.task],
              total: 1,
            },
          }),
        )
        return

      case `GET /api/governance/tasks/${state.task.id}`:
        await route.fulfill(
          json({
            success: true,
            message: 'ok',
            data: {
              task: state.task,
              reviews: state.reviews,
              evidences: [],
              comments: state.comments,
              external_links: state.externalLinks,
            },
          }),
        )
        return

      case `PUT /api/governance/tasks/${state.task.id}`: {
        state.updateCalls += 1
        const payload = JSON.parse(request.postData() || '{}') as Record<string, string | null>
        state.task = {
          ...state.task,
          title: String(payload.title ?? state.task.title),
          description: String(payload.description ?? state.task.description),
          status: String(payload.status ?? state.task.status),
          priority: String(payload.priority ?? state.task.priority),
          assignee_id: String(payload.assignee_id ?? state.task.assignee_id),
          due_at: payload.due_at ? String(payload.due_at) : state.task.due_at,
          updated_at: '2026-03-24T09:00:00Z',
        }

        await route.fulfill(
          json({
            success: true,
            message: 'ok',
            data: state.task,
          }),
        )
        return
      }

      case `POST /api/governance/reviews/${state.reviews[0].id}/apply`:
        state.applyCalls += 1
        state.task = {
          ...state.task,
          status: 'done',
          updated_at: '2026-03-24T09:10:00Z',
        }
        state.reviews = state.reviews.map((review) =>
          review.id === state.reviews[0].id
            ? {
                ...review,
                apply_status: 'succeeded',
                apply_error: '',
                apply_result: JSON.stringify({ status: 'ok' }),
                applied_at: '2026-03-24T09:10:00Z',
                updated_at: '2026-03-24T09:10:00Z',
              }
            : review,
        )
        state.comments = [
          ...state.comments,
          {
            id: 'gcm_apply_1',
            task_id: state.task.id,
            content: '治理审核已成功回写到 fuckcmdb，响应：{"status":"ok"}',
            created_by: 'system:outbox',
            created_at: '2026-03-24T09:10:00Z',
          },
        ]

        await route.fulfill(
          json({
            success: true,
            message: 'ok',
            data: {
              id: 'gox_apply_1',
              status: 'succeeded',
            },
          }),
        )
        return

      default:
        await route.fulfill(
          json({
            success: false,
            message: `Unexpected mocked request: ${key}`,
            data: null,
          }),
        )
    }
  })
}

async function loginToGovernance(page: Page) {
  await page.goto('/login?redirect=%2Fgovernance')
  await page.getByPlaceholder('请输入用户名').fill('governance.reviewer')
  await page.getByPlaceholder('请输入密码').fill('password123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL(/\/governance$/)
  await expect(page.getByTestId('governance-page')).toBeVisible()
}

test.describe('Governance UI regression', () => {
  test('renders task detail and review templates from live governance data', async ({ page }) => {
    const state = createGovernanceMockState()
    await installGovernanceMocks(page, state)

    await loginToGovernance(page)

    await expect(page.getByText(state.task.title)).toBeVisible()
    await expect(page.getByText('术语审核').first()).toBeVisible()

    await page.getByTestId('governance-detail-button').click()
    await expect(page.getByText('外部资源引用')).toBeVisible()

    const externalLink = page.getByTestId('governance-external-link')
    await expect(externalLink).toHaveAttribute('href', state.externalLinks[0].target_url)
    await expect(externalLink).toHaveText('panel_id')

    await page.getByTestId('governance-open-review-dialog').click()

    const reviewJSON = page.getByRole('textbox', { name: '提案 JSON' })
    await expect(reviewJSON).toHaveValue(/"recommendation_type": "term_binding"/)
    await expect(reviewJSON).toHaveValue(/"resource_id": "col_panel_id"/)

    await page.getByTestId('governance-template-classification').click()
    await expect(reviewJSON).toHaveValue(/"recommendation_type": "classification"/)
    await expect(reviewJSON).toHaveValue(/"sensitivity_level": ""/)

    await page.getByTestId('governance-template-generic').click()
    await expect(reviewJSON).toHaveValue(/"recommendation_type": "generic"/)
    await expect(reviewJSON).toHaveValue(/"summary": "处理 panel_id 术语归一"/)
  })

  test('retries approved review apply and refreshes governance detail state', async ({ page }) => {
    const state = createGovernanceMockState()
    await installGovernanceMocks(page, state)

    await loginToGovernance(page)

    await page.getByTestId('governance-detail-button').click()
    await expect(page.getByTestId('governance-apply-review-button')).toHaveText('重试回写')

    await page.getByTestId('governance-apply-review-button').click()

    await expect.poll(() => state.applyCalls).toBe(1)
    await expect(page.getByText('已回写').first()).toBeVisible()
    await expect(page.getByText(/治理审核已成功回写到 fuckcmdb/)).toBeVisible()
    await expect(page.locator('.el-table').getByText('已完成').first()).toBeVisible()
  })
})
