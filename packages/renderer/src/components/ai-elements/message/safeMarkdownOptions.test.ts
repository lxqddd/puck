import { describe, expect, it } from 'vitest'
import { createMarkmendParser } from 'vue-stream-markdown'
import { safeMarkdownParserOptions } from './safeMarkdownOptions'

function collectTags(value: unknown, out: string[] = []): string[] {
  if (Array.isArray(value)) {
    if (typeof value[0] === 'string') out.push(value[0].toLowerCase())
    for (const child of value) collectTags(child, out)
  }
  else if (value && typeof value === 'object') {
    for (const child of Object.values(value)) collectTags(child, out)
  }
  return out
}

async function parse(markdown: string): Promise<unknown[]> {
  const parser = createMarkmendParser({ parserOptions: safeMarkdownParserOptions() })
  const result = await parser.parse(markdown, 'static')
  return result.document.nodes as unknown[]
}

describe('safe markdown options', () => {
  const forbidden = new Set(['script', 'meta', 'style', 'link', 'iframe', 'base', 'form', 'svg'])

  it.each([
    ':script[alert(1)]',
    ':meta{http-equiv="refresh" content="0;url=https://attacker.example/"}',
    ':style[body { display: none }]',
    ':link{rel="stylesheet" href="https://attacker.example/x.css"}',
    ':iframe{src="https://attacker.example/"}',
    ':base{href="https://attacker.example/"}',
    ':form{action="https://attacker.example/"}',
    ':svg[<script>alert(1)</script>]',
  ])('drops dangerous component tag: %s', async (markdown) => {
    const tags = collectTags(await parse(markdown))
    expect(tags.filter(tag => forbidden.has(tag))).toEqual([])
  })

  it('escapes literal HTML instead of rendering script elements', async () => {
    const tags = collectTags(await parse('<script>alert(1)</script>'))
    expect(tags).not.toContain('script')
  })

  it('removes overlay, event and runtime-link attributes', async () => {
    const span = await parse(
      ':span{style="position:fixed;inset:0" class="fixed inset-0" onclick="alert(1)"}[x]',
    )
    const spanJson = JSON.stringify(span)
    expect(collectTags(span)).toContain('span')
    expect(spanJson).not.toContain('position:fixed')
    expect(spanJson).not.toContain('fixed inset-0')
    expect(spanJson).not.toContain('onclick')

    const link = await parse(':a{href="wails://localhost/wails/runtime?object=0" title="x"}[x]')
    expect(JSON.stringify(link)).not.toContain('wails://')
    expect(collectTags(link)).toContain('a')

    const image = await parse(':img{src="wails://localhost/wails/runtime?object=0" alt="x"}')
    expect(JSON.stringify(image)).not.toContain('wails://')
    expect(collectTags(image)).toContain('img')
  })

  it('keeps safe Markdown, links and code blocks', async () => {
    const tags = collectTags(
      await parse('**bold**\n\n- item\n\n[link](https://example.com)\n\n```ts\nconst x = 1\n```'),
    )
    expect(tags).toContain('strong')
    expect(tags).toContain('ul')
    expect(tags).toContain('li')
    expect(tags).toContain('a')
    expect(tags.some(tag => tag === 'pre' || tag === 'code')).toBe(true)
  })
})
