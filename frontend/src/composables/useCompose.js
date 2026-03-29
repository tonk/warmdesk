/**
 * useCompose — emoji insertion and @mention autocomplete for plain <textarea> elements.
 *
 * Usage:
 *   const { mentionUsers, mentionIndex, insertText, onTextareaInput, onTextareaKeydown, pickMention }
 *     = useCompose({ textareaEl, getValue, setValue, users })
 *
 *   - textareaEl : ref to the <textarea> DOM element
 *   - getValue   : () => string  — read the current model value
 *   - setValue   : (s) => void   — write a new model value
 *   - users      : ref/computed of user objects { id, username, display_name }
 */
import { ref, computed, nextTick } from 'vue'

export function useCompose({ textareaEl, getValue, setValue, users }) {
  const mentionQuery = ref(null)   // null = no active mention; string = partial after @
  const mentionStart = ref(0)      // character offset of the leading @
  const mentionIndex = ref(0)      // keyboard-highlighted row

  const mentionUsers = computed(() => {
    if (mentionQuery.value === null) return []
    const q = (mentionQuery.value || '').toLowerCase()
    return (users.value || []).filter(u =>
      u.username.toLowerCase().startsWith(q) ||
      (u.display_name || '').toLowerCase().startsWith(q)
    ).slice(0, 8)
  })

  // Insert arbitrary text at the textarea cursor (emoji, completed mention, etc.)
  function insertText(text) {
    const el = textareaEl.value
    if (!el) return
    const start = el.selectionStart
    const end   = el.selectionEnd
    const val   = getValue()
    setValue(val.slice(0, start) + text + val.slice(end))
    nextTick(() => {
      el.selectionStart = el.selectionEnd = start + [...text].length
      el.focus()
    })
  }

  // Call this from the textarea's @input handler
  function onTextareaInput() {
    const el = textareaEl.value
    if (!el) return
    const pos    = el.selectionStart
    const before = getValue().slice(0, pos)
    const m      = before.match(/@(\w*)$/)
    if (m) {
      mentionQuery.value = m[1]
      mentionStart.value = pos - m[0].length
      mentionIndex.value = 0
    } else {
      mentionQuery.value = null
    }
  }

  // Call this from the textarea's @keydown handler (before default handling)
  // Returns true if the key was consumed (caller should suppress default + stop).
  function onTextareaKeydown(e) {
    if (mentionQuery.value === null || !mentionUsers.value.length) return false
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      mentionIndex.value = (mentionIndex.value + 1) % mentionUsers.value.length
      return true
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      mentionIndex.value = (mentionIndex.value - 1 + mentionUsers.value.length) % mentionUsers.value.length
      return true
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault()
      pickMention(mentionUsers.value[mentionIndex.value])
      return true
    }
    if (e.key === 'Escape') {
      mentionQuery.value = null
      return true
    }
    return false
  }

  function pickMention(user) {
    const el  = textareaEl.value
    const pos = el?.selectionStart ?? (getValue().length)
    const val = getValue()
    const mention = '@' + user.username + ' '
    setValue(val.slice(0, mentionStart.value) + mention + val.slice(pos))
    mentionQuery.value = null
    nextTick(() => {
      if (el) {
        el.selectionStart = el.selectionEnd = mentionStart.value + mention.length
        el.focus()
      }
    })
  }

  return { mentionUsers, mentionIndex, insertText, onTextareaInput, onTextareaKeydown, pickMention }
}
