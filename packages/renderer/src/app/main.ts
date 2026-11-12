import { createApp } from 'vue'
import AppShell from './AppShell.vue'
import { router } from './router'
import 'ant-design-vue/dist/reset.css'
import '../shared/tailwind.css'
import '../shared/base.scss'
import '../chat/chat.scss'
import '../settings/settings.scss'
import '../history/history.scss'
import { initTheme } from '../shared/theme'

void initTheme()
createApp(AppShell).use(router).mount('#app')
