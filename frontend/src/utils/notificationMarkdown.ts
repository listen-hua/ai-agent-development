import { feishuEmojiByCode, type FeishuEmoji } from '@/constants/feishuEmojis'

function escapeHTML(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function protectFeishuEmojiTokens(value: string) {
  const emojis: FeishuEmoji[] = []
  const content = value.replace(/:([A-Z][A-Z0-9_]*):/g, (token, code: string) => {
    const emoji = feishuEmojiByCode.get(code)
    if (!emoji) return token
    const index = emojis.push(emoji) - 1
    return `\uE000${index}\uE001`
  })
  return { content, emojis }
}

function restoreFeishuEmojiTokens(value: string, emojis: FeishuEmoji[]) {
  return value.replace(/\uE000(\d+)\uE001/g, (token, index: string) => {
    const emoji = emojis[Number(index)]
    if (!emoji) return token
    return `<img class="feishu-emoji-inline" src="${emoji.imageUrl}" alt="${emoji.name}" title="${emoji.name} · :${emoji.code}:" draggable="false">`
  })
}

function renderInline(value: string) {
  const protectedValue = protectFeishuEmojiTokens(value)
  const rendered = protectedValue.content
    .replace(/\[([^\]]+)]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/__([^_]+)__/g, '<strong>$1</strong>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/(?<!\*)\*([^*]+)\*(?!\*)/g, '<em>$1</em>')
  return restoreFeishuEmojiTokens(rendered, protectedValue.emojis)
}

export function renderNotificationText(value: string) {
  const protectedValue = protectFeishuEmojiTokens(escapeHTML(value))
  return restoreFeishuEmojiTokens(protectedValue.content, protectedValue.emojis)
}

export function normalizeFeishuCardMarkdown(markdown: string) {
  return markdown.replace(/^#{1,6}\s+(.+)$/gm, (line, title: string) => {
    const marker = line.startsWith('# ') ? '▌ ' : '• '
    return `**${marker}${title.trim()}**`
  })
}

export function renderNotificationMarkdown(markdown: string) {
  const lines = escapeHTML(normalizeFeishuCardMarkdown(markdown)).split(/\r?\n/)
  const output: string[] = []
  let listType: 'ul' | 'ol' | undefined
  const closeList = () => {
    if (listType) output.push(`</${listType}>`)
    listType = undefined
  }
  for (const line of lines) {
    const unordered = line.match(/^\s*[-*]\s+(.+)$/)
    const ordered = line.match(/^\s*\d+[.)]\s+(.+)$/)
    if (unordered || ordered) {
      const nextType = unordered ? 'ul' : 'ol'
      if (listType !== nextType) {
        closeList()
        listType = nextType
        output.push(`<${nextType}>`)
      }
      output.push(`<li>${renderInline((unordered || ordered)![1])}</li>`)
      continue
    }
    closeList()
    if (!line.trim()) {
      output.push('<br>')
    } else {
      output.push(`<p>${renderInline(line)}</p>`)
    }
  }
  closeList()
  return output.join('')
}
