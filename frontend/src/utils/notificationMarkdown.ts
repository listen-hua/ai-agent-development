function escapeHTML(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function renderInline(value: string) {
  return value
    .replace(/\[([^\]]+)]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/__([^_]+)__/g, '<strong>$1</strong>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/(?<!\*)\*([^*]+)\*(?!\*)/g, '<em>$1</em>')
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
