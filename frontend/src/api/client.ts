import axios from 'axios'

// Relative baseURL: in dev this goes through the Vite proxy (vite.config.ts)
// to the Go backend on :8080; override via VITE_API_BASE_URL if the
// frontend and backend are served from different origins in production.
const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '',
})

export default apiClient
