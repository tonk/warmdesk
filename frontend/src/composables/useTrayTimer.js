import { watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useTimerStore } from '@/stores/timer'
import { useUIStore } from '@/stores/ui'
import { useChatNotify } from '@/composables/useChatNotify'
import { timerLabel, timerBookedMessage, timerStartedAt } from '@/utils/timerFormat'

const isTauri = !!window.__TAURI_INTERNALS__

// The tray menu's timer section for the current state, translated — or null
// when the user can't use the timer (the menu then shows only WarmDesk/Quit).
export function trayTimerMenu(t, { canUse, available, running, timer, lastPick }) {
  if (!canUse || available !== true) return null
  if (running && timer) {
    return {
      status: t('timer.tray_running', {
        label: timerLabel(timer.project?.name, timer.customer?.name, timer.description),
        time: timerStartedAt(timer),
      }),
      actions: [
        { id: 'stop', label: t('timer.stop') },
        { id: 'switch', label: t('timer.switch') },
      ],
    }
  }
  const actions = []
  if (lastPick?.label) actions.push({ id: 'start-last', label: t('timer.tray_start_last', { label: lastPick.label }) })
  actions.push({ id: 'start', label: t('timer.tray_start') })
  return { status: t('timer.tray_idle'), actions }
}

// Puts the timer in the desktop app's tray menu (src-tauri set_tray_timer)
// and carries out what's clicked there (the "tray-timer" event). A no-op in
// the browser.
export function useTrayTimer() {
  if (!isTauri) return
  const { t, locale } = useI18n()
  const auth = useAuthStore()
  const timerStore = useTimerStore()
  const ui = useUIStore()
  const { desktopNotify } = useChatNotify()

  async function push() {
    const timer = trayTimerMenu(t, {
      canUse: !!auth.user && (auth.isAdmin || !!auth.user.time_tracking_enabled),
      available: timerStore.available,
      running: timerStore.running,
      timer: timerStore.timer,
      lastPick: timerStore.lastPick,
    })
    try {
      const { invoke } = await import('@tauri-apps/api/core')
      await invoke('set_tray_timer', { timer })
    } catch {
      // No tray (e.g. an older desktop shell)
    }
  }

  // Feedback both in the window and, when it's hidden in the tray, as a
  // desktop notification.
  function tell(message, kind = 'success') {
    ui[kind](message)
    desktopNotify('WarmDesk', message)
  }

  async function onAction(action) {
    try {
      if (action === 'stop') {
        const entries = await timerStore.stop()
        if (entries.length) tell(timerBookedMessage(t, entries))
      } else if (action === 'start-last' && timerStore.lastPick) {
        const { projectId, customerId } = timerStore.lastPick
        const data = await timerStore.start({ projectId, customerId, description: '' })
        if (data?.stopped_entries?.length) tell(timerBookedMessage(t, data.stopped_entries))
        const tm = data?.timer
        tell(t('timer.started', { label: timerLabel(tm?.project?.name, tm?.customer?.name, tm?.description) }))
      } else if (action === 'start' || action === 'switch') {
        // The window was brought to the front by the tray; open the panel.
        timerStore.requestPanel(action === 'switch')
      }
    } catch (e) {
      tell(e?.response?.data?.error || t('timer.failed'), 'error')
      timerStore.refresh()
    }
  }

  watch(
    [
      () => auth.user?.id,
      () => auth.user?.time_tracking_enabled,
      () => timerStore.available,
      () => timerStore.running,
      () => timerStore.timer,
      () => timerStore.lastPick,
      locale,
    ],
    push,
    { immediate: true, deep: true },
  )

  let unlisten = null
  import('@tauri-apps/api/event')
    .then(({ listen }) => listen('tray-timer', (e) => onAction(e.payload)))
    .then((fn) => { unlisten = fn })
    .catch(() => {})
  onUnmounted(() => unlisten?.())
}
