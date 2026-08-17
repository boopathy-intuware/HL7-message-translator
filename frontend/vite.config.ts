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
      // Scoped to the backend's actual /fhir/Patient and /fhir/Observation
      // routes, not a bare '/fhir' prefix — the frontend also owns the
      // page route /fhir (FhirExplorerPage), and a prefix match would
      // proxy a hard reload of that page straight to the backend, which
      // has no route for the bare path and 404s.
      '/fhir/Patient': process.env.BACKEND_URL ?? 'http://localhost:8080',
      '/fhir/Observation': process.env.BACKEND_URL ?? 'http://localhost:8080',
    },
  },
})
