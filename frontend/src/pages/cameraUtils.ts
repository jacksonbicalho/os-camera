export interface Recording {
  filename: string
  start: string
  url: string
  is_recording: boolean
  has_motion: boolean
}

export interface MotionBBox {
  x: number
  y: number
  w: number
  h: number
}

export interface MotionEvent {
  id?: number
  time: string
  score: number
  frame?: string
  bbox?: MotionBBox
  label?: string
  color?: string
}

export function mergeRecordings(
  prev: Recording[],
  fresh: Recording[],
  sortOrder: 'asc' | 'desc',
  hasMore: boolean,
): Recording[] {
  const freshByName = new Map(fresh.map(r => [r.filename, r]))
  const freshFilenames = new Set(fresh.map(r => r.filename))
  const freshAsc = [...fresh].sort((a, b) => a.filename.localeCompare(b.filename))
  const oldestFresh = freshAsc[0]?.filename ?? ''
  const newestFresh = freshAsc[freshAsc.length - 1]?.filename ?? ''

  const kept = prev
    .map(r => freshByName.get(r.filename) ?? r)
    .filter(r => {
      if (freshFilenames.has(r.filename)) return true
      // !hasMore: fresh é a lista completa — tudo fora dela foi deletado
      if (!hasMore) return false
      // hasMore: preserva gravações mais antigas fora da janela retornada
      return sortOrder === 'desc' ? r.filename < oldestFresh : r.filename > newestFresh
    })

  const existingNames = new Set(prev.map(r => r.filename))
  const newOnes = fresh.filter(r => !existingNames.has(r.filename))

  const result = [...kept, ...newOnes].sort((a, b) =>
    sortOrder === 'desc'
      ? b.filename.localeCompare(a.filename)
      : a.filename.localeCompare(b.filename)
  )

  // Return same reference when nothing changed — React bails out and skips re-render
  if (
    result.length === prev.length &&
    result.every((r, i) => r.filename === prev[i].filename && r.is_recording === prev[i].is_recording)
  ) {
    return prev
  }
  return result
}

// Parses a Go-formatted duration string ("5m", "30s", "2h") to milliseconds.
export function parseDurationToMs(s: string | undefined, defaultMs = 30_000): number {
  if (!s) return defaultMs
  const m = s.match(/^(\d+)(h|m|s)$/)
  if (!m) return defaultMs
  const n = parseInt(m[1], 10)
  if (m[2] === 'h') return n * 3_600_000
  if (m[2] === 'm') return n * 60_000
  return n * 1_000
}
