import { createRouter, createWebHashHistory } from 'vue-router'
import ChatView from '../chat/ChatApp.vue'
import HistoryView from '../history/HistoryView.vue'
import SettingsView from '../settings/SettingsApp.vue'

export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/chat' },
    { path: '/chat/:id?', name: 'chat', component: ChatView },
    { path: '/history', name: 'history', component: HistoryView },
    { path: '/settings', name: 'settings', component: SettingsView },
    { path: '/:pathMatch(.*)*', redirect: '/chat' },
  ],
})
