// Pure formatting helpers for the time-tracking overview (all inputs are minutes).

/** 90 -> "1:30"; negative values keep their sign ("-0:45"). */
export function fmtHM(minutes) {
  const m = Math.round(Number(minutes) || 0)
  const sign = m < 0 ? '-' : ''
  const abs = Math.abs(m)
  return `${sign}${Math.floor(abs / 60)}:${String(abs % 60).padStart(2, '0')}`
}

/** Always signed: 90 -> "+1:30", -45 -> "-0:45", 0 -> "0:00". */
export function fmtSignedHM(minutes) {
  const m = Math.round(Number(minutes) || 0)
  if (m === 0) return '0:00'
  return m > 0 ? `+${fmtHM(m)}` : fmtHM(m)
}

/** Decimal hours rounded to `digits` places: 90 -> 1.5. */
export function minutesToHours(minutes, digits = 2) {
  const f = 10 ** digits
  return Math.round(((Number(minutes) || 0) / 60) * f) / f
}

/** Hours (from an input) -> minutes, tolerating a decimal comma. */
export function hoursToMinutes(hours) {
  const n = typeof hours === 'string' ? parseFloat(hours.replace(',', '.')) : Number(hours)
  return Number.isFinite(n) ? Math.round(n * 60) : 0
}

/** Class name for a difference: 'diff-pos' | 'diff-neg' | 'diff-zero'. */
export function diffClass(minutes) {
  const m = Number(minutes) || 0
  return m > 0 ? 'diff-pos' : m < 0 ? 'diff-neg' : 'diff-zero'
}

/** Kilometres with at most one decimal. */
export function fmtKm(km) {
  return String(Math.round((Number(km) || 0) * 10) / 10)
}

/** Localised long month name for a 1-based month number. */
// weekdayName returns the full weekday of a YYYY-MM-DD date in the locale,
// computed in UTC so the date never shifts with the browser's time zone.
export function weekdayName(locale, date) {
  return new Intl.DateTimeFormat(locale || 'en', { weekday: 'long', timeZone: 'UTC' })
    .format(new Date(`${date}T12:00:00Z`))
}

export function monthName(locale, month) {
  return new Intl.DateTimeFormat(locale || 'en', { month: 'long' }).format(new Date(2024, month - 1, 1))
}

/** Minutes -> days of `perDayMinutes` each, at most two decimals: (720, 480) -> 1.5. */
export function minutesToDays(minutes, perDayMinutes) {
  const per = Number(perDayMinutes) || 0
  if (per <= 0) return 0
  return Math.round(((Number(minutes) || 0) / per) * 100) / 100
}
