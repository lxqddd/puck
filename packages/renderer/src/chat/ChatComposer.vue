<script setup lang="ts">
import { ref } from 'vue'
import type { ProviderConfig } from '@puck/contract'
import { PromptInputProvider } from '@/components/ai-elements/prompt-input'
import ChatComposerInner from './ChatComposerInner.vue'

defineProps<{
  providers: ProviderConfig[]
  selectedProviderId: string
  pending: boolean
}>()

const emit = defineEmits<{
  (e: 'update:selectedProviderId', value: string): void
  (e: 'submit', text: string): void
  (e: 'error', message: string): void
}>()

const inner = ref<InstanceType<typeof ChatComposerInner> | null>(null)

function handleSubmit(message: { text: string }) {
  const text = message.text.trim()
  if (!text) return
  emit('submit', text)
}

function handleError(error: { message: string }) {
  emit('error', error.message)
}

function setText(text: string) {
  inner.value?.setText(text)
}

function clear() {
  inner.value?.clear()
}

function focus() {
  inner.value?.focus()
}

defineExpose({ setText, clear, focus })
</script>

<template>
  <PromptInputProvider @submit="handleSubmit" @error="handleError">
    <ChatComposerInner
      ref="inner"
      :providers="providers"
      :selected-provider-id="selectedProviderId"
      :pending="pending"
      @update:selected-provider-id="emit('update:selectedProviderId', $event)"
    />
  </PromptInputProvider>
</template>
