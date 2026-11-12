import { createApp } from 'vue'
import MascotApp from './MascotApp.vue'
import '../shared/tailwind.css'
import '../shared/base.scss'
import './mascot.scss'

createApp(MascotApp).mount('#app')
document.getElementById('boot')?.remove()
