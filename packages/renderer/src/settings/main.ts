import { createApp } from 'vue'
import SettingsApp from './SettingsApp.vue'
import '../shared/tailwind.css'
import '../shared/base.scss'
import './settings.scss'
import { initTheme } from '../shared/theme'

void initTheme()
createApp(SettingsApp).mount('#app')
