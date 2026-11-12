import { computed, ref } from 'vue'
import type { ThemeMode } from '@puck/contract'
import { bridge } from '../bridge'

const mode = ref<ThemeMode>('system')
const systemDark = ref(false)
let started = false

export const themeMode = computed(() => mode.value)
export const isDark = computed(
  () => mode.value === 'dark' || (mode.value === 'system' && systemDark.value),
)

function applyTheme() {
  const dark = isDark.value
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
}

export async function initTheme() {
  if (started) return
  started = true

  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  systemDark.value = mediaQuery.matches
  mediaQuery.addEventListener('change', () => {
    systemDark.value = mediaQuery.matches
    applyTheme()
  })
  applyTheme()

  try {
    const settings = await bridge.getSettings()
    mode.value = settings.theme || 'system'
  } catch {
    mode.value = 'system'
  }

  try {
    bridge.onThemeChanged((event) => {
      mode.value = event.theme
      applyTheme()
    })
  } catch {
    // 浏览器预览没有 Wails 运行时，保持本地主题即可。
  }

  applyTheme()
}

export async function setTheme(next: ThemeMode) {
  mode.value = next
  applyTheme()
  try {
    await bridge.setTheme(next)
  } catch {
    // 即使持久化失败，当前窗口仍先应用新主题。
  }
}

export function useTheme() {
  return { mode: themeMode, isDark, setTheme }
}
