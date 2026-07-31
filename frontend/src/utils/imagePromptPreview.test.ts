import { describe, expect, it } from 'vitest'
import { promptPreviewMaxBytes, validatePromptPreviewFile } from './imagePromptPreview'

describe('validatePromptPreviewFile', () => {
  it('accepts supported image types within the size limit', () => {
    expect(validatePromptPreviewFile(new File(['image'], 'preview.png', { type: 'image/png' }))).toBeNull()
  })

  it('rejects unsupported images and oversized files', () => {
    expect(validatePromptPreviewFile(new File(['gif'], 'preview.gif', { type: 'image/gif' }))).toContain('JPG')
    const oversized = new File([new Uint8Array(promptPreviewMaxBytes + 1)], 'large.webp', { type: 'image/webp' })
    expect(validatePromptPreviewFile(oversized)).toContain('5 MB')
  })
})
