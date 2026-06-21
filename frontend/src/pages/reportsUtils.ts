import { eventCategory, type EventCategory } from './eventCategory'

export interface EventReport {
  total: number
  by_day: { day: string; count: number }[]
  by_label: Record<string, number>
  by_camera: Record<string, number>
}

// categoryBuckets dobra as contagens cruas por label nas categorias do app
// (mesma regra do eventCategory): label vazio → movimento, pessoa → pessoa, etc.
export function categoryBuckets(byLabel: Record<string, number>): Record<EventCategory, number> {
  const out: Record<EventCategory, number> = { movimento: 0, pessoa: 0, ia: 0, estados: 0 }
  for (const [label, count] of Object.entries(byLabel)) {
    out[eventCategory({ label })] += count
  }
  return out
}
