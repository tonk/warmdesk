import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useSystemStore } from '@/stores/system'
import './styles/main.css'

// In the Tauri desktop app, patch window.fetch so all HTTP requests
// (including those from Axios) are routed through the native Rust HTTP
// client. This bypasses WebView2's mixed-content restrictions when the app
// origin is https://tauri.localhost but the server is plain HTTP.
if (window.__TAURI_INTERNALS__) {
  import('@tauri-apps/plugin-http').catch(() => {})
}

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(i18n)

app.config.errorHandler = (err, _instance, info) => {
  console.error('[Vue error]', info, err)
}

app.mount('#app')

useSystemStore().fetchSettings()
