<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-logo">
        <img src="/logo.svg" alt="Coworker" style="height:36px;width:auto" />
      </div>
      <h1 class="auth-title">{{ $t('auth.login_title') }}</h1>
      <form @submit.prevent="handleSubmit">
        <div class="form-group">
          <label class="form-label">{{ $t('auth.email') }} / {{ $t('auth.username') }}</label>
          <input class="form-input" v-model="form.login" required autofocus />
        </div>
        <div class="form-group">
          <label class="form-label">{{ $t('auth.password') }}</label>
          <input class="form-input" type="password" v-model="form.password" required />
        </div>
        <p v-if="error" class="auth-error">{{ error }}</p>
        <button type="submit" class="btn btn-primary" style="width:100%" :disabled="loading">
          <span v-if="loading" class="spinner" style="width:16px;height:16px;border-width:2px"></span>
          {{ $t('auth.login') }}
        </button>
      </form>
      <p v-if="registrationEnabled" class="auth-link">
        {{ $t('auth.no_account') }}
        <RouterLink to="/register">{{ $t('auth.register') }}</RouterLink>
      </p>
    </div>
    <div class="auth-version">Coworker {{ appVersion }}</div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const appVersion = __APP_VERSION__
import { RouterLink, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { systemApi } from '@/api/system'

const auth = useAuthStore()
const router = useRouter()
const form = ref({ login: '', password: '' })
const error = ref('')
const loading = ref(false)
const registrationEnabled = ref(true)

onMounted(async () => {
  try {
    const { data } = await systemApi.getSettings()
    registrationEnabled.value = data.registration_enabled
  } catch {}
})

async function handleSubmit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(form.value.login, form.value.password)
    router.push('/')
  } catch (e) {
    error.value = e.response?.data?.error || 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg);
  padding: 24px;
}

.auth-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  padding: 40px;
  width: 100%;
  max-width: 400px;
  box-shadow: var(--shadow-md);
}

.auth-logo {
  font-size: 24px;
  font-weight: 800;
  color: var(--color-primary);
  text-align: center;
  margin-bottom: 8px;
}

.auth-title {
  font-size: 18px;
  font-weight: 600;
  text-align: center;
  margin-bottom: 28px;
  color: var(--color-text);
}

.auth-error { color: var(--color-danger); font-size: 13px; margin-bottom: 12px; }
.auth-link { text-align: center; margin-top: 20px; font-size: 13px; color: var(--color-text-muted); }

.auth-version {
  margin-top: 16px;
  font-size: 12px;
  color: var(--color-text-muted);
  opacity: 0.6;
}
</style>
