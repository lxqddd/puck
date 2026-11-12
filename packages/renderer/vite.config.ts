import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import wails from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  plugins: [vue(), wails('./src/bindings')],
  build: {
    outDir: fileURLToPath(new URL('../../apps/desktop-go/frontend/dist', import.meta.url)),
    emptyOutDir: true,
    rollupOptions: {
      input: {
        index: fileURLToPath(new URL('index.html', import.meta.url)),
        mascot: fileURLToPath(new URL('mascot.html', import.meta.url)),
        chat: fileURLToPath(new URL('chat.html', import.meta.url)),
        settings: fileURLToPath(new URL('settings.html', import.meta.url)),
      },
    },
  },
  server: {
    port: 34115,
    strictPort: true,
    fs: {
      allow: [fileURLToPath(new URL('../..', import.meta.url))],
    },
  },
})
