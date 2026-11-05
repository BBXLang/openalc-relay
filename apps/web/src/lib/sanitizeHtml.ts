/*
 * 邮件 HTML 保守白名单净化（FORMAL-UI-MAIL-PLAN §2.3）。
 *
 * 重要：真实数据开放前须替换为经审计的净化库（例如 DOMPurify，固定版本并审查许可证）。
 * 当前实现仅用于正式链路开放前的结构验证，策略为「默认拒绝」：
 *   - 只保留极小的展示标签白名单，其余标签解除包裹；script/style/iframe 等危险标签连同内容删除；
 *   - 删除全部属性，仅保留 <a href>，且 href 只允许 http/https/mailto；
 *   - 删除所有事件属性、内联样式与注释；
 *   - <img> 一律移除（远程图片默认不加载），保留 alt 文本占位。
 * DOMParser 不会执行脚本，序列化结果再以 v-html 插入。
 */
const MAX_INPUT_LENGTH = 512 * 1024

const ALLOWED_TAGS = new Set([
  'p', 'br', 'div', 'span', 'blockquote', 'pre', 'code',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'ul', 'ol', 'li',
  'b', 'strong', 'i', 'em', 'u', 's', 'small', 'sub', 'sup',
  'table', 'thead', 'tbody', 'tfoot', 'tr', 'td', 'th', 'caption', 'colgroup', 'col',
  'hr',
])

/** 连同内容一起删除的标签。 */
const DROP_WITH_CONTENT = new Set([
  'script', 'style', 'iframe', 'frame', 'frameset', 'object', 'embed', 'applet',
  'template', 'noscript', 'svg', 'math', 'form', 'input', 'button', 'select',
  'option', 'textarea', 'audio', 'video', 'source', 'track', 'canvas', 'dialog', 'slot',
])

const SAFE_URL_PATTERN = /^(?:https?:|mailto:)/i

function safeHref(value: string): string | null {
  const trimmed = value.trim()
  if (trimmed.length === 0 || trimmed.length > 2048) return null
  // 相对地址与协议相对地址按不可信处理，一律拒绝。
  if (!SAFE_URL_PATTERN.test(trimmed)) return null
  return trimmed
}

function escapeText(value: string): string {
  return value.replace(/[&<>"']/g, char => (
    char === '&' ? '&amp;' :
    char === '<' ? '&lt;' :
    char === '>' ? '&gt;' :
    char === '"' ? '&quot;' : '&#39;'
  ))
}

function sanitizeNode(node: Element): void {
  for (const child of Array.from(node.children)) {
    const tag = child.tagName.toLowerCase()
    if (DROP_WITH_CONTENT.has(tag) || tag === 'img' || tag === 'picture') {
      const alt = tag === 'img' && typeof child.getAttribute('alt') === 'string'
        ? child.getAttribute('alt')!.trim()
        : ''
      child.remove()
      if (alt.length > 0) node.appendChild(document.createTextNode(`［图片未加载：${alt}］`))
      continue
    }
    if (!ALLOWED_TAGS.has(tag)) {
      // 白名单之外的危险性未知的标签：解除包裹，保留其子节点继续净化。
      const parent = child.parentElement
      if (parent === null) { child.remove(); continue }
      const fragment = document.createDocumentFragment()
      while (child.firstChild) fragment.appendChild(child.firstChild)
      child.replaceWith(fragment)
      // 子节点已移动，继续在原层级净化（重新遍历当前节点）。
      sanitizeNode(node)
      return
    }
    for (const attribute of Array.from(child.attributes)) child.removeAttribute(attribute.name)
    if (tag === 'a') {
      const href = child.getAttribute('href') === null ? '' : child.getAttribute('href')!
      const safe = safeHref(href)
      if (safe === null) child.removeAttribute('href')
      else {
        child.setAttribute('href', safe)
        child.setAttribute('rel', 'noopener noreferrer nofollow')
      }
    }
    sanitizeNode(child)
  }
}

export function sanitizeHtml(input: string): string {
  if (typeof input !== 'string' || input.length === 0) return ''
  if (input.length > MAX_INPUT_LENGTH) {
    // 超长输入不解析为 HTML，按纯文本转义处理。
    return escapeText(input.slice(0, MAX_INPUT_LENGTH))
  }
  const parsed = new DOMParser().parseFromString(input, 'text/html')
  // head 中可能出现的标签由浏览器归一化；只净化并序列化 body。
  sanitizeNode(parsed.body)
  return parsed.body.innerHTML
}
