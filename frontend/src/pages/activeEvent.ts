import type { MotionEvent } from './cameraUtils'

// activeEventForPlayhead devolve o evento cujo `time` é o mais próximo do playhead
// (ms absolutos) dentro da tolerância; `null` se nenhum cair dentro dela. Usado
// para a seleção do evento seguir o avanço do vídeo.
export function activeEventForPlayhead<T extends Pick<MotionEvent, 'time'>>(
  events: T[],
  playheadMs: number,
  toleranceMs: number,
): T | null {
  let best: T | null = null
  let bestDelta = Infinity
  for (const ev of events) {
    const t = Date.parse(ev.time)
    if (Number.isNaN(t)) continue
    const delta = Math.abs(t - playheadMs)
    if (delta <= toleranceMs && delta < bestDelta) {
      best = ev
      bestDelta = delta
    }
  }
  return best
}
