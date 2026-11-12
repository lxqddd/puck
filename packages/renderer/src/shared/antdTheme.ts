import { computed } from 'vue'
import { theme as antdTheme } from 'ant-design-vue'
import { useTheme } from './theme'

export function useAntTheme() {
  const { isDark } = useTheme()

  return computed(() => ({
    algorithm: isDark.value ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary: isDark.value ? '#5fd8b4' : '#35bd98',
      colorText: isDark.value ? '#f7f2e9' : '#17342d',
      colorTextSecondary: isDark.value ? '#b9ab9b' : '#596c65',
      colorTextDescription: isDark.value ? '#b9ab9b' : '#596c65',
      colorTextPlaceholder: isDark.value ? '#b9ab9b' : '#596c65',
      colorBgLayout: isDark.value ? '#1a1511' : '#fbf7ef',
      colorBgContainer: isDark.value ? '#2d261f' : '#ffffff',
      colorBgElevated: isDark.value ? '#362e25' : '#ffffff',
      colorBorder: isDark.value ? 'rgba(247, 242, 233, 0.12)' : 'rgba(23, 52, 45, 0.12)',
      colorBorderSecondary: isDark.value
        ? 'rgba(247, 242, 233, 0.08)'
        : 'rgba(23, 52, 45, 0.08)',
      colorFillSecondary: isDark.value
        ? 'rgba(247, 242, 233, 0.06)'
        : 'rgba(23, 52, 45, 0.06)',
      colorFillTertiary: isDark.value
        ? 'rgba(247, 242, 233, 0.04)'
        : 'rgba(23, 52, 45, 0.04)',
      borderRadius: 12,
      fontFamily:
        "'SF Pro Rounded', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', system-ui, sans-serif",
    },
  }))
}
