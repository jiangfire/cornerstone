import { describe, expect, it } from 'vitest'
import { isSafeHttpUrl, safeHttpUrl } from '../safeUrl'

describe('safeUrl', () => {
  describe('isSafeHttpUrl', () => {
    it('accepts http and https URLs', () => {
      expect(isSafeHttpUrl('http://example.com')).toBe(true)
      expect(isSafeHttpUrl('https://example.com/foo?bar=1')).toBe(true)
      expect(isSafeHttpUrl('https://sub.example.co.uk:8443/path#frag')).toBe(true)
    })

    it('rejects javascript: and other dangerous pseudo-protocols', () => {
      expect(isSafeHttpUrl('javascript:alert(1)')).toBe(false)
      expect(isSafeHttpUrl('JAVASCRIPT:alert(1)')).toBe(false)
      expect(isSafeHttpUrl(' javascript:alert(1)')).toBe(false)
      expect(isSafeHttpUrl('data:text/html,<script>alert(1)</script>')).toBe(false)
      expect(isSafeHttpUrl('vbscript:msgbox(1)')).toBe(false)
      expect(isSafeHttpUrl('file:///etc/passwd')).toBe(false)
    })

    it('rejects empty / nullish / non-string inputs', () => {
      expect(isSafeHttpUrl('')).toBe(false)
      expect(isSafeHttpUrl('   ')).toBe(false)
      expect(isSafeHttpUrl(null)).toBe(false)
      expect(isSafeHttpUrl(undefined)).toBe(false)
    })

    it('rejects malformed URLs', () => {
      expect(isSafeHttpUrl('not a url')).toBe(false)
      expect(isSafeHttpUrl('://broken')).toBe(false)
      expect(isSafeHttpUrl('//no-protocol.com')).toBe(false)
    })
  })

  describe('safeHttpUrl', () => {
    it('returns the trimmed URL when safe', () => {
      expect(safeHttpUrl('  https://example.com  ')).toBe('https://example.com')
    })

    it('returns null when unsafe', () => {
      expect(safeHttpUrl('javascript:alert(1)')).toBeNull()
      expect(safeHttpUrl('')).toBeNull()
      expect(safeHttpUrl(null)).toBeNull()
    })
  })
})
