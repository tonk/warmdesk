import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUIStore = defineStore('ui', () => {
  const toasts = ref([])
  let nextId = 0

  function toast(message, type = 'info', duration = 3000) {
    const id = ++nextId
    toasts.value.push({ id, message, type })
    setTimeout(() => {
      toasts.value = toasts.value.filter(t => t.id !== id)
    }, duration)
  }

  function success(message) { toast(message, 'success') }
  function error(message) { toast(message, 'error', 5000) }
  function info(message) { toast(message, 'info') }

  return { toasts, toast, success, error, info }
})
