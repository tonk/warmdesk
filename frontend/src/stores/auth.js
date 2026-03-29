import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'
import { setLocale } from '@/i18n'
import client from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const user = ref(JSON.parse(localStorage.getItem('user') || 'null'))
  const accessToken = ref(localStorage.getItem('access_token') || null)

  // Seed the axios default header from the stored token so requests never miss
  // the token even if localStorage is cleared after initialization.
  if (accessToken.value) {
    client.defaults.headers.common.Authorization = `Bearer ${accessToken.value}`
  }

  const isLoggedIn = computed(() => !!accessToken.value && !!user.value)
  const isAdmin = computed(() => user.value?.global_role === 'admin')
  const canViewReports = computed(() => isAdmin.value || !!user.value?.can_view_reports)

  // ── Idle session timeout ─────────────────────────────────────────────────
  let idleTimer = null

  function startIdleTimer(timeoutMinutes) {
    stopIdleTimer()
    if (!timeoutMinutes || timeoutMinutes <= 0) return
    idleTimer = setTimeout(() => {
      logout()
      window.location.href = '/login'
    }, timeoutMinutes * 60 * 1000)
  }

  function resetIdleTimer(timeoutMinutes) {
    if (!timeoutMinutes || timeoutMinutes <= 0) return
    startIdleTimer(timeoutMinutes)
  }

  function stopIdleTimer() {
    if (idleTimer) {
      clearTimeout(idleTimer)
      idleTimer = null
    }
  }

  async function login(login, password) {
    const { data } = await authApi.login({ login, password })
    setTokens(data.access_token, data.refresh_token)
    await fetchMe()
  }

  async function register(payload) {
    const { data } = await authApi.register(payload)
    setTokens(data.access_token, data.refresh_token)
    await fetchMe()
  }

  async function fetchMe() {
    const { data } = await authApi.me()
    user.value = data
    localStorage.setItem('user', JSON.stringify(data))
    if (data.locale) setLocale(data.locale)
  }

  function setTokens(access, refresh) {
    accessToken.value = access
    localStorage.setItem('access_token', access)
    localStorage.setItem('refresh_token', refresh)
    client.defaults.headers.common.Authorization = `Bearer ${access}`
  }

  function logout() {
    stopIdleTimer()
    user.value = null
    accessToken.value = null
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('user')
    delete client.defaults.headers.common.Authorization
  }

  async function updateProfile(data) {
    const { data: updated } = await authApi.updateMe(data)
    user.value = updated
    localStorage.setItem('user', JSON.stringify(updated))
    if (data.locale) setLocale(data.locale)
  }

  return { user, accessToken, isLoggedIn, isAdmin, canViewReports, login, register, logout, fetchMe, updateProfile, startIdleTimer, resetIdleTimer, stopIdleTimer }
})
