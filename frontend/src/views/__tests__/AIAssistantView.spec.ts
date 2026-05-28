import { describe, expect, it, vi, beforeEach } from 'vitest'

import { flushUi, mountView, setInputValue } from '@/test-utils/view-test-helpers'
import AIAssistantView from '@/views/AIAssistantView.vue'

const { chatMock } = vi.hoisted(() => ({
  chatMock: vi.fn(),
}))

vi.mock('@/services/api', () => ({
  aiAPI: {
    chat: chatMock,
  },
}))

describe('AIAssistantView', () => {
  beforeEach(() => {
    chatMock.mockReset()
  })

  it('renders a successful AI reply from the API response payload', async () => {
    chatMock.mockResolvedValue({
      code: 0,
      data: {
        reply: '已创建 orders 表',
      },
    })

    const wrapper = mountView(AIAssistantView)
    const textarea = wrapper.get('textarea').element as HTMLTextAreaElement
    await setInputValue(textarea, '创建订单表')

    await wrapper.get('button').trigger('click')
    await flushUi()

    expect(chatMock).toHaveBeenCalledWith('创建订单表')
    expect(wrapper.text()).toContain('已创建 orders 表')
  })
})
