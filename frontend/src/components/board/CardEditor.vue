<template>
  <div class="editor-wrapper">
    <textarea ref="editorEl"></textarea>
    <!-- Mention dropdown — positioned over the editor -->
    <MentionDropdown
      v-if="mentionUsers.length"
      :users="mentionUsers"
      :active-index="mentionIndex"
      :style="mentionPos"
      class="editor-mention-dropdown"
      @pick="editorPickMention"
      @update:activeIndex="mentionIndex = $event"
    />
    <!-- Emoji picker — anchored to the emoji toolbar button -->
    <InlineEmojiPicker
      v-if="emojiOpen"
      class="editor-emoji-picker"
      @pick="onEmojiPick"
      @close="emojiOpen = false"
    />
  </div>
</template>

<script setup>
import { ref, watch, computed, onMounted, onBeforeUnmount } from 'vue'
import EasyMDE from 'easymde'
import 'easymde/dist/easymde.min.css'
import MentionDropdown from '@/components/common/MentionDropdown.vue'
import InlineEmojiPicker from '@/components/common/InlineEmojiPicker.vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'Write using Markdown...' },
  minHeight: { type: String, default: '150px' },
  users: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:modelValue'])

const editorEl = ref(null)
let mde = null

// ── Emoji ─────────────────────────────────────────────────────────────────────
const emojiOpen = ref(false)

function onEmojiPick(emoji) {
  if (!mde) return
  mde.codemirror.replaceSelection(emoji)
  mde.codemirror.focus()
  emojiOpen.value = false
}

// ── Mention state ─────────────────────────────────────────────────────────────
const mentionQuery = ref(null)
const mentionStart = ref(null)   // CodeMirror { line, ch } of the leading @
const mentionIndex = ref(0)
const mentionPos   = ref({ top: '0px', left: '0px' })

const mentionUsers = computed(() => {
  if (mentionQuery.value === null) return []
  const q = (mentionQuery.value || '').toLowerCase()
  return (props.users || []).filter(u =>
    u.username.toLowerCase().startsWith(q) ||
    (u.display_name || '').toLowerCase().startsWith(q)
  ).slice(0, 8)
})

function detectMention() {
  if (!mde) return
  const cm     = mde.codemirror
  const cursor = cm.getCursor()
  const line   = cm.getLine(cursor.line)
  const before = line.slice(0, cursor.ch)
  const m      = before.match(/@(\w*)$/)
  if (m) {
    mentionQuery.value = m[1]
    mentionStart.value = { line: cursor.line, ch: cursor.ch - m[0].length }
    mentionIndex.value = 0
    // Position relative to the editor wrapper
    const coords = cm.cursorCoords(true, 'local')
    mentionPos.value = { top: (coords.bottom + 4) + 'px', left: coords.left + 'px' }
  } else {
    mentionQuery.value = null
  }
}

function editorPickMention(user) {
  if (!mde || !mentionStart.value) return
  const cm   = mde.codemirror
  const text = '@' + user.username + ' '
  cm.replaceRange(text, mentionStart.value, cm.getCursor())
  mentionQuery.value = null
  cm.focus()
}

function handleCmKeydown(cm, e) {
  if (mentionQuery.value === null || !mentionUsers.value.length) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    mentionIndex.value = (mentionIndex.value + 1) % mentionUsers.value.length
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    mentionIndex.value = (mentionIndex.value - 1 + mentionUsers.value.length) % mentionUsers.value.length
  } else if (e.key === 'Enter' || e.key === 'Tab') {
    e.preventDefault()
    editorPickMention(mentionUsers.value[mentionIndex.value])
  } else if (e.key === 'Escape') {
    mentionQuery.value = null
  }
}

// ── EasyMDE setup ─────────────────────────────────────────────────────────────
onMounted(() => {
  mde = new EasyMDE({
    element: editorEl.value,
    initialValue: props.modelValue,
    placeholder: props.placeholder,
    spellChecker: false,
    autofocus: false,
    minHeight: props.minHeight,
    toolbar: [
      'bold', 'italic', 'strikethrough', '|',
      'heading', 'quote', 'code', '|',
      'unordered-list', 'ordered-list', '|',
      'link', 'image', '|',
      {
        name: 'emoji',
        action: () => { emojiOpen.value = !emojiOpen.value },
        className: 'emoji-toolbar-btn',
        title: 'Emoji',
        text: '😊',
      },
      '|',
      'preview', 'side-by-side', 'fullscreen', '|',
      'guide'
    ]
  })

  mde.codemirror.on('change', () => {
    emit('update:modelValue', mde.value())
    detectMention()
  })

  mde.codemirror.on('cursorActivity', detectMention)
  mde.codemirror.on('keydown', handleCmKeydown)
})

watch(() => props.modelValue, (val) => {
  if (mde && mde.value() !== val) mde.value(val)
})

onBeforeUnmount(() => {
  mde?.codemirror.off('change')
  mde?.codemirror.off('cursorActivity', detectMention)
  mde?.codemirror.off('keydown', handleCmKeydown)
  mde?.toTextArea()
  mde = null
})
</script>

<style scoped>
.editor-wrapper {
  width: 100%;
  position: relative;
}

/* Override EasyMDE toolbar emoji button — suppress the icon font, show text emoji */
:deep(.emoji-toolbar-btn) {
  font-style: normal !important;
  font-size: 16px !important;
  line-height: 1 !important;
}
:deep(.emoji-toolbar-btn::before) {
  content: none !important;
}

.editor-mention-dropdown {
  position: absolute !important;
  z-index: 500;
}

/* Anchor the emoji picker to the top-right area of the editor wrapper, near toolbar */
.editor-emoji-picker {
  position: absolute !important;
  top: 36px;   /* just below the toolbar */
  right: 0;
  bottom: auto !important;
  left: auto !important;
  z-index: 500;
}
</style>
