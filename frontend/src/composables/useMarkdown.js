// 极简 Markdown 渲染（从 recovered App.vue 的 renderMarkdown 迁移，行为一一保留）。
// 支持：代码块 ```、ATX 标题 #~######（降级 2 级）、无序列表 - * +、行内 code / **粗体** / [文本](链接)。
export function renderMarkdown(md) {
  if (!md) return ''
  const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  const inline = (s) =>
    s
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '$1')
  const lines = esc(md).split(/\r?\n/)
  const out = []
  let inCode = false
  let inList = false
  const closeList = () => {
    if (inList) {
      out.push('</ul>')
      inList = false
    }
  }
  for (const line of lines) {
    if (line.trim().startsWith('```')) {
      closeList()
      out.push(inCode ? '</code></pre>' : '<pre><code>')
      inCode = !inCode
      continue
    }
    if (inCode) {
      out.push(line)
      continue
    }
    const h = /^(#{1,6})\s+(.*)$/.exec(line)
    if (h) {
      closeList()
      const lv = Math.min(h[1].length + 2, 6)
      out.push(`<h${lv}>${inline(h[2])}</h${lv}>`)
      continue
    }
    const li = /^\s*[-*+]\s+(.*)$/.exec(line)
    if (li) {
      if (!inList) {
        out.push('<ul>')
        inList = true
      }
      out.push(`<li>${inline(li[1])}</li>`)
      continue
    }
    closeList()
    if (line.trim() === '') {
      out.push('')
      continue
    }
    out.push(`<p>${inline(line)}</p>`)
  }
  closeList()
  if (inCode) out.push('</code></pre>')
  return out.join('\n')
}

export function useMarkdown() {
  return { renderMarkdown }
}

export default useMarkdown
