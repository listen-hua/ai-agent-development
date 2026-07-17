import { describe, expect, it } from 'vitest'
import { renderNotificationMarkdown } from './notificationMarkdown'

describe('renderNotificationMarkdown', () => {
  it('renders supported rich text', () => {
    const html = renderNotificationMarkdown('## 标题\n**重点**\n- 第一项\n[链接](https://example.com)')
    expect(html).toContain('<h3>标题</h3>')
    expect(html).toContain('<strong>重点</strong>')
    expect(html).toContain('<ul><li>第一项</li></ul>')
    expect(html).toContain('href="https://example.com"')
  })

  it('escapes raw HTML and unsafe links', () => {
    const html = renderNotificationMarkdown('<img src=x onerror=alert(1)>\n[x](javascript:alert(1))')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('href="javascript:')
    expect(html).toContain('&lt;img')
  })
})
