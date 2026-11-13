<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { Markdown } from 'vue-stream-markdown'
import { bridge } from '@/bridge'
import { safeMarkdownParserOptions } from './safeMarkdownOptions'
import { useMarkdownExtensions } from './useMarkdownExtensions'
import 'vue-stream-markdown/index.css'
import 'katex/dist/katex.min.css'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  content: string
  class?: HTMLAttributes['class']
}>()

const { extensions } = useMarkdownExtensions()
const parserOptions = safeMarkdownParserOptions()

function handleClick(event: MouseEvent) {
  if (!(event.target instanceof Element)) return
  const anchor = event.target.closest('a[href]')
  if (!anchor) return

  event.preventDefault()
  event.stopPropagation()

  const href = anchor.getAttribute('href')?.trim() ?? ''
  if (!/^https?:\/\//i.test(href)) return
  void Promise.resolve().then(() => bridge.openExternal(href)).catch(() => {})
}
</script>

<template>
  <div class="contents" @click.capture="handleClick">
    <Markdown
      :content="content"
      :extensions="extensions"
      :parser-options="parserOptions"
      :class="props.class"
      v-bind="$attrs"
    />
  </div>
</template>
