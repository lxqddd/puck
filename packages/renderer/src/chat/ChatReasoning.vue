<script setup lang="ts">
import { BrainIcon, ChevronDownIcon } from '@lucide/vue'
import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from '@/components/ai-elements/reasoning'
import { Shimmer } from '@/components/ai-elements/shimmer'

withDefaults(
  defineProps<{
    content: string
    isStreaming?: boolean
    defaultOpen?: boolean
  }>(),
  {
    isStreaming: false,
    defaultOpen: false,
  },
)
</script>

<template>
  <Reasoning :is-streaming="isStreaming" :default-open="defaultOpen" class="mb-2">
    <ReasoningTrigger
      class="group rounded-lg border border-border bg-secondary/45 px-3 py-2 text-xs text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
    >
      <div class="flex w-full items-center gap-2">
        <BrainIcon class="size-3.5 shrink-0" />
        <Shimmer v-if="isStreaming" :duration="1.4" class="text-xs">
          正在思考…
        </Shimmer>
        <span v-else>思考过程</span>
        <ChevronDownIcon
          class="ml-auto size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-180"
        />
      </div>
    </ReasoningTrigger>
    <ReasoningContent
      :content="content"
      class="mt-2 rounded-lg border border-border bg-secondary/30 px-3 py-2 text-xs leading-6"
    />
  </Reasoning>
</template>
