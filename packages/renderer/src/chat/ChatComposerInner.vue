<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import type { ProviderConfig } from '@puck/contract'
import {
  ArrowUpIcon,
  CheckIcon,
  ChevronsUpDownIcon,
  Loader2Icon,
} from '@lucide/vue'
import {
  ModelSelector,
  ModelSelectorContent,
  ModelSelectorEmpty,
  ModelSelectorGroup,
  ModelSelectorInput,
  ModelSelectorItem,
  ModelSelectorList,
  ModelSelectorName,
  ModelSelectorTrigger,
} from '@/components/ai-elements/model-selector'
import {
  PromptInput,
  PromptInputBody,
  PromptInputFooter,
  PromptInputSubmit,
  PromptInputTextarea,
  PromptInputTools,
  usePromptInput,
} from '@/components/ai-elements/prompt-input'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  providers: ProviderConfig[]
  selectedProviderId: string
  pending: boolean
}>()

const emit = defineEmits<{
  (e: 'update:selectedProviderId', value: string): void
}>()

const { textInput, setTextInput, clearInput } = usePromptInput()
const modelOpen = ref(false)
const promptRef = ref<InstanceType<typeof PromptInput> | null>(null)

const canSubmit = computed(() => !props.pending && textInput.value.trim().length > 0)

const selectedLabel = computed(() => {
  const provider = props.providers.find((item) => item.id === props.selectedProviderId)
  return provider ? providerLabel(provider) : '选择模型'
})

function providerLabel(provider: ProviderConfig): string {
  const model = provider.model?.trim()
  const name = provider.name?.trim() || provider.type || '模型'
  return model ? `${name} · ${model}` : name
}

function chooseProvider(id: string) {
  emit('update:selectedProviderId', id)
  modelOpen.value = false
}

function focus() {
  const root = promptRef.value?.$el as HTMLElement | undefined
  root?.querySelector('textarea')?.focus()
}

function setText(text: string) {
  setTextInput(text)
  void nextTick(focus)
}

function clear() {
  clearInput()
}

defineExpose({ setText, clear, focus })
</script>

<template>
  <PromptInput ref="promptRef" class="chat-composer">
    <PromptInputBody>
      <PromptInputTextarea
        :disabled="pending"
        placeholder="跟 Puck 说点什么…"
        aria-label="消息内容"
        class="chat-composer-input"
      />
    </PromptInputBody>
    <PromptInputFooter class="chat-composer-footer">
      <PromptInputTools>
        <ModelSelector v-if="providers.length" v-model:open="modelOpen">
          <ModelSelectorTrigger as-child>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              class="h-7 max-w-[220px] gap-1.5 px-2 text-xs text-muted-foreground hover:text-foreground"
              aria-label="切换当前对话使用的模型"
            >
              <span class="truncate">{{ selectedLabel }}</span>
              <ChevronsUpDownIcon class="size-3 shrink-0 opacity-60" />
            </Button>
          </ModelSelectorTrigger>
          <ModelSelectorContent title="选择模型">
            <ModelSelectorInput placeholder="搜索模型…" />
            <ModelSelectorList>
              <ModelSelectorEmpty>没有可用的模型</ModelSelectorEmpty>
              <ModelSelectorGroup heading="可用模型">
                <ModelSelectorItem
                  v-for="provider in providers"
                  :key="provider.id"
                  :value="provider.id"
                  @select="chooseProvider(provider.id)"
                >
                  <ModelSelectorName>{{ providerLabel(provider) }}</ModelSelectorName>
                  <CheckIcon
                    v-if="provider.id === selectedProviderId"
                    class="size-4 text-primary"
                  />
                </ModelSelectorItem>
              </ModelSelectorGroup>
            </ModelSelectorList>
          </ModelSelectorContent>
        </ModelSelector>
        <span v-else class="px-2 text-xs text-muted-foreground">未配置模型</span>
      </PromptInputTools>
      <div class="flex items-center gap-2">
        <span class="hidden font-mono text-[10px] text-muted-foreground/70 sm:inline">
          Enter 发送
        </span>
        <PromptInputSubmit
          :status="pending ? 'submitted' : undefined"
          :disabled="!canSubmit"
          aria-label="发送"
          class="chat-send-button size-9 rounded-full"
        >
          <Loader2Icon v-if="pending" class="size-4 animate-spin" />
          <ArrowUpIcon v-else class="size-4" />
        </PromptInputSubmit>
      </div>
    </PromptInputFooter>
  </PromptInput>
</template>
