// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'
import {
  CANVAS_IMPORT_MAX_BYTES,
  canvasImportPositions,
  collectTransferFiles,
  isEditableTarget,
  validateCanvasImportFiles,
} from './imageCanvasImport'

describe('image canvas imports', () => {
  it('accepts supported files and reports type, size, and count limits', () => {
    const files = Array.from({ length: 22 }, (_, index) =>
      new File(['image'], `image-${index}.png`, { type: 'image/png' }))
    files[1] = new File(['pdf'], 'document.pdf', { type: 'application/pdf' })
    files[2] = new File([new Uint8Array(CANVAS_IMPORT_MAX_BYTES + 1)], 'large.webp', { type: 'image/webp' })

    const result = validateCanvasImportFiles(files)

    expect(result.accepted).toHaveLength(18)
    expect(result.rejected.map((item) => item.reason)).toEqual(['type', 'size', 'limit', 'limit'])
  })

  it('places multiple images in a three-column grid from the target point', () => {
    expect(canvasImportPositions({ x: 100, y: 200 }, 5)).toEqual([
      { x: 100, y: 200 },
      { x: 484, y: 200 },
      { x: 868, y: 200 },
      { x: 100, y: 584 },
      { x: 484, y: 584 },
    ])
  })

  it('only extracts real file items and ignores URL-only payloads', () => {
    const image = new File(['image'], 'clipboard.png', { type: 'image/png' })
    const fileTransfer = {
      items: [
        { kind: 'string', getAsFile: () => null },
        { kind: 'file', getAsFile: () => image },
      ],
      files: [],
    } as unknown as DataTransfer
    const urlTransfer = {
      items: [{ kind: 'string', getAsFile: () => null }],
      files: [],
    } as unknown as DataTransfer

    expect(collectTransferFiles(fileTransfer)).toEqual([image])
    expect(collectTransferFiles(urlTransfer)).toEqual([])
  })

  it('recognizes text-editing targets so paste stays in the editor', () => {
    const input = document.createElement('input')
    const editable = document.createElement('div')
    editable.setAttribute('contenteditable', 'true')
    const plain = document.createElement('div')

    expect(isEditableTarget(input)).toBe(true)
    expect(isEditableTarget(editable)).toBe(true)
    expect(isEditableTarget(plain)).toBe(false)
  })
})
