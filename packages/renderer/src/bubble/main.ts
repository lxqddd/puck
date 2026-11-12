import { createApp } from 'vue'
import BubbleApp from './BubbleApp.vue'
import '../shared/tailwind.css'
import '../shared/base.scss'
import './bubble.scss'
import { initTheme } from '../shared/theme'

void initTheme()
createApp(BubbleApp).mount('#app')
