<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { bridge } from '../bridge'

const baseURL = ref('https://api.deepseek.com/v1')
const model = ref('deepseek-chat')
const apiKey = ref('')
const hasKey = ref(false)
const mascotScale = ref(1)
const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const errorText = ref('')

onMounted(async () => {
  try {
    const settings = await bridge.getSettings()
    if (settings.provider) {
      baseURL.value = settings.provider.baseURL || baseURL.value
      model.value = settings.provider.model || model.value
      hasKey.value = settings.provider.hasKey
    }
    mascotScale.value = settings.mascotScale > 0 ? settings.mascotScale : 1
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  } finally {
    loading.value = false
  }
})

async function save() {
  saving.value = true
  errorText.value = ''
  saved.value = false
  try {
    await bridge.saveSettings(
      {
        mascotScale: mascotScale.value,
        provider: {
          id: 'default',
          name: '默认模型',
          type: 'openai-compatible',
          baseURL: baseURL.value.trim(),
          model: model.value.trim(),
          hasKey: hasKey.value,
        },
      },
      apiKey.value,
    )
    apiKey.value = ''
    const settings = await bridge.getSettings()
    hasKey.value = settings.provider?.hasKey ?? false
    saved.value = true
    window.setTimeout(() => {
      saved.value = false
    }, 2400)
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="settings-app">
    <header class="settings-header">
      <h1>设置</h1>
      <p>接入任意 OpenAI 兼容服务；API Key 保存在系统钥匙串。</p>
    </header>

    <form v-if="!loading" class="settings-form" @submit.prevent="save">
      <label>
        <span>服务地址</span>
        <input v-model="baseURL" placeholder="https://api.deepseek.com/v1" autocomplete="off" />
      </label>

      <label>
        <span>模型</span>
        <input v-model="model" placeholder="deepseek-chat" autocomplete="off" />
      </label>

      <label>
        <span>API Key <em v-if="hasKey">已保存</em></span>
        <input
          v-model="apiKey"
          type="password"
          :placeholder="hasKey ? '留空保持不变' : 'sk-…'"
          autocomplete="off"
        />
      </label>

      <p class="hint">支持 OpenAI、DeepSeek、通义、Kimi、智谱、Ollama 等兼容端点。</p>

      <div class="actions">
        <button type="submit" class="primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
        <span v-if="saved" class="saved">已保存</span>
      </div>

      <p v-if="errorText" class="error">{{ errorText }}</p>
    </form>
  </div>
</template>
