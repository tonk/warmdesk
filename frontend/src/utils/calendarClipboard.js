import { parseWallClock, fmtWallClock, wallClockSpanMinutes, addDaysISO } from './shiftTimeEntries'

const MINUTES_PER_DAY = 24 * 60

function dayDiff(fromISO, toISO) {
  const a = Date.UTC(...fromISO.split('-').map((n, i) => Number(n) - (i === 1 ? 1 : 0)))
  const b = Date.UTC(...toISO.split('-').map((n, i) => Number(n) - (i === 1 ? 1 : 0)))
  return Math.round((b - a) / 86400000)
}

function isScheduled(e) {
  return parseWallClock(e.start_time) >= 0 && parseWallClock(e.end_time) >= 0
}

/**
 * Earliest copied entry by date + start time — the reference point a paste is
 * positioned against. Scheduled entries win over unscheduled ones on the same day.
 */
export function clipboardAnchor(entries) {
  let best = null
  for (const e of entries) {
    if (!e.date) continue
    const date = e.date.slice(0, 10)
    const start = isScheduled(e) ? parseWallClock(e.start_time) : MINUTES_PER_DAY
    if (!best || date < best.date || (date === best.date && start < best.start)) best = { date, start }
  }
  return best
}

/**
 * Turns copied time entries into create payloads for a paste at targetDate.
 *
 * With targetStartTime set (pasting onto a clicked slot) the whole group shifts so
 * its earliest entry starts there, keeping every other entry's distance to it — day
 * and time alike. Without it (paste at original times) only the date shifts, so a
 * copied day lands on the target day with the same times. Same-day entries that
 * would run past midnight are clamped to 23:59, as a single-entry paste always did;
 * overnight entries keep their duration and wrap. Unscheduled entries (no start/end)
 * only move by whole days.
 */
export function buildPastePayloads(entries, targetDate, targetStartTime = null) {
  const anchor = clipboardAnchor(entries)
  if (!anchor) return []
  const dayShift = dayDiff(anchor.date, targetDate)
  const targetStart = targetStartTime == null ? -1 : parseWallClock(targetStartTime)
  const timeShift = targetStart >= 0 && anchor.start < MINUTES_PER_DAY
    ? dayShift * MINUTES_PER_DAY + targetStart - anchor.start
    : null

  const payloads = []
  for (const src of entries) {
    if (!src.date) continue
    const srcDate = src.date.slice(0, 10)
    const base = {
      id: undefined,
      customer_id: src.customer_id ?? null,
      project_id: src.project_id ?? null,
      contract_id: src.contract_id ?? null,
      description: src.description || '',
      is_holiday: src.is_holiday || false,
      distance: src.distance ?? null,
      location_id: src.location_id ?? null,
    }

    const startM = parseWallClock(src.start_time)
    const endM = parseWallClock(src.end_time)
    // Unscheduled entries and the whole-day convention (start === end) have no
    // meaningful time to shift — move them by whole days only.
    if (!isScheduled(src) || startM === endM || timeShift == null) {
      payloads.push({
        ...base,
        date: addDaysISO(srcDate, dayShift),
        minutes: src.minutes,
        start_time: src.start_time || '',
        end_time: src.end_time || '',
      })
      continue
    }

    const absStart = dayDiff(anchor.date, srcDate) * MINUTES_PER_DAY + startM + timeShift
    const newDate = addDaysISO(anchor.date, Math.floor(absStart / MINUTES_PER_DAY))
    const newStart = ((absStart % MINUTES_PER_DAY) + MINUTES_PER_DAY) % MINUTES_PER_DAY
    const duration = wallClockSpanMinutes(startM, endM)
    const newEnd = endM > startM
      ? Math.min(MINUTES_PER_DAY - 1, newStart + duration)
      : (newStart + duration) % MINUTES_PER_DAY
    payloads.push({
      ...base,
      date: newDate,
      minutes: wallClockSpanMinutes(newStart, newEnd) ?? duration,
      start_time: fmtWallClock(newStart),
      end_time: fmtWallClock(newEnd),
    })
  }
  return payloads
}
