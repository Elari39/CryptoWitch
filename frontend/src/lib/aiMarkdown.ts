import { marked } from 'marked'
import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import python from 'highlight.js/lib/languages/python'
import go from 'highlight.js/lib/languages/go'
import java from 'highlight.js/lib/languages/java'
import c from 'highlight.js/lib/languages/c'
import cpp from 'highlight.js/lib/languages/cpp'
import csharp from 'highlight.js/lib/languages/csharp'
import bash from 'highlight.js/lib/languages/bash'
import shell from 'highlight.js/lib/languages/shell'
import json from 'highlight.js/lib/languages/json'
import xml from 'highlight.js/lib/languages/xml'
import css from 'highlight.js/lib/languages/css'
import sql from 'highlight.js/lib/languages/sql'
import markdown from 'highlight.js/lib/languages/markdown'
import yaml from 'highlight.js/lib/languages/yaml'
import rust from 'highlight.js/lib/languages/rust'
import php from 'highlight.js/lib/languages/php'
import ruby from 'highlight.js/lib/languages/ruby'
import kotlin from 'highlight.js/lib/languages/kotlin'
import swift from 'highlight.js/lib/languages/swift'
import diff from 'highlight.js/lib/languages/diff'
import ini from 'highlight.js/lib/languages/ini'
import dockerfile from 'highlight.js/lib/languages/dockerfile'

// 常用语言按需注册，避免引入 highlight.js 全量包。
const languages: Record<string, unknown> = {
  javascript,
  typescript,
  python,
  go,
  java,
  c,
  cpp,
  csharp,
  bash,
  shell,
  json,
  xml,
  css,
  sql,
  markdown,
  yaml,
  rust,
  php,
  ruby,
  kotlin,
  swift,
  diff,
  ini,
  dockerfile,
}

for (const [name, language] of Object.entries(languages)) {
  hljs.registerLanguage(name, language as never)
}

/**
 * AI 回复的 KaTeX 分隔符配置，与正文阅读器（DocumentViewer）保持一致：
 * $$ / \[ \] 为块级公式，$ / \( \) 为行内公式。
 */
export const AI_MATH_DELIMITERS = [
  { left: '$$', right: '$$', display: true },
  { left: '\\[', right: '\\]', display: true },
  { left: '\\(', right: '\\)', display: false },
  { left: '$', right: '$', display: false },
]

// 整体移除的危险标签（连同内容），防止脚本/表单/嵌入内容进入 DOM。
const DANGEROUS_TAGS = new Set([
  'script',
  'style',
  'iframe',
  'frame',
  'object',
  'embed',
  'link',
  'meta',
  'base',
  'form',
  'input',
  'button',
  'textarea',
  'select',
  'option',
  'svg',
  'math',
  'template',
  'noscript',
  'audio',
  'video',
  'source',
  'track',
  'canvas',
])

// 链接协议白名单，镜像后端 markdownSanitizer：http/https/mailto 与相对路径（含 # 锚点）。
function isSafeLink(href: string): boolean {
  const value = href.trim().toLowerCase()
  if (value.startsWith('#') || value.startsWith('/') || value.startsWith('./') || value.startsWith('../')) {
    return true
  }
  return value.startsWith('http://') || value.startsWith('https://') || value.startsWith('mailto:')
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

// 代码语法高亮：语言未注册或解析失败时回退为纯文本（已转义）。
function highlightCode(code: string, lang: string): string {
  if (lang && hljs.getLanguage(lang)) {
    try {
      return hljs.highlight(code, { language: lang, ignoreIllegals: true }).value
    } catch {
      // 流式输出中途代码不完整时可能抛错，回退纯文本。
    }
  }
  return escapeHtml(code)
}

// 自定义代码块渲染：输出「代码窗」结构（深色卡片 + 语言标识 + 复制按钮）。
marked.use({
  renderer: {
    code({ text, lang }: { text: string; lang?: string }) {
      const language = (lang || '').trim().split(/\s+/)[0] || ''
      const label = language || 'code'
      const highlighted = highlightCode(text, language)
      return (
        '<div class="ai-code-window">' +
        '<div class="ai-code-header">' +
        `<span class="ai-code-lang">${escapeHtml(label)}</span>` +
        '<span class="ai-copy-btn" role="button" tabindex="0">复制</span>' +
        '</div>' +
        `<pre><code>${highlighted}</code></pre>` +
        '</div>'
      )
    },
  },
})

/**
 * 白名单消毒：危险标签整体移除；img 替换为 alt 文本（隐私优先，不加载远程图片）；
 * a 链接仅保留合法协议，其余解包为纯文本；
 * 其余标签仅放行 class 属性（hljs 高亮与代码窗依赖），剥离其余全部属性（防 on* / style / data-* 注入）。
 */
function sanitizeHTML(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  const root = doc.body

  const walk = (parent: Node) => {
    for (const node of Array.from(parent.childNodes)) {
      if (node.nodeType !== 1 /* Node.ELEMENT_NODE */) {
        continue
      }
      const el = node as Element
      const tag = el.tagName.toLowerCase()

      if (DANGEROUS_TAGS.has(tag)) {
        // 整体移除（含子树），无需继续遍历。
        el.remove()
        continue
      }
      // 先深度遍历子节点：后续的 img 替换 / a 解包会把子节点移动到父级，
      // 若在遍历后才处理，被移动的子节点会漏检（如 javascript: 链接内嵌 <img onerror>）。
      walk(el)

      if (tag === 'img') {
        const alt = (el.getAttribute('alt') || '').trim()
        el.replaceWith(doc.createTextNode(alt ? `（图片：${alt}）` : '（图片）'))
        continue
      }
      if (tag === 'a') {
        const href = el.getAttribute('href') || ''
        if (isSafeLink(href)) {
          el.setAttribute('href', href)
          el.removeAttribute('target')
          el.removeAttribute('rel')
          el.removeAttribute('class')
        } else {
          // 解包为纯文本（子节点已消毒完毕）。
          el.replaceWith(...Array.from(el.childNodes))
          continue
        }
      } else {
        // 仅放行 class / role / tabindex（hljs 高亮、代码窗与复制按钮依赖），剥离其余属性。
        for (const attr of Array.from(el.attributes)) {
          if (attr.name !== 'class' && attr.name !== 'role' && attr.name !== 'tabindex') {
            el.removeAttribute(attr.name)
          }
        }
      }
    }
  }

  walk(root)
  return root.innerHTML
}

/**
 * 把 AI 回复的 Markdown 渲染为安全的 HTML 片段。
 * - GFM（表格、删除线等）+ breaks（单换行也断行，契合聊天流式文本）；
 * - 代码块输出为带语言标识/复制按钮/语法高亮的代码窗；
 * - 输出经白名单消毒后再交给模板插入，脚本与危险协议无法进入 DOM。
 */
export function renderAIMarkdown(source: string): string {
  const raw = marked.parse(source, { gfm: true, breaks: true }) as string
  return sanitizeHTML(raw)
}
