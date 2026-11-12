import { createApp } from 'vue'
import MascotApp from './MascotApp.vue'
import '../shared/base.css'
import './mascot.css'

createApp(MascotApp).mount('#app')
document.getElementById('boot')?.remove()
