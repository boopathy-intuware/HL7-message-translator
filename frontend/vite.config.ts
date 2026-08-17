import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // BACKEND_URL lets docker-compose point this at the "backend" service
      // by name; bare-metal `npm run dev` falls back to localhost.
      '/api': process.env.BACKEND_URL ?? 'http://localhost:8080',
    },
  },
})
