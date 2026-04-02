import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useSystemStore } from '@/stores/system'
import './styles/main.css'

// On Windows, WebView2 treats http:// server requests as mixed content
// because the Tauri app origin is https://tauri.localhost.  We patch
// window.fetch via tauri-plugin-http so the native Rust HTTP client handles
// all requests.  This is Windows-only: Linux (tauri://) and macOS have no
// mixed-content restriction and must NOT be patched (causes blank screen on
// WebKitGTK).  The import must be awaited before the app mounts so Axios
// already sees the patched fetch when it makes its first request.
async function init() {
  if (window.__TAURI_INTERNALS__ && navigator.userAgent.includes('Windows')) {
    await import('@tauri-apps/plugin-http')
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
}

init()
