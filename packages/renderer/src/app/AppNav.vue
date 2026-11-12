<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { Tooltip } from 'ant-design-vue'

const route = useRoute()
const router = useRouter()

const items = [
  { name: 'chat', label: '聊天' },
  { name: 'history', label: '历史' },
] as const
</script>

<template>
  <nav
    class="app-nav flex h-full w-16 flex-none flex-col items-center border-r border-puck-line bg-puck-panel"
  >
    <div class="h-[38px] w-full flex-none [-webkit-app-region:drag]"></div>
    <div class="flex w-full flex-col items-center gap-2 py-2 [-webkit-app-region:no-drag]">
      <Tooltip v-for="item in items" :key="item.name" :title="item.label" placement="right">
        <button
          type="button"
          class="app-nav-item"
          :class="{ 'app-nav-item--active': route.name === item.name }"
          :aria-label="item.label"
          @click="router.push({ name: item.name })"
        >
          <svg
            v-if="item.name === 'chat'"
            class="h-5 w-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4z" />
          </svg>
          <svg
            v-else
            class="h-5 w-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
            <path d="M3 3v5h5" />
            <path d="M12 7v5l3 2" />
          </svg>
        </button>
      </Tooltip>
    </div>
    <div class="mt-auto w-full pb-3 [-webkit-app-region:no-drag]">
      <div class="flex justify-center">
        <Tooltip title="设置" placement="right">
          <button
            type="button"
            class="app-nav-item"
            :class="{ 'app-nav-item--active': route.name === 'settings' }"
            aria-label="设置"
            @click="router.push({ name: 'settings' })"
          >
            <svg
              class="h-5 w-5"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <circle cx="12" cy="12" r="3" />
              <path
                d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1.08-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1.08 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
              />
            </svg>
          </button>
        </Tooltip>
      </div>
    </div>
  </nav>
</template>

<style scoped lang="scss">
.app-nav-item {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  color: var(--puck-muted);
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease;

  &:hover {
    background: color-mix(in srgb, var(--puck-mint) 10%, transparent);
    color: var(--puck-ink);
  }
}

.app-nav-item--active {
  background: color-mix(in srgb, var(--puck-mint) 16%, transparent);
  color: var(--puck-mint-deep);
}
</style>
