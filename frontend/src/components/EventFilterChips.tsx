import type { EventFilter } from '../pages/eventCategory'

const CHIPS: Array<{ key: EventFilter; label: string; dot?: string }> = [
  { key: 'todos', label: 'Todos' },
  { key: 'movimento', label: 'Movimento', dot: 'bg-amber-400' },
  { key: 'pessoa', label: 'Pessoa', dot: 'bg-red-500' },
  { key: 'ia', label: 'IA', dot: 'bg-violet-500' },
  { key: 'estados', label: 'Estados', dot: 'bg-green-500' },
]

interface EventFilterChipsProps {
  value: EventFilter
  onChange: (f: EventFilter) => void
  counts?: Partial<Record<EventFilter, number>>
}

// EventFilterChips — fileira de chips de filtro do painel de eventos (redesign do
// Escopo B): Todos/Movimento/Pessoa/IA/Estados, cada um com ponto colorido por
// categoria e contagem opcional.
export default function EventFilterChips({ value, onChange, counts }: EventFilterChipsProps) {
  return (
    <div id="event-filter-chips" className="flex flex-wrap items-center gap-1.5 px-3 py-2 border-b border-border shrink-0">
      {CHIPS.map(chip => {
        const active = value === chip.key
        const count = counts?.[chip.key]
        return (
          <button
            key={chip.key}
            id={`event-chip-${chip.key}`}
            onClick={() => onChange(chip.key)}
            className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs transition-colors ${
              active ? 'bg-primary text-primary-foreground' : 'bg-surface-2 text-muted hover:text-foreground'
            }`}
          >
            {chip.dot && <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${chip.dot}`} />}
            <span>{chip.label}</span>
            {count != null && count > 0 && <span className="tabular-nums opacity-80">{count}</span>}
          </button>
        )
      })}
    </div>
  )
}
