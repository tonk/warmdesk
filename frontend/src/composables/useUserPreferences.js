/**
 * Applies user preferences (font, font-size, sidebar position, theme) to the document.
 * Falls back to global system defaults when the user has not set a preference.
 * Call applyUserPreferences(user) whenever the user object changes.
 */
import { useSystemStore } from '@/stores/system'

// Maps font name → Google Fonts family string
const GOOGLE_FONTS = {
  'Inter': 'Inter:wght@400;500;600;700',
  'Roboto': 'Roboto:wght@400;500;700',
  'Open Sans': 'Open+Sans:wght@400;500;600;700',
  'Source Code Pro': 'Source+Code+Pro:wght@400;500;600',
}

// Extract the first font name from a CSS font-family value like "'Open Sans', sans-serif"
function extractFontName(cssValue) {
  const m = cssValue.match(/^['"]?([^'",]+)['"]?/)
  return m ? m[1].trim() : null
}

function loadGoogleFont(fontName) {
  const family = GOOGLE_FONTS[fontName]
  if (!family) return
  const id = `gfont-${fontName.replace(/\s+/g, '-').toLowerCase()}`
  if (document.getElementById(id)) return
  const link = document.createElement('link')
  link.id = id
  link.rel = 'stylesheet'
  link.href = `https://fonts.googleapis.com/css2?family=${family}&display=swap`
  document.head.appendChild(link)
}

export function applyUserPreferences(user) {
  if (!user) return

  const root = document.documentElement
  const systemStore = useSystemStore()
  const sysDefaults = systemStore.defaults

  // Font family — user value takes priority, fall back to system default
  const font = user.font || sysDefaults.font || 'system'
  if (font === 'system') {
    root.style.removeProperty('--user-font')
  } else {
    loadGoogleFont(extractFontName(font))
    root.style.setProperty('--user-font', font)
  }

  // Font size — user value takes priority, fall back to system default
  const size = parseInt(user.font_size || sysDefaults.font_size) || 14
  root.style.setProperty('--user-font-size', `${size}px`)

  // Sidebar position — stored in localStorage so App.vue can react
  const pos = user.sidebar_position === 'right' ? 'right' : 'left'
  localStorage.setItem('sidebar_position', pos)
  root.setAttribute('data-sidebar', pos)
}
