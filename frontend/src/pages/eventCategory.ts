import type { MotionEvent, Recording } from './cameraUtils'

// Categoria de um evento (redesign do Escopo B — chips do painel de eventos).
// Transições de classificador de estado chegam ao feed com `kind:'state'` (mescladas
// no backend a partir de camera_state_history) e caem em `estados`, independente do
// label; os demais derivam do label.
export type EventCategory = 'movimento' | 'pessoa' | 'ia' | 'estados'
export type EventFilter = 'todos' | EventCategory

export function eventCategory(ev: Pick<MotionEvent, 'label' | 'kind'>): EventCategory {
  if (ev.kind === 'state') return 'estados'
  const label = (ev.label ?? '').trim()
  if (!label) return 'movimento'
  if (/pessoa|person/i.test(label)) return 'pessoa'
  return 'ia'
}

export type RecordingCategory = EventCategory | 'continua'

// Prioridade (maior → menor) para resolver a categoria de um chunk com vários
// eventos: pessoa > ia > movimento.
const CAT_PRIORITY: EventCategory[] = ['pessoa', 'ia', 'movimento']

// recordingCategory classifica um chunk de gravação pela categoria dos eventos
// no seu intervalo [start, start+chunk): a de maior prioridade; `continua` se
// não houver evento. Usado para colorir o thumbnail no filmstrip (legenda).
export function recordingCategory(
  rec: Pick<Recording, 'start'>,
  events: Pick<MotionEvent, 'time' | 'label' | 'kind'>[],
  chunkMs: number,
): RecordingCategory {
  const start = Date.parse(rec.start)
  if (Number.isNaN(start)) return 'continua'
  const end = start + chunkMs
  let best: EventCategory | null = null
  for (const ev of events) {
    const t = Date.parse(ev.time)
    if (Number.isNaN(t) || t < start || t >= end) continue
    const cat = eventCategory(ev)
    if (cat === 'estados') continue
    if (best === null || CAT_PRIORITY.indexOf(cat) < CAT_PRIORITY.indexOf(best)) best = cat
  }
  return best ?? 'continua'
}

// Título legível do evento por categoria, para o card do painel de eventos.
export function eventTitle(ev: Pick<MotionEvent, 'label' | 'kind'>): string {
  switch (eventCategory(ev)) {
    case 'pessoa': return 'Pessoa detectada'
    case 'movimento': return 'Movimento detectado'
    case 'estados': return (ev.label ?? '').trim() || 'Estado'
    case 'ia': return (ev.label ?? '').trim() || 'Detecção IA'
  }
}

export function filterEventsByCategory<T extends Pick<MotionEvent, 'label' | 'kind'>>(
  events: T[],
  filter: EventFilter,
): T[] {
  if (filter === 'todos') return events
  return events.filter(ev => eventCategory(ev) === filter)
}
