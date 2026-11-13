import security from 'comark/plugins/security'
import { visitAsync } from 'comark/utils'
import type { ComarkPlugin } from 'comark'
import type { StreamMarkdownParserOptions } from 'vue-stream-markdown'

// Only the tags produced by the Markdown/UI features we support. In particular,
// this drops component-syntax tags such as :meta, :script, :iframe and :style
// even before they reach Vue's renderer.
const SAFE_TAGS = [
  'a',
  'blockquote',
  'br',
  'code',
  'del',
  'em',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'hr',
  'img',
  'li',
  'math',
  'mermaid',
  'ol',
  'p',
  'pre',
  's',
  'span',
  'strong',
  'sub',
  'sup',
  'table',
  'tbody',
  'td',
  'th',
  'thead',
  'tr',
  'ul',
] as const

// Attributes that are either semantic or consumed by the Markdown renderer.
// style/class/id, on*, target, ping, srcset, form*, http-equiv, etc. are all
// removed, so Markdown cannot produce overlays, navigation or runtime calls.
const SAFE_ATTRS: Record<string, readonly string[]> = {
  a: ['href', 'title'],
  img: ['src', 'alt', 'title'],
  code: ['lang', 'meta'],
  pre: ['lang', 'meta'],
  math: ['display'],
  td: ['align', 'colspan', 'rowspan'],
  th: ['align', 'colspan', 'rowspan'],
  ol: ['start'],
}

type ElementTuple = [string, Record<string, unknown>, ...unknown[]]

const attributesPlugin: ComarkPlugin = {
  name: 'puck-markdown-attributes',
  async post(state) {
    await visitAsync(
      state.tree,
      node => typeof node !== 'string' && node[0] !== null,
      (node) => {
        if (typeof node === 'string' || !Array.isArray(node)) return
        const element = node as ElementTuple
        if (typeof element[0] !== 'string') return
        const attrs = element[1]
        if (!attrs || typeof attrs !== 'object') return
        const allowed = SAFE_ATTRS[element[0].toLowerCase()] ?? []
        for (const key of Object.keys(attrs)) {
          if (!allowed.includes(key)) delete attrs[key]
        }
      },
    )
  },
}

export function safeMarkdownParserOptions(): StreamMarkdownParserOptions {
  return {
    html: false,
    plugins: [
      security({
        allowedTags: [...SAFE_TAGS],
        tagFallback: () => false,
        allowedLinkPrefixes: ['https://', 'http://'],
        allowedImagePrefixes: ['https://', 'http://', 'data:image/'],
        allowedProtocols: ['http', 'https'],
        allowDataImages: true,
      }),
      attributesPlugin,
    ],
  }
}
