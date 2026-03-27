import { defineStore } from 'pinia'
import { ref } from 'vue'
import { systemApi } from '@/api/system'

export const useSystemStore = defineStore('system', () => {
  const registrationEnabled = ref(true)
  const defaults = ref({
    date_time_format: 'YYYY-MM-DD HH:mm',
    timezone: 'UTC',
    theme: 'system',
    font: 'system',
    font_size: '14'
  })

  async function fetchSettings() {
    try {
      const { data } = await systemApi.getSettings()
      registrationEnabled.value = data.registration_enabled !== false
      if (data.default_date_time_format) defaults.value.date_time_format = data.default_date_time_format
      if (data.default_timezone)         defaults.value.timezone         = data.default_timezone
      if (data.default_theme)            defaults.value.theme            = data.default_theme
      if (data.default_font)             defaults.value.font             = data.default_font
      if (data.default_font_size)        defaults.value.font_size        = data.default_font_size
    } catch {}
  }

  return { registrationEnabled, defaults, fetchSettings }
})
