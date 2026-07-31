const previewMimeTypes = new Set(['image/jpeg', 'image/png', 'image/webp'])
export const promptPreviewMaxBytes = 5 * 1024 * 1024

export function validatePromptPreviewFile(file: File): string | null {
  if (!previewMimeTypes.has(file.type)) return '预览图只支持 JPG、PNG 和 WEBP 格式'
  if (file.size <= 0) return '预览图内容不能为空'
  if (file.size > promptPreviewMaxBytes) return '预览图不能超过 5 MB'
  return null
}
