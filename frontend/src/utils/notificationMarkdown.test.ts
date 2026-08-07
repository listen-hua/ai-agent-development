import { describe, expect, it } from 'vitest'
import { normalizeFeishuCardMarkdown, renderNotificationMarkdown, renderNotificationText } from './notificationMarkdown'

describe('renderNotificationMarkdown', () => {
  it('renders supported rich text', () => {
    const html = renderNotificationMarkdown('## 标题\n**重点**\n- 第一项\n[链接](https://example.com)')
    expect(html).toContain('<p><strong>• 标题</strong></p>')
    expect(html).toContain('<strong>重点</strong>')
    expect(html).toContain('<ul><li>第一项</li></ul>')
    expect(html).toContain('href="https://example.com"')
  })

  it('converts unsupported headings to Feishu-compatible emphasis', () => {
    expect(normalizeFeishuCardMarkdown('# 主标题\n## 小标题\n### 三级标题')).toBe('**▌ 主标题**\n**• 小标题**\n**• 三级标题**')
  })

  it('escapes raw HTML and unsafe links', () => {
    const html = renderNotificationMarkdown('<img src=x onerror=alert(1)>\n[x](javascript:alert(1))')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('href="javascript:')
    expect(html).toContain('&lt;img')
  })

  it('renders allowlisted Feishu emojis and preserves unknown tokens', () => {
    const html = renderNotificationMarkdown('已完成 :OK:，未知 :NOT_REAL:')
    expect(html).toContain('class="feishu-emoji-inline"')
    expect(html).toContain('title="好的 · :OK:"')
    expect(html).toContain(':NOT_REAL:')

    const title = renderNotificationText('<通知> :PARTY:')
    expect(title).toContain('&lt;通知&gt;')
    expect(title).toContain('title="庆祝 · :PARTY:"')
  })
})
