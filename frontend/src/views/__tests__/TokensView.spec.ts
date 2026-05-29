import { beforeEach, describe, expect, it, vi } from 'vitest'

import { flushUi, mountView } from '@/test-utils/view-test-helpers'
import TokensView from '@/views/TokensView.vue'

const { listMock, createMock, deleteMock, setApiKeyMock } = vi.hoisted(() => ({
  listMock: vi.fn(),
  createMock: vi.fn(),
  deleteMock: vi.fn(),
  setApiKeyMock: vi.fn(),
}))

vi.mock('@/services/api', () => ({
  tokenAPI: {
    list: listMock,
    create: createMock,
    delete: deleteMock,
  },
  setApiKey: setApiKeyMock,
}))

describe('TokensView', () => {
  beforeEach(() => {
    listMock.mockReset()
    createMock.mockReset()
    deleteMock.mockReset()
    setApiKeyMock.mockReset()
  })

  it('does not offer copying token IDs from the persisted token list', async () => {
    listMock.mockResolvedValue({
      code: 0,
      data: {
        tokens: [
          {
            id: 'tok_existing',
            name: 'Existing Token',
            is_master: false,
            scopes: '{}',
            created_at: '2026-05-28T00:00:00Z',
          },
        ],
      },
    })

    const wrapper = mountView(TokensView)
    await flushUi()

    expect(wrapper.text()).toContain('Existing Token')
    expect(wrapper.text()).not.toContain('📋')
  })

  it('shows the created token value without silently switching the current API key', async () => {
    listMock.mockResolvedValue({
      code: 0,
      data: {
        tokens: [],
      },
    })
    createMock.mockResolvedValue({
      code: 0,
      data: {
        token: 'cs_new_token',
      },
    })

    const wrapper = mountView(TokensView)
    await flushUi()

    await wrapper.get('button').trigger('click')
    await flushUi()
    await wrapper.get('input[placeholder="令牌名称"]').setValue('Script Token')
    await wrapper.findAll('button').find((button) => button.text().includes('创建'))?.trigger('click')
    await flushUi()

    expect(createMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('cs_new_token')
    expect(setApiKeyMock).not.toHaveBeenCalled()
  })
})
