import { describe, expect, it } from 'vitest'
import { isChunkLoadError } from './chunkRecovery'

describe('isChunkLoadError', () => {
  it.each([
    'Failed to fetch dynamically imported module',
    'Importing a module script failed',
    'Expected a JavaScript-or-Wasm module script but the server responded with a MIME type of text/html',
    'Loading chunk 21 failed',
    'ChunkLoadError',
  ])('recognizes recoverable deployment error: %s', (message) => {
    expect(isChunkLoadError(new Error(message))).toBe(true)
  })

  it('does not reload for application exceptions', () => {
    expect(isChunkLoadError(new Error('Cannot read properties of undefined'))).toBe(false)
  })
})
