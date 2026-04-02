import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useSystemStore } from '@/stores/system'
import client from '@/api/client.js'
import './styles/main.css'

// On Windows, WebView2 treats http:// server requests as mixed content
// because the Tauri app origin is https://tauri.localhost.
//
// @tauri-apps/plugin-http exposes a fetch() that routes requests through
// the native Rust HTTP client, bypassing the WebView's mixed-content check.
// Two things must happen before the app boots:
//
//   1. window.fetch = tauriFetch  — so ConnectView's globalThis.fetch probe works.
//   2. client.defaults.adapter    — Axios captures window.fetch at *module load
//      time* via a default parameter, so patching window.fetch afterwards never
//      reaches Axios.  We use Axios's exported getFetch(env) factory to create a
//      fresh adapter that holds a direct reference to tauriFetch.
//
// This is Windows-only.  Linux (tauri://) and macOS have no mixed-content
// restriction and must NOT be changed (patching breaks Linux WebKitGTK).
async function init() {
  if (window.__TAURI_INTERNALS__ && navigator.userAgent.includes('Windows')) {
    const [{ fetch: tauriFetch }, { getFetch }] = await Promise.all([
      import('@tauri-apps/plugin-http'),
      import('axios/unsafe/adapters/fetch.js'),
    ])
    window.fetch = tauriFetch
    client.defaults.adapter = getFetch({ env: { fetch: tauriFetch, Request, Response } })
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
