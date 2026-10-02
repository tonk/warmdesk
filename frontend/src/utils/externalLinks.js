// External links in the desktop app (Tauri) must open in the user's default
// browser. Left to the WebView, a plain <a href> replaces the app itself with
// the target page (with no way back), and target="_blank" is not reliably
// honoured either.

const EXTERNAL_SCHEMES = ['http:', 'https:', 'mailto:', 'tel:']

export function isTauri() {
  return typeof window !== 'undefined' && !!window.__TAURI_INTERNALS__
}

// Returns true when `href` points outside the running app.
export function isExternalUrl(href, origin = window.location.origin) {
  let url
  try {
    url = new URL(href, origin)
  } catch {
    return false
  }
  if (!EXTERNAL_SCHEMES.includes(url.protocol)) return false
  if (url.protocol === 'mailto:' || url.protocol === 'tel:') return true
  return url.origin !== origin
}

export async function openExternal(url) {
  if (isTauri()) {
    await window.__TAURI_INTERNALS__.invoke('plugin:opener|open_url', { url, with: null })
  } else {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

function onDocumentClick(event) {
  if (event.defaultPrevented || event.button > 1) return
  const anchor = event.target?.closest?.('a[href]')
  if (!anchor || anchor.hasAttribute('download')) return
  const href = anchor.href
  if (!isExternalUrl(href)) return
  event.preventDefault()
  openExternal(href).catch(() => {})
}

// Installs a document-level handler that routes clicks on external links to
// the default browser. Runs in the bubble phase so component handlers that
// call preventDefault() (e.g. card-reference links) keep precedence.
export function installExternalLinkHandler() {
  if (!isTauri()) return
  document.addEventListener('click', onDocumentClick)
  document.addEventListener('auxclick', onDocumentClick)
}
