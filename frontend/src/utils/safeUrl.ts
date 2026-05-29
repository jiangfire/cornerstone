// safeUrl.ts
// 外部注入的 URL 字段可能包含恶意协议,前端在 `:href` /
// `window.open` 前必须过 isSafeHttpUrl 检查,阻断 `javascript:` / `data:` /
// `vbscript:` 等 XSS 注入向量。

const SAFE_PROTOCOLS = new Set(['http:', 'https:'])

/**
 * 判断字符串是否为可安全嵌入 <a href> / window.open 的 http(s) URL。
 *
 * 规则:
 *  - 必须能通过 URL 构造器解析
 *  - 协议必须为 http: 或 https:
 *  - 拒绝 `javascript:` / `data:` / `vbscript:` / `file:` 等
 */
export function isSafeHttpUrl(raw: string | null | undefined): boolean {
  if (!raw || typeof raw !== 'string') {
    return false
  }
  const trimmed = raw.trim()
  if (!trimmed) {
    return false
  }
  // 显式过滤一遍危险伪协议,避免被某些边角字符串绕开 URL 构造器
  if (/^\s*(javascript|data|vbscript|file):/i.test(trimmed)) {
    return false
  }
  try {
    const parsed = new URL(trimmed)
    return SAFE_PROTOCOLS.has(parsed.protocol)
  } catch {
    return false
  }
}

/**
 * 安全场景下返回 raw,否则返回 null。模板里可用 `safeHttpUrl(url) ?? '#'` 拼底色。
 */
export function safeHttpUrl(raw: string | null | undefined): string | null {
  return isSafeHttpUrl(raw) ? (raw as string).trim() : null
}
