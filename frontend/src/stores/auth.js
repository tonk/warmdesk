import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'
import { setLocale } from '@/i18n'
import client from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const user = ref(JSON.parse(sessionStorage.getItem('user') || 'null'))
  const accessToken = ref(sessionStorage.getItem('access_token') || null)
  const pendingMFAToken = ref(null)
  const mfaSetupRequired = ref(false)

  // Seed the axios default header from the stored token so requests never miss
  // the token even if sessionStorage is cleared after initialization.
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
    if (data.mfa_required) {
      pendingMFAToken.value = data.mfa_token
      return { mfa_required: true }
    }
    setTokens(data.access_token, data.refresh_token)
    mfaSetupRequired.value = !!data.mfa_setup_required
    await fetchMe()
    return {}
  }

  async function verifyMFA(code) {
    const { data } = await authApi.verifyMFA(pendingMFAToken.value, code)
    pendingMFAToken.value = null
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
    sessionStorage.setItem('user', JSON.stringify(data))
    if (data.locale) setLocale(data.locale)
  }

  function setTokens(access, refresh) {
    accessToken.value = access
    sessionStorage.setItem('access_token', access)
    sessionStorage.setItem('refresh_token', refresh)
    client.defaults.headers.common.Authorization = `Bearer ${access}`
  }

  function logout() {
    stopIdleTimer()
    user.value = null
    accessToken.value = null
    pendingMFAToken.value = null
    mfaSetupRequired.value = false
    sessionStorage.removeItem('access_token')
    sessionStorage.removeItem('refresh_token')
    sessionStorage.removeItem('user')
    delete client.defaults.headers.common.Authorization
  }

  async function updateProfile(data) {
    const { data: updated } = await authApi.updateMe(data)
    user.value = updated
    sessionStorage.setItem('user', JSON.stringify(updated))
    if (data.locale) setLocale(data.locale)
  }

  return { user, accessToken, isLoggedIn, isAdmin, canViewReports, pendingMFAToken, mfaSetupRequired, login, verifyMFA, register, logout, fetchMe, updateProfile, startIdleTimer, resetIdleTimer, stopIdleTimer }
})
