export interface Recording {
  id: number
  filename: string
  start: string
  url: string
  is_recording: boolean
  has_motion: boolean
  detections?: Array<{ label: string; confidence: number; frame_count: number; custom_model?: boolean }>
}

export interface MotionBBox {
  x: number
  y: number
  w: number
  h: number
}

export interface Annotation {
  id: number
  event_id: number
  label: string
  bbox_x: number
  bbox_y: number
  bbox_w: number
  bbox_h: number
  created_at: string
}

export interface MotionEvent {
  id: number
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

export type SecondStep =
  | { kind: 'same'; time: number }
  | { kind: 'load'; rec: Recording; offsetSeconds: number; fromEnd?: boolean }

// Alvo de um passo de 1 segundo (atalho Ctrl+Shift+seta) no ponteiro da régua.
// `dir` é +1 (avança) ou -1 (retrocede). Dentro do chunk atual → {same}; ao cruzar a
// fronteira → {load} da gravação adjacente reproduzível (pulando vãos e o chunk em
// gravação). `fromEnd` indica que o offset é contado a partir do fim (passo pra trás).
// Retorna null quando não há gravação naquela direção.
export function secondStepTarget(
  recsAsc: Recording[],
  currentFilename: string,
  currentTime: number,
  currentDuration: number,
  dir: 1 | -1,
): SecondStep | null {
  const curIdx = recsAsc.findIndex(r => r.filename === currentFilename)
  if (curIdx === -1) return null
  const newTime = currentTime + dir
  if (newTime >= 0 && (currentDuration <= 0 || newTime <= currentDuration)) {
    return { kind: 'same', time: newTime }
  }
  if (dir > 0) {
    for (let i = curIdx + 1; i < recsAsc.length; i++) {
      if (!recsAsc[i].is_recording) {
        return { kind: 'load', rec: recsAsc[i], offsetSeconds: Math.max(0, newTime - currentDuration) }
      }
    }
    return null
  }
  for (let i = curIdx - 1; i >= 0; i--) {
    if (!recsAsc[i].is_recording) {
      return { kind: 'load', rec: recsAsc[i], offsetSeconds: -newTime, fromEnd: true }
    }
  }
  return null
}
