import { defineComponent, h, nextTick, type Component } from 'vue'
import { mount, flushPromises, type MountingOptions, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { vi } from 'vitest'

let browserMocksInstalled = false

const DialogStub = defineComponent({
  name: 'ElDialog',
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '',
    },
  },
  setup(props, { slots }) {
    return () => {
      if (!props.modelValue) {
        return null
      }

      return h('div', { class: 'el-dialog-stub' }, [
        props.title ? h('div', { class: 'el-dialog-stub__title' }, props.title) : null,
        h('div', { class: 'el-dialog-stub__body' }, slots.default?.()),
        slots.footer ? h('div', { class: 'el-dialog-stub__footer' }, slots.footer()) : null,
      ])
    }
  },
})

export function installBrowserMocks() {
  if (browserMocksInstalled) {
    return
  }

  class ResizeObserverMock {
    observe() {}
    unobserve() {}
    disconnect() {}
  }

  vi.stubGlobal('ResizeObserver', ResizeObserverMock)
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  )

  window.scrollTo = vi.fn()
  browserMocksInstalled = true
}

export function mountView(
  component: Component,
  options: MountingOptions<unknown> = {},
): VueWrapper<unknown> {
  installBrowserMocks()

  return mount(component, {
    attachTo: document.body,
    ...options,
    global: {
      plugins: [ElementPlus],
      stubs: {
        ElDialog: DialogStub,
        teleport: true,
        transition: false,
      },
      ...options.global,
    },
  })
}

export async function flushUi() {
  await flushPromises()
  await nextTick()
  await nextTick()
}

const normalizeText = (value: string | null | undefined) => value?.replace(/\s+/g, ' ').trim() || ''

export async function clickButtonByText(text: string, index = 0) {
  await flushUi()
  const buttons = Array.from(document.body.querySelectorAll('button')).filter((button) =>
    button.textContent?.includes(text),
  )
  const button = buttons[index]
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  button.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  await flushUi()
}

export async function clickExactButtonByText(text: string, index = 0) {
  await flushUi()
  const buttons = Array.from(document.body.querySelectorAll('button')).filter(
    (button) => normalizeText(button.textContent) === text,
  )
  const button = buttons[index]
  if (!button) {
    throw new Error(`Exact button not found: ${text}`)
  }
  button.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  await flushUi()
}

export function getInputByPlaceholder(placeholder: string): HTMLInputElement {
  const input = Array.from(document.body.querySelectorAll('input')).find(
    (element) => element.getAttribute('placeholder') === placeholder,
  )
  if (!(input instanceof HTMLInputElement)) {
    throw new Error(`Input not found: ${placeholder}`)
  }
  return input
}

export function getTextareaByPlaceholder(placeholder: string): HTMLTextAreaElement {
  const textarea = Array.from(document.body.querySelectorAll('textarea')).find(
    (element) => element.getAttribute('placeholder') === placeholder,
  )
  if (!(textarea instanceof HTMLTextAreaElement)) {
    throw new Error(`Textarea not found: ${placeholder}`)
  }
  return textarea
}

export async function setInputValue(element: HTMLInputElement | HTMLTextAreaElement, value: string) {
  element.value = value
  element.dispatchEvent(new Event('input', { bubbles: true }))
  element.dispatchEvent(new Event('change', { bubbles: true }))
  await flushUi()
}

export function getSetupState<T extends object = Record<string, unknown>>(wrapper: VueWrapper<unknown>): T {
  return (wrapper.vm as { $?: { setupState?: T } }).$?.setupState as T
}
