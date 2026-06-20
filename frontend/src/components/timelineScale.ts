// Escala da timeline horizontal (redesign do Escopo B). Helpers puros para
// mapear tempo → posição dentro de uma janela [startMs, endMs]. O ponteiro e o
// seek por clique ficam para a história #6.

export type TimelineRange = '1h' | '6h' | '24h' | '7d'

export interface TimelineWindow {
  startMs: number
  endMs: number
}

const RANGE_MS: Record<TimelineRange, number> = {
  '1h': 3600_000,
  '6h': 6 * 3600_000,
  '24h': 24 * 3600_000,
  '7d': 7 * 24 * 3600_000,
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

// `count` timestamps uniformemente espaçados, inclusive os extremos.
export function timelineTicks(win: TimelineWindow, count: number): number[] {
  if (count < 2) return [win.startMs]
  const span = win.endMs - win.startMs
  const step = span / (count - 1)
  return Array.from({ length: count }, (_, i) => win.startMs + step * i)
}
