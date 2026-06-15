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

// Vídeo mínimo que o passo Ctrl+Shift+seta precisa manipular.
export interface SteppableVideo {
  currentTime: number
  play(): unknown
  pause(): void
}

// Aplica um passo de 1s dentro do mesmo chunk: posiciona no tempo alvo e mantém
// pausado. O atalho Ctrl+Shift+seta navega segundo a segundo com o vídeo parado —
// nunca dá play.
export function applySameChunkStep(v: SteppableVideo, time: number): void {
  v.currentTime = time
  v.pause()
}

// Novo tempo ao pular um frame (←/→) dentro da gravação atual. `dir` é +1
// (avança) ou -1 (retrocede). Clampa em [0, duration] — frame step não cruza
// chunk; nas bordas, satura.
export function frameStepTime(
  currentTime: number,
  duration: number,
  frameDuration: number,
  dir: 1 | -1,
): number {
  const next = currentTime + dir * frameDuration
  if (next < 0) return 0
  if (duration > 0 && next > duration) return duration
  return next
}

// Vídeo mínimo para o passo frame a frame com serialização de seeks.
export interface FrameSteppable {
  currentTime: number
  duration: number
  seeking: boolean
  pause(): void
}

// Resultado de um passo de frame:
// - `busy`: havia um seek em andamento → ignorado (serialização).
// - `applied`: pulou um frame dentro do chunk atual.
// - `cross-forward` / `cross-back`: chegou ao fim/início do chunk → o chamador
//   deve carregar o chunk seguinte/anterior. `overflow` é o quanto passou da
//   borda (em segundos), pra posicionar no vizinho.
export type FrameStepResult =
  | { kind: 'busy' }
  | { kind: 'applied' }
  | { kind: 'cross-forward'; overflow: number }
  | { kind: 'cross-back'; overflow: number }

// Aplica um passo de frame (←/→) serializando os seeks: se um seek ainda está em
// andamento (`seeking`), ignora o passo — evita a fila de seeks que, ao segurar a
// tecla, faz o vídeo travar e dar um salto ao soltar. Dentro do chunk, pausa e
// avança/retrocede um frame. Nas bordas, sinaliza `cross-*` para o chamador trocar
// de gravação em vez de saturar.
export function applyFrameStep(
  v: FrameSteppable,
  recDuration: number,
  frameDuration: number,
  dir: 1 | -1,
): FrameStepResult {
  if (v.seeking) return { kind: 'busy' }
  const dur = v.duration || recDuration
  const next = v.currentTime + dir * frameDuration
  if (next < 0) return { kind: 'cross-back', overflow: -next }
  if (dur > 0 && next > dur) return { kind: 'cross-forward', overflow: next - dur }
  v.pause()
  v.currentTime = next
  return { kind: 'applied' }
}

// Calcula a posição de seek e se deve reproduzir ao carregar a metadata de uma
// gravação. `stepPaused` indica que o load veio de um passo Ctrl+Shift+seta que
// cruzou a fronteira do chunk — nesse caso mantém o vídeo parado.
export function loadedMetadataSeek(
  duration: number,
  pendingFromEnd: number | null,
  pending: number | null,
  stepPaused: boolean,
): { seekTo: number | null; shouldPlay: boolean } {
  let seekTo: number | null = null
  if (pendingFromEnd !== null) seekTo = Math.max(0, duration - pendingFromEnd)
  else if (pending !== null) seekTo = pending
  return { seekTo, shouldPlay: !stepPaused }
}
