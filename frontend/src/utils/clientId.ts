export interface BrowserCrypto {
  randomUUID?: () => string
  getRandomValues?: (array: Uint8Array) => Uint8Array
}

let fallbackSequence = 0

function browserCrypto(): BrowserCrypto | null {
  return typeof globalThis.crypto === 'undefined' ? null : globalThis.crypto
}

function formatUUID(bytes: Uint8Array): string {
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, '0'))
  return `${hex.slice(0, 4).join('')}-${hex.slice(4, 6).join('')}-${hex.slice(6, 8).join('')}-${hex.slice(8, 10).join('')}-${hex.slice(10).join('')}`
}

function fallbackBytes(): Uint8Array {
  const bytes = new Uint8Array(16)
  const sequence = fallbackSequence++
  const timestamp = Date.now()
  for (let index = 0; index < bytes.length; index++) {
    const timeByte = Math.floor(timestamp / (2 ** ((index % 6) * 8))) & 0xff
    const sequenceByte = (sequence >>> ((index % 4) * 8)) & 0xff
    bytes[index] = Math.floor(Math.random() * 256) ^ timeByte ^ sequenceByte
  }
  return bytes
}

function randomBytes(provider: BrowserCrypto | null): Uint8Array | null {
  if (typeof provider?.getRandomValues !== 'function') return null
  return provider.getRandomValues(new Uint8Array(16))
}

/**
 * Generates a browser-compatible UUID for temporary UI records and idempotency keys.
 * `crypto.randomUUID` is unavailable on non-secure HTTP origins in some WebViews, so
 * the implementation falls back to `getRandomValues` and finally to a uniqueness-only
 * generator for legacy clients.
 */
export function createClientUUID(provider: BrowserCrypto | null = browserCrypto()): string {
  if (typeof provider?.randomUUID === 'function') return provider.randomUUID()
  return formatUUID(randomBytes(provider) || fallbackBytes())
}

/**
 * Generates a cryptographically secure UUID for authentication state.
 * Authentication must never use the uniqueness-only legacy fallback.
 */
export function createSecureClientUUID(provider: BrowserCrypto | null = browserCrypto()): string {
  if (typeof provider?.randomUUID === 'function') return provider.randomUUID()
  const bytes = randomBytes(provider)
  if (!bytes) throw new Error('当前浏览器不支持安全随机数，请升级浏览器或飞书客户端')
  return formatUUID(bytes)
}
