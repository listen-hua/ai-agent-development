import { describe, expect, it, vi } from 'vitest'
import { createClientUUID, createSecureClientUUID, type BrowserCrypto } from './clientId'

describe('client UUID compatibility', () => {
  it('uses native randomUUID when available', () => {
    const randomUUID = vi.fn(() => 'native-uuid')
    expect(createClientUUID({ randomUUID })).toBe('native-uuid')
    expect(randomUUID).toHaveBeenCalledOnce()
  })

  it('uses getRandomValues when randomUUID is unavailable', () => {
    const provider: BrowserCrypto = {
      getRandomValues(array) {
        array.fill(0xab)
        return array
      },
    }

    expect(createClientUUID(provider)).toBe('abababab-abab-4bab-abab-abababababab')
  })

  it('keeps a UUID shape in legacy clients without Web Crypto', () => {
    const first = createClientUUID(null)
    const second = createClientUUID(null)

    expect(first).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
    expect(second).not.toBe(first)
  })

  it('does not use the insecure fallback for authentication state', () => {
    expect(() => createSecureClientUUID(null)).toThrow('当前浏览器不支持安全随机数')
  })
})
