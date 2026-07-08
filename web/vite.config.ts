import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

// Masha agent yerel yuz (127.0.0.1). Uretimde Go binary dist/'i EMBED eder + ayni origin'de
// /healthz /try /schema JSON API sunar. Dev'de bu uclar Go agent'a (8787) proxy'lenir.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, 'src') },
  },
  build: { outDir: 'dist', emptyOutDir: true },
  server: {
    proxy: {
      '/healthz': 'http://127.0.0.1:8787',
      '/try': 'http://127.0.0.1:8787',
      '/schema': 'http://127.0.0.1:8787',
    },
  },
})
