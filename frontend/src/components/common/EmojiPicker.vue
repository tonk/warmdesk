<template>
  <div
    ref="el"
    class="emoji-picker"
    role="dialog"
    aria-modal="true"
    :aria-label="t('common.emoji_picker')"
    @keydown="onPickerKeyDown"
  >
    <div role="group" class="emoji-row">
      <button
        v-for="e in emojis"
        :key="e"
        type="button"
        class="emoji-btn"
        :aria-label="emojiLabel(e)"
        @click="$emit('pick', e)"
      >{{ e }}</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'

const emit = defineEmits(['pick', 'close'])
const { t } = useI18n()

const emojis = ['👍', '❤️', '😂', '😮', '😢', '👎']

const EMOJI_KEYS = {
  '👍': 'thumbs_up',
  '❤️': 'heart',
  '😂': 'laughing',
  '😮': 'surprised',
  '😢': 'sad',
  '👎': 'thumbs_down',
}

function emojiLabel(e) {
  return EMOJI_KEYS[e] ? t('common.emoji_' + EMOJI_KEYS[e]) : e
}
const el = ref(null)
let opener = null

function buttons() {
  return el.value ? Array.from(el.value.querySelectorAll('.emoji-btn')) : []
}

function onClickOutside(e) {
  if (el.value && !el.value.contains(e.target)) {
    Promise.resolve().then(() => emit('close'))
  }
}

// Escape closes; arrow keys / Home / End move between emojis; Tab is trapped
// inside the picker (WCAG 2.1.1 Keyboard, 2.4.3 Focus Order).
function onPickerKeyDown(e) {
  const btns = buttons()
  if (!btns.length) return
  const i = btns.indexOf(document.activeElement)
  let next = null
  if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next = btns[(i + 1) % btns.length]
  else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') next = btns[(i - 1 + btns.length) % btns.length]
  else if (e.key === 'Home') next = btns[0]
  else if (e.key === 'End') next = btns[btns.length - 1]
  else if (e.key === 'Tab') next = e.shiftKey ? btns[(i - 1 + btns.length) % btns.length] : btns[(i + 1) % btns.length]
  if (next) {
    e.preventDefault()
    next.focus()
  }
}

function onKeyDown(e) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => {
  opener = document.activeElement
  document.addEventListener('mousedown', onClickOutside)
  document.addEventListener('keydown', onKeyDown)
  buttons()[0]?.focus()
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onClickOutside)
  document.removeEventListener('keydown', onKeyDown)
  if (opener && typeof opener.focus === 'function' && document.contains(opener)) opener.focus()
})
</script>

<style scoped>
.emoji-row { display: flex; gap: 2px; }
.emoji-picker {
  padding: 4px 6px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 20px;
  box-shadow: var(--shadow-md, 0 4px 12px rgba(0,0,0,.1));
  position: absolute;
  z-index: 300;
  bottom: 100%;
  left: 0;
  margin-bottom: 4px;
  white-space: nowrap;
}
.emoji-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 18px;
  padding: 2px 3px;
  border-radius: 6px;
  line-height: 1;
  transition: transform .1s;
}
.emoji-btn:hover { transform: scale(1.3); background: var(--color-bg); }
.emoji-btn:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 1px; background: var(--color-bg); }
</style>
