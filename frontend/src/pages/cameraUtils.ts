export interface Recording {
  filename: string
  start: string
  url: string
  is_recording: boolean
}

export interface MotionBBox {
  x: number
  y: number
  w: number
  h: number
}

export interface MotionEvent {
  time: string
  score: number
  frame?: string
  bbox?: MotionBBox
}

const FALLBACK_WINDOW_MS = 5 * 60 * 1000

export function mergeRecordings(
  prev: Recording[],
  fresh: Recording[],
  sortOrder: 'asc' | 'desc',
  hasMore: boolean,
): Recording[] {
  if (!hasMore) return fresh

  const freshByName = new Map(fresh.map(r => [r.filename, r]))
  const freshFilenames = new Set(fresh.map(r => r.filename))
  const freshAsc = [...fresh].sort((a, b) => a.filename.localeCompare(b.filename))
  const oldestFresh = freshAsc[0]?.filename ?? ''
  const newestFresh = freshAsc[freshAsc.length - 1]?.filename ?? ''

  const kept = prev
    .map(r => freshByName.get(r.filename) ?? r)
    .filter(r => {
      if (freshFilenames.has(r.filename)) return true
      return sortOrder === 'desc' ? r.filename < oldestFresh : r.filename > newestFresh
    })

  const existingNames = new Set(prev.map(r => r.filename))
  const newOnes = fresh.filter(r => !existingNames.has(r.filename))

  return [...kept, ...newOnes].sort((a, b) =>
    sortOrder === 'desc'
      ? b.filename.localeCompare(a.filename)
      : a.filename.localeCompare(b.filename)
  )
}

export function eventsWithinRecordings(events: MotionEvent[], recordings: Recording[]): MotionEvent[] {
  if (recordings.length === 0) return []
  const asc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
  return events.filter(ev => {
    const evTime = new Date(ev.time).getTime()
    for (let i = 0; i < asc.length; i++) {
      const recStart = new Date(asc[i].start).getTime()
      const nextStart = i + 1 < asc.length
        ? new Date(asc[i + 1].start).getTime()
        : recStart + FALLBACK_WINDOW_MS
      if (evTime >= recStart && evTime < nextStart) return true
    }
    return false
  })
}
