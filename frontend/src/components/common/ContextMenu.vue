<template>
  <Teleport to="body">
    <div ref="probeEl" class="ctx-menu-probe" aria-hidden="true"></div>
    <ul
      ref="menuEl"
      class="ctx-menu"
      role="menu"
      tabindex="-1"
      :style="{ visibility: ready ? 'visible' : 'hidden' }"
      @keydown="onKeyDown"
    >
      <li v-for="(item, i) in items" :key="item.key" role="none">
        <button
          role="menuitem"
          type="button"
          class="ctx-menu-item"
          :class="{ 'ctx-menu-item-danger': item.danger }"
          :tabindex="i === activeIndex ? 0 : -1"
          :disabled="item.disabled"
          @click="select(item)"
          @mouseenter="activeIndex = i"
        >{{ item.label }}</button>
      </li>
    </ul>
  </Teleport>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'

const props = defineProps({
  x: { type: Number, required: true },
  y: { type: Number, required: true },
  items: { type: Array, required: true }, // [{ key, label, danger?, disabled? }]
})
const emit = defineEmits(['select', 'close'])

const menuEl = ref(null)
const probeEl = ref(null)
const ready = ref(false)
const activeIndex = ref(props.items.findIndex(i => !i.disabled))

// The event's clientX/clientY (and getBoundingClientRect()) don't necessarily share
// the CSS pixel space of a fixed element's left/top: the app zoom (CSS `zoom` on
// <html>, see App.vue) scales left/top but not the pointer coordinates, so placing the
// menu at the raw click point drifts further off the lower/righter you click, and a
// clamp against window.innerHeight checks the wrong space. Measure instead: where the
// menu lands for two known left/top values gives the live scale and origin, and a
// full-viewport fixed probe gives the viewport bounds in that same measured space.
// Engines also disagree on whether getBoundingClientRect() itself is zoomed (Chromium
// 128+ yes, WebKitGTK no) while clientX/Y and window.innerHeight never are, so the
// probe's height vs innerHeight converts the click point into the measured space too.
function clampPosition() {
  const el = menuEl.value
  if (!el) return
  el.style.left = '0px'
  el.style.top = '0px'
  const r0 = el.getBoundingClientRect()
  el.style.left = '100px'
  el.style.top = '100px'
  const r1 = el.getBoundingClientRect()
  const sx = (r1.left - r0.left) / 100 || 1
  const sy = (r1.top - r0.top) / 100 || 1
  const vp = probeEl.value?.getBoundingClientRect()
    || { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight }
  const k = window.innerHeight ? (vp.bottom - vp.top) / window.innerHeight || 1 : 1
  const margin = 8
  let left = props.x * k
  let top = props.y * k
  if (left + r0.width > vp.right - margin) left = vp.right - r0.width - margin
  if (top + r0.height > vp.bottom - margin) top = vp.bottom - r0.height - margin
  left = Math.max(vp.left + margin, left)
  top = Math.max(vp.top + margin, top)
  el.style.left = (left - r0.left) / sx + 'px'
  el.style.top = (top - r0.top) / sy + 'px'
}

function select(item) {
  if (item.disabled) return
  emit('select', item.key)
}

function focusActive() {
  const buttons = menuEl.value?.querySelectorAll('button[role="menuitem"]')
  buttons?.[activeIndex.value]?.focus()
}

function moveActive(delta) {
  const count = props.items.length
  if (!count) return
  let idx = activeIndex.value
  for (let step = 0; step < count; step++) {
    idx = (idx + delta + count) % count
    if (!props.items[idx].disabled) break
  }
  activeIndex.value = idx
  focusActive()
}

function onKeyDown(e) {
  if (e.key === 'ArrowDown') { e.preventDefault(); moveActive(1); return }
  if (e.key === 'ArrowUp') { e.preventDefault(); moveActive(-1); return }
  if (e.key === 'Home') { e.preventDefault(); activeIndex.value = -1; moveActive(1); return }
  if (e.key === 'End') { e.preventDefault(); activeIndex.value = 0; moveActive(-1); return }
}

// Escape must close the menu even when focus didn't move into it (after a right-click
// focus often stays on the element that was clicked), so listen on the document too.
function onDocKeyDown(e) {
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); emit('close') }
}

function onDocMouseDown(e) {
  if (menuEl.value && !menuEl.value.contains(e.target)) emit('close')
}

onMounted(async () => {
  await nextTick()
  clampPosition()
  ready.value = true
  focusActive()
  document.addEventListener('mousedown', onDocMouseDown)
  document.addEventListener('keydown', onDocKeyDown, true)
})

onUnmounted(() => {
  document.removeEventListener('mousedown', onDocMouseDown)
  document.removeEventListener('keydown', onDocKeyDown, true)
})
</script>

<style scoped>
.ctx-menu-probe {
  position: fixed;
  inset: 0;
  visibility: hidden;
  pointer-events: none;
}

.ctx-menu {
  position: fixed;
  z-index: 2000;
  min-width: 160px;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-md);
}

.ctx-menu-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 8px 12px;
  border: none;
  background: none;
  color: var(--color-text);
  font-size: 13px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.ctx-menu-item:hover,
.ctx-menu-item:focus-visible {
  background: var(--color-bg);
  outline: none;
}

.ctx-menu-item:disabled {
  color: var(--color-text-muted);
  cursor: not-allowed;
}

.ctx-menu-item-danger {
  color: var(--color-danger);
}
</style>
