import { describe, expect, it } from 'vitest'
import { inferFeishuSourceType } from './feishuSource'

describe('inferFeishuSourceType', () => {
  it('recognizes Wiki URLs and node tokens', () => {
    expect(inferFeishuSourceType('https://example.feishu.cn/wiki/wikcnRoot?from=copy')).toBe('feishu_wiki')
    expect(inferFeishuSourceType('wikcnRoot')).toBe('feishu_wiki')
  })

  it('recognizes Drive folder URLs and tokens', () => {
    expect(inferFeishuSourceType('https://example.feishu.cn/drive/folder/fldcnRoot')).toBe('feishu_folder')
    expect(inferFeishuSourceType('fldcnRoot')).toBe('feishu_folder')
  })
})
