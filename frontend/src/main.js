import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useSystemStore } from '@/stores/system'
import './styles/main.css'

// On Windows, WebView2 blocks http:// requests from the https://tauri.localhost
// origin as mixed content.  index.html installs a window.fetch proxy before the
// ES module bundle loads (so Axios captures the proxy, not the native fetch).
// Here we point that proxy at the tauri-plugin-http fetch, which routes requests
// through the native Rust HTTP client.  Linux and macOS are unaffected because
// we only set window.__tauriFetch on Windows.
async function init() {
  if (window.__TAURI_INTERNALS__ && navigator.userAgent.includes('Windows')) {
    const { fetch: tauriFetch } = await import('@tauri-apps/plugin-http')
    window.__tauriFetch = tauriFetch
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
