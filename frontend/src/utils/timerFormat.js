// Shared wording for the time-tracking timer (top-bar button and tray menu).

// "Project (Customer) — description", leaving out what's missing.
export function timerLabel(project, customer, description) {
  let s = project && customer ? `${project} (${customer})` : (project || customer || '')
  if (description) s += ` — ${description}`
  return s
}

// "45m" / "1h 05m", translated.
export function timerDuration(t, minutes) {
  return minutes < 60
    ? t('timer.minutes', { m: minutes })
    : t('timer.hours_minutes', { h: Math.floor(minutes / 60), m: String(minutes % 60).padStart(2, '0') })
}

// "Booked 1h 30m on Travel (Globex)" for the entries a stop returned.
export function timerBookedMessage(t, entries) {
  const total = entries.reduce((sum, e) => sum + (e.minutes || 0), 0)
  const e = entries[0]
  return t('timer.booked', {
    duration: timerDuration(t, total),
    label: timerLabel(e?.project?.name, e?.customer?.name, e?.description),
  })
}

// The local wall-clock time a timer started at, e.g. "09:03".
export function timerStartedAt(timer) {
  const started = timer?.started_at
  return started ? new Date(started).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ''
}
