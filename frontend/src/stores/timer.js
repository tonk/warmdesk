import { defineStore } from 'pinia'
import { ref } from 'vue'
import { timerApi } from '@/api/timer'

const LAST_KEY = 'warmdesk_timer_last'

function loadLastPick() {
  try {
    const v = JSON.parse(localStorage.getItem(LAST_KEY) || 'null')
    return v && (v.projectId || v.customerId) ? v : null
  } catch {
    return null
  }
}

// The user's time-tracking timer (see handlers/timer.go). It lives on the
// server, so the CLI or another tab can change it too; App.vue forwards the
// user WebSocket's "timer.changed" message to onChanged().
export const useTimerStore = defineStore('timer', () => {
  const running = ref(false)
  const timer = ref(null)
  // Whether the server has the timer at all: null until the first answer,
  // false when it doesn't know /timer (404). The desktop app bundles its own
  // frontend, so it can be newer than the server it connects to (the timer
  // arrived in v0.33.0).
  const available = ref(null)
  // Bumped whenever time got booked, by any client — the Log Time tab
  // watches it to reload the week.
  const bookedVersion = ref(0)
  // This browser's last pick ({ projectId, customerId, label }) — a
  // convenience for the panel and the tray's one-click start, nothing more.
  const lastPick = ref(loadLastPick())
  // Bumped to ask the top-bar button to open its panel (from the tray menu);
  // switching opens it in "switch to another task" mode.
  const panelRequest = ref({ n: 0, switching: false })

  function apply(data) {
    running.value = !!data?.running
    timer.value = data?.running ? data.timer : null
  }

  async function refresh() {
    try {
      const { data } = await timerApi.get()
      apply(data)
      available.value = true
    } catch (e) {
      apply(null)
      // Any other failure (network, 5xx) keeps the button: clicking it then
      // reports the error, instead of the button vanishing until a reload.
      available.value = e?.response?.status === 404 ? false : true
    }
  }

  function browserTimeZone() {
    try {
      return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
    } catch {
      return ''
    }
  }

  // Starts a timer; a running one is booked first (returned as stopped_entries).
  async function start({ projectId, customerId, description }) {
    const body = { description: description || '' }
    if (projectId) body.project_id = projectId
    if (customerId) body.customer_id = customerId
    const zone = browserTimeZone()
    if (zone) body.time_zone = zone
    const { data } = await timerApi.start(body)
    apply(data)
    if (data?.stopped_entries?.length) bookedVersion.value++
    return data
  }

  // Stops the timer and returns the booked time entries.
  async function stop() {
    const { data } = await timerApi.stop()
    apply(null)
    bookedVersion.value++
    return data || []
  }

  async function cancel() {
    await timerApi.cancel()
    apply(null)
  }

  function rememberPick(pick) {
    lastPick.value = pick
    try { localStorage.setItem(LAST_KEY, JSON.stringify(pick)) } catch {}
  }

  function requestPanel(switching = false) {
    panelRequest.value = { n: panelRequest.value.n + 1, switching }
  }

  function onChanged(payload) {
    if (payload?.booked > 0) bookedVersion.value++
    refresh()
  }

  function reset() {
    apply(null)
  }

  return {
    running, timer, available, bookedVersion, lastPick, panelRequest,
    refresh, start, stop, cancel, rememberPick, requestPanel, onChanged, reset,
  }
})
