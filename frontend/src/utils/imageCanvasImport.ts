export type CanvasImportOrigin = 'paste' | 'drop'

export const CANVAS_IMPORT_MAX_FILES = 20
export const CANVAS_IMPORT_MAX_BYTES = 10 * 1024 * 1024
export const CANVAS_IMPORT_GRID_STEP = 384

const allowedTypes = new Set(['image/jpeg', 'image/png', 'image/webp'])
const allowedExtensions = /\.(jpe?g|png|webp)$/i

export interface CanvasImportRejection {
  file: File
  reason: 'limit' | 'type' | 'size'
}

export function collectTransferFiles(transfer: DataTransfer | null): File[] {
  if (!transfer) return []
  const fromItems = Array.from(transfer.items || [])
    .filter((item) => item.kind === 'file')
    .map((item) => item.getAsFile())
    .filter((file): file is File => file !== null)
  return fromItems.length ? fromItems : Array.from(transfer.files || [])
}

export function validateCanvasImportFiles(files: File[]) {
  const accepted: File[] = []
  const rejected: CanvasImportRejection[] = []
  files.forEach((file, index) => {
    if (index >= CANVAS_IMPORT_MAX_FILES) {
      rejected.push({ file, reason: 'limit' })
    } else if (!allowedTypes.has(file.type.toLowerCase()) && !(file.type === '' && allowedExtensions.test(file.name))) {
      rejected.push({ file, reason: 'type' })
    } else if (file.size > CANVAS_IMPORT_MAX_BYTES) {
      rejected.push({ file, reason: 'size' })
    } else {
      accepted.push(file)
    }
  })
  return { accepted, rejected }
}

export function canvasImportPositions(origin: { x: number; y: number }, count: number) {
  return Array.from({ length: count }, (_, index) => ({
    x: origin.x + (index % 3) * CANVAS_IMPORT_GRID_STEP,
    y: origin.y + Math.floor(index / 3) * CANVAS_IMPORT_GRID_STEP,
  }))
}

export function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || Boolean(target.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]'))
}

export function hasExternalFiles(transfer: DataTransfer | null) {
  return Boolean(transfer && Array.from(transfer.types || []).includes('Files'))
}
