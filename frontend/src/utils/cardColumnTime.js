// How long a card has spent in each column, derived from its column-move
// history (GET /projects/:slug/cards/:id/history) — wall-clock time, not
// worked time: that is what time entries and the timer are for.

function isColumnMove(h) {
  return (h.event_type === 'column_move' || !h.event_type) && h.from_column_id && h.to_column_id
}

// columnDurations returns [{ column_id, name, minutes, current }], in board
// order when `columns` (the board's columns) is given, else in the order the
// card visited them. The card starts in the first move's source column (or
// its current column when it never moved) at card.created_at; the column it
// is in now runs until `now`.
export function columnDurations(history, card, columns = [], now = Date.now()) {
  if (!card?.created_at) return []
  const moves = (history || [])
    .filter(isColumnMove)
    .slice()
    .sort((a, b) => new Date(a.created_at) - new Date(b.created_at))

  const names = new Map()
  for (const c of columns) names.set(c.id, c.name)
  for (const m of moves) {
    if (m.from_column?.name && !names.has(m.from_column_id)) names.set(m.from_column_id, m.from_column.name)
    if (m.to_column?.name && !names.has(m.to_column_id)) names.set(m.to_column_id, m.to_column.name)
  }

  const totals = new Map()
  const add = (colId, ms) => totals.set(colId, (totals.get(colId) || 0) + Math.max(0, ms))

  let current = moves.length ? moves[0].from_column_id : card.column_id
  let cursor = new Date(card.created_at).getTime()
  for (const m of moves) {
    const at = new Date(m.created_at).getTime()
    add(current, at - cursor)
    current = m.to_column_id
    cursor = Math.max(cursor, at)
  }
  add(current, now - cursor)

  const order = columns.length ? columns.map(c => c.id) : [...totals.keys()]
  const ids = [...order.filter(id => totals.has(id)), ...[...totals.keys()].filter(id => !order.includes(id))]
  return ids.map(id => ({
    column_id: id,
    name: names.get(id) || '?',
    minutes: Math.floor(totals.get(id) / 60000),
    current: id === current,
  }))
}

// "3d 4h", "5h 20m", "12m" — for spans that can run into days.
export function formatSpan(minutes) {
  const m = Math.max(0, Math.floor(minutes || 0))
  const d = Math.floor(m / 1440)
  const h = Math.floor((m % 1440) / 60)
  const min = m % 60
  if (d > 0) return h > 0 ? `${d}d ${h}h` : `${d}d`
  if (h > 0) return min > 0 ? `${h}h ${min}m` : `${h}h`
  return `${min}m`
}
