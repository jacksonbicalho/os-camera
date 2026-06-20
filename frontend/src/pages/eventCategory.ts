import type { MotionEvent } from './cameraUtils'

// Categoria de um evento, derivada do label (redesign do Escopo B — chips do
// painel de eventos). `estados` (transições de classificador de estado) não vive
// em motion_events, então nunca é devolvida por eventCategory: o chip Estados
// filtra vazio até o feed unificar esses eventos (história futura).
export type EventCategory = 'movimento' | 'pessoa' | 'ia' | 'estados'
export type EventFilter = 'todos' | EventCategory

export function eventCategory(ev: Pick<MotionEvent, 'label'>): EventCategory {
  const label = (ev.label ?? '').trim()
  if (!label) return 'movimento'
  if (/pessoa|person/i.test(label)) return 'pessoa'
  return 'ia'
}

// Título legível do evento por categoria, para o card do painel de eventos.
export function eventTitle(ev: Pick<MotionEvent, 'label'>): string {
  switch (eventCategory(ev)) {
    case 'pessoa': return 'Pessoa detectada'
    case 'movimento': return 'Movimento detectado'
    case 'estados': return (ev.label ?? '').trim() || 'Estado'
    case 'ia': return (ev.label ?? '').trim() || 'Detecção IA'
  }
}

export function filterEventsByCategory<T extends Pick<MotionEvent, 'label'>>(
  events: T[],
  filter: EventFilter,
): T[] {
  if (filter === 'todos') return events
  return events.filter(ev => eventCategory(ev) === filter)
}
