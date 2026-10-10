<template>
  <div ref="rootEl" class="hcm-wrap">
    <button
      ref="buttonEl"
      type="button"
      :class="buttonClass"
      class="hcm-button"
      :aria-expanded="String(open)"
      aria-haspopup="menu"
      :aria-label="ariaLabel || undefined"
      :disabled="disabled"
      @click="toggle"
      @keydown.down.prevent="openMenu(0)"
    >
      {{ label }}<span class="hcm-chevron" :class="{ open }" aria-hidden="true">›</span>
    </button>
    <div v-if="open" class="hcm-menu" :class="`hcm-align-${align}`" role="menu" @keydown="onMenuKey">
      <button
        v-for="(c, i) in HOLIDAY_COUNTRIES" :key="c.locale"
        :ref="el => itemEls[i] = el"
        type="button"
        class="hcm-item"
        role="menuitem"
        tabindex="-1"
        @click="choose(c.locale)"
      >
        <span v-if="isTauri" class="hcm-flag" :data-country="c.flag" aria-hidden="true"></span>
        <span v-else class="hcm-flag-emoji" aria-hidden="true">{{ c.emoji }}</span>{{ c.label }}
      </button>
    </div>
  </div>
</template>

<script setup>
// A button that opens a list of countries with public-holiday tables (the
// locales the backend's holiday import knows). Used by the time-tracking
// "Add holidays" button and the business-hours calendar editor.
import { ref, nextTick, onMounted, onBeforeUnmount } from 'vue'

defineProps({
  label: { type: String, required: true },
  ariaLabel: { type: String, default: '' },
  buttonClass: { type: [String, Array, Object], default: '' },
  align: { type: String, default: 'right' }, // which edge of the button the menu lines up with
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['select'])

const HOLIDAY_COUNTRIES = [
  { locale: 'en', flag: 'GB', emoji: '🇬🇧', label: 'United Kingdom' },
  { locale: 'nl', flag: 'NL', emoji: '🇳🇱', label: 'Nederland' },
  { locale: 'de', flag: 'DE', emoji: '🇩🇪', label: 'Deutschland' },
  { locale: 'fr', flag: 'FR', emoji: '🇫🇷', label: 'France' },
  { locale: 'es', flag: 'ES', emoji: '🇪🇸', label: 'España' },
  { locale: 'da', flag: 'DK', emoji: '🇩🇰', label: 'Danmark' },
  { locale: 'sv', flag: 'SE', emoji: '🇸🇪', label: 'Sverige' },
  { locale: 'nb', flag: 'NO', emoji: '🇳🇴', label: 'Norge' },
  { locale: 'fi', flag: 'FI', emoji: '🇫🇮', label: 'Suomi' },
  { locale: 'is', flag: 'IS', emoji: '🇮🇸', label: 'Ísland' },
  { locale: 'pt', flag: 'PT', emoji: '🇵🇹', label: 'Portugal' },
  { locale: 'it', flag: 'IT', emoji: '🇮🇹', label: 'Italia' },
]
// WebKitGTK (the Linux desktop app) has no flag emoji, so draw flags in CSS there.
const isTauri = !!window.__TAURI_INTERNALS__

const open = ref(false)
const rootEl = ref(null)
const buttonEl = ref(null)
const itemEls = []

function openMenu(focusIndex = null) {
  open.value = true
  if (focusIndex !== null) nextTick(() => itemEls[focusIndex]?.focus())
}
function close(returnFocus = false) {
  open.value = false
  if (returnFocus) buttonEl.value?.focus()
}
function toggle() {
  if (open.value) close()
  else openMenu(0)
}
function choose(locale) {
  close(true)
  emit('select', locale)
}
function onMenuKey(e) {
  const i = itemEls.indexOf(document.activeElement)
  const last = HOLIDAY_COUNTRIES.length - 1
  const next = { ArrowDown: i < last ? i + 1 : 0, ArrowUp: i > 0 ? i - 1 : last, Home: 0, End: last }[e.key]
  if (next !== undefined) {
    e.preventDefault()
    itemEls[next]?.focus()
  } else if (e.key === 'Tab') {
    close()
  }
}
function onDocMouseDown(e) {
  if (open.value && rootEl.value && !rootEl.value.contains(e.target)) close()
}
// Capture phase, so Escape closes this menu before a surrounding view or
// dialog reacts to it.
function onDocKey(e) {
  if (open.value && e.key === 'Escape') {
    e.stopPropagation()
    close(true)
  }
}
onMounted(() => {
  document.addEventListener('mousedown', onDocMouseDown)
  document.addEventListener('keydown', onDocKey, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMouseDown)
  document.removeEventListener('keydown', onDocKey, true)
})
</script>

<style scoped>
.hcm-wrap { position: relative; display: inline-block; }
.hcm-button { display: inline-flex; align-items: center; gap: 4px; }
.hcm-chevron { display: inline-block; font-style: normal; font-size: 11px; transform: rotate(90deg); transition: transform .15s; line-height: 1; }
.hcm-chevron.open { transform: rotate(-90deg); }
.hcm-menu {
  position: absolute; top: calc(100% + 4px);
  min-width: 180px; padding: 4px 0;
  background: var(--color-surface); border: 1px solid var(--color-border);
  border-radius: var(--radius-sm); box-shadow: 0 4px 12px color-mix(in srgb, var(--color-text) 15%, transparent); z-index: 200;
}
.hcm-align-right { right: 0; }
.hcm-align-left { left: 0; }
.hcm-item {
  display: flex; align-items: center; gap: 8px; width: 100%;
  padding: 7px 14px; border: none; background: none;
  font-size: 13px; color: var(--color-text); cursor: pointer; text-align: left;
}
.hcm-item:hover, .hcm-item:focus-visible { background: color-mix(in srgb, var(--color-primary) 10%, transparent); outline: none; }
.hcm-flag {
  display: inline-block; width: 26px; height: 17px; border-radius: 2px; flex-shrink: 0;
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--color-text) 18%, transparent);
}
.hcm-flag-emoji { font-size: 18px; line-height: 1; flex-shrink: 0; }
/* National flag colours (approximations), deliberately not theme colours. */
/* Horizontal tricolors */
.hcm-flag[data-country="NL"] { background: linear-gradient(to bottom, #AE1C28 0% 33.4%, #fff 33.4% 66.6%, #21468B 66.6% 100%); }
.hcm-flag[data-country="DE"] { background: linear-gradient(to bottom, #222 0% 33.4%, #DD0000 33.4% 66.6%, #FFCE00 66.6% 100%); }
.hcm-flag[data-country="ES"] { background: linear-gradient(to bottom, #AA151B 0% 25%, #F1BF00 25% 75%, #AA151B 75% 100%); }
/* Vertical tricolors */
.hcm-flag[data-country="FR"] { background: linear-gradient(to right, #002395 0% 33.4%, #fff 33.4% 66.6%, #ED2939 66.6% 100%); }
.hcm-flag[data-country="IT"] { background: linear-gradient(to right, #009246 0% 33.4%, #fff 33.4% 66.6%, #CE2B37 66.6% 100%); }
.hcm-flag[data-country="PT"] { background: linear-gradient(to right, #006600 0% 38%, #CC0000 38% 100%); }
/* Nordic cross / other — approximated as bicolour split */
.hcm-flag[data-country="GB"] { background: linear-gradient(135deg, #012169 0% 50%, #CF101A 50% 100%); }
.hcm-flag[data-country="DK"] { background: linear-gradient(to right, #C60C30 0% 35%, #fff 35% 45%, #C60C30 45% 100%); }
.hcm-flag[data-country="SE"] { background: linear-gradient(to right, #006AA7 0% 35%, #FECC00 35% 45%, #006AA7 45% 100%); }
.hcm-flag[data-country="NO"] { background: linear-gradient(to right, #EF2B2D 0% 30%, #fff 30% 40%, #002868 40% 48%, #fff 48% 58%, #EF2B2D 58% 100%); }
.hcm-flag[data-country="FI"] { background: linear-gradient(to right, #fff 0% 35%, #003580 35% 45%, #fff 45% 100%); }
.hcm-flag[data-country="IS"] { background: linear-gradient(to right, #003897 0% 30%, #fff 30% 40%, #DC143C 40% 48%, #fff 48% 58%, #003897 58% 100%); }
</style>
