import axios from 'axios'
import { getServerUrl } from './serverConfig'

// Base URL is resolved at request time so it picks up runtime server config.
// Falls back to relative '/api/v1' for the normal browser / Vite-proxy workflow.
function apiBase() {
  const server = getServerUrl()
  return server ? `${server}/api/v1` : '/api/v1'
}

// In the Tauri desktop app, Axios defaults to XHR which WebView2 treats as
// mixed content when the app origin is https://tauri.localhost but the server
// is plain HTTP.  Switching to the fetch adapter lets tauri-plugin-http
// intercept the call and use a native HTTP client, bypassing the restriction.
const isTauri = typeof window !== 'undefined' && !!window.__TAURI_INTERNALS__

const client = axios.create({
  headers: { 'Content-Type': 'application/json' },
  ...(isTauri ? { adapter: 'fetch' } : {})
})

let isRefreshing = false
let refreshQueue = []

function processQueue(error, token = null) {
  refreshQueue.forEach(({ resolve, reject }) => {
    if (error) reject(error)
    else resolve(token)
  })
  refreshQueue = []
}

client.interceptors.request.use(config => {
  config.baseURL = apiBase()
  const token = sessionStorage.getItem('access_token')
    || (client.defaults.headers.common.Authorization || '').replace('Bearer ', '')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

client.interceptors.response.use(
  response => response,
  async error => {
    const original = error.config
    if (error.response?.status === 401 && !original._retry) {
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          refreshQueue.push({ resolve, reject })
        }).then(token => {
          original.headers.Authorization = `Bearer ${token}`
          return client(original)
        })
      }

      original._retry = true
      isRefreshing = true

      const refreshToken = sessionStorage.getItem('refresh_token')
      if (!refreshToken) {
        isRefreshing = false
        sessionStorage.removeItem('access_token')
        sessionStorage.removeItem('refresh_token')
        window.location.href = '/login'
        return Promise.reject(error)
      }

      try {
        const { data } = await axios.post(`${apiBase()}/auth/refresh`, { refresh_token: refreshToken })
        sessionStorage.setItem('access_token', data.access_token)
        sessionStorage.setItem('refresh_token', data.refresh_token)
        client.defaults.headers.common.Authorization = `Bearer ${data.access_token}`
        processQueue(null, data.access_token)
        original.headers.Authorization = `Bearer ${data.access_token}`
        return client(original)
      } catch (err) {
        processQueue(err, null)
        sessionStorage.removeItem('access_token')
        sessionStorage.removeItem('refresh_token')
        window.location.href = '/login'
        return Promise.reject(err)
      } finally {
        isRefreshing = false
      }
    }
    return Promise.reject(error)
  }
)

export default client
