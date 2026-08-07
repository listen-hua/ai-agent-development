import { describe, expect, it } from 'vitest'
import { feishuEmojiByCode, feishuEmojis, filterFeishuEmojis } from './feishuEmojis'

describe('feishuEmojis', () => {
  it('contains the fixed set of 40 bundled Feishu emojis', () => {
    expect(feishuEmojis).toHaveLength(40)
    expect(new Set(feishuEmojis.map((item) => item.code)).size).toBe(40)
    expect(feishuEmojiByCode.get('OK')?.imageUrl).toContain('ok')
  })

  it('searches Chinese names, English codes, and keywords', () => {
    expect(filterFeishuEmojis('敬礼').map((item) => item.code)).toEqual(['SALUTE'])
    expect(filterFeishuEmojis('thumb').map((item) => item.code)).toEqual(['THUMBSUP'])
    expect(filterFeishuEmojis('生日').map((item) => item.code)).toEqual(['CAKE'])
  })
})
