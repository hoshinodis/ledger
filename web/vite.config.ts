import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Dev server proxies API calls to a locally running ledger server.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
