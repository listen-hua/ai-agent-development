import type { ConnectedKnowledgeSourceType } from '@/types/domain'

export function inferFeishuSourceType(value: string): ConnectedKnowledgeSourceType | undefined {
  const normalized = value.trim().toLowerCase()
  if (normalized.includes('/wiki/') || (!normalized.includes('://') && normalized.startsWith('wik'))) {
    return 'feishu_wiki'
  }
  if (normalized.includes('/drive/folder/') || (!normalized.includes('://') && normalized.startsWith('fld'))) {
    return 'feishu_folder'
  }
  return undefined
}
