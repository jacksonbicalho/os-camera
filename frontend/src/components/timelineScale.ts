import type { Recording } from '../pages/cameraUtils'

// Escala da timeline horizontal (redesign do Escopo B). Helpers puros para
// mapear tempo ↔ posição dentro de uma janela [startMs, endMs].

export type TimelineRange = '1h' | '6h' | '24h'

export interface TimelineWindow {
  startMs: number
  endMs: number
}

const RANGE_MS: Record<TimelineRange, number> = {
  '1h': 3600_000,
  '6h': 6 * 3600_000,
  '24h': 24 * 3600_000,
}

export function timelineRangeMs(range: TimelineRange): number {
  return RANGE_MS[range]
}

// Janela que termina no anchor e recua a duração do range.
export function timelineWindow(endMs: number, range: TimelineRange): TimelineWindow {
  return { startMs: endMs - RANGE_MS[range], endMs }
}

// Fração 0..1 da posição de um timestamp dentro da janela (clampada).
export function timePosFraction(tsMs: number, win: TimelineWindow): number {
  const span = win.endMs - win.startMs
  if (span <= 0) return 0
  const f = (tsMs - win.startMs) / span
  return f < 0 ? 0 : f > 1 ? 1 : f
}

export function isInWindow(tsMs: number, win: TimelineWindow): boolean {
  return tsMs >= win.startMs && tsMs <= win.endMs
}

// Inverso de timePosFraction: fração 0..1 → timestamp (ms) na janela.
export function posToTime(fraction: number, win: TimelineWindow): number {
  const f = fraction < 0 ? 0 : fraction > 1 ? 1 : fraction
  return win.startMs + f * (win.endMs - win.startMs)
}

// Gravação (não-ativa) cujo intervalo [start, start+chunk) cobre o ms, e o
// offset em segundos dentro dela. `null` numa lacuna (sem gravação no instante).
export function recordingAtMs(
  recordings: Recording[],
  ms: number,
  chunkMs: number,
): { rec: Recording; offsetSeconds: number } | null {
  for (const rec of recordings) {
    if (rec.is_recording) continue
    const startMs = Date.parse(rec.start)
    if (Number.isNaN(startMs)) continue
    if (ms >= startMs && ms < startMs + chunkMs) {
      return { rec, offsetSeconds: Math.max(0, (ms - startMs) / 1000) }
    }
  }
  return null
}

// `count` timestamps uniformemente espaçados, inclusive os extremos.
export function timelineTicks(win: TimelineWindow, count: number): number[] {
  if (count < 2) return [win.startMs]
  const span = win.endMs - win.startMs
  const step = span / (count - 1)
  return Array.from({ length: count }, (_, i) => win.startMs + step * i)
}
