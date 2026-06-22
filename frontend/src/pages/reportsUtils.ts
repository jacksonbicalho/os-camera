import { eventCategory, type EventCategory } from './eventCategory'

export interface EventReport {
  total: number
  by_day: { day: string; count: number; by_category?: Record<string, number> }[]
  by_label: Record<string, number>
  // estados (e futuras categorias não derivadas de label) já vêm bucketizadas do backend
  by_category?: Record<string, number>
}

// categoryBuckets dobra as contagens cruas por label nas categorias do app (mesma regra
// do eventCategory): label vazio → movimento, pessoa → pessoa, etc. `byCategory` (opcional)
// traz categorias que não vêm de label — hoje `estados` (de camera_state_history).
export function categoryBuckets(
  byLabel: Record<string, number>,
  byCategory?: Record<string, number>,
): Record<EventCategory, number> {
  const out: Record<EventCategory, number> = { movimento: 0, pessoa: 0, ia: 0, estados: 0 }
  for (const [label, count] of Object.entries(byLabel)) {
    out[eventCategory({ label })] += count
  }
  for (const [cat, count] of Object.entries(byCategory ?? {})) {
    if (cat in out) out[cat as EventCategory] += count
  }
  return out
}
