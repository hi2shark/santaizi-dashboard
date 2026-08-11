import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  base: '/admin/',
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: {
    outDir: '../../../resource/web/admin',
    emptyOutDir: true,
  },
  server: { proxy: { '/api': 'http://127.0.0.1:8000', '/oauth2': 'http://127.0.0.1:8000', '/ws': { target: 'ws://127.0.0.1:8000', ws: true }, '/static': 'http://127.0.0.1:8000' } },
})
