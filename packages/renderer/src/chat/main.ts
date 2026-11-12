import { createApp } from 'vue'
import ChatApp from './ChatApp.vue'
import '../shared/tailwind.css'
import 'ant-design-vue/dist/reset.css'
import '../shared/base.scss'
import './chat.scss'

createApp(ChatApp).mount('#app')
