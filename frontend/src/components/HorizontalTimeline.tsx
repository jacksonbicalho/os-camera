import type { Recording, MotionEvent } from '../pages/cameraUtils'
import { eventCategory } from '../pages/eventCategory'
import {
  timelineWindow,
  timePosFraction,
  isInWindow,
  timelineTicks,
  type TimelineRange,
} from './timelineScale'
import { ChevronLeft, ChevronRight } from './Icons'

const RANGES: TimelineRange[] = ['1h', '6h', '24h', '7d']

// Cor (bg-*) por categoria de marca na trilha e na legenda.
const CAT_COLOR: Record<string, string> = {
  continua: 'bg-blue-500',
  movimento: 'bg-amber-400',
  pessoa: 'bg-red-500',
  ia: 'bg-violet-500',
  estados: 'bg-green-500',
}

const LEGEND: Array<{ key: string; label: string }> = [
  { key: 'continua', label: 'Contínua' },
  { key: 'movimento', label: 'Movimento' },
  { key: 'pessoa', label: 'Pessoa' },
  { key: 'ia', label: 'IA' },
  { key: 'estados', label: 'Estados' },
]

const CHUNK_FALLBACK_MS = 5 * 60_000

interface HorizontalTimelineProps {
  recordings: Recording[]
  events: MotionEvent[]
  range: TimelineRange
  onRangeChange: (r: TimelineRange) => void
  /** Fim da janela (anchor) em ms — a janela recua a duração do range. */
  endMs: number
  /** Rótulo da data selecionada. */
  dateLabel: string
  onPrevDate: () => void
  onNextDate: () => void
  /** Formata um timestamp (ms) num rótulo de tick (ex: HH:MM). */
  formatTick: (ms: number) => string
}

// HorizontalTimeline — régua de tempo horizontal sob o player (redesign do
// Escopo B): seletor de janela, navegação de data, legenda e a trilha com a
// faixa de gravação contínua + marcas de evento por categoria. Esta história
// (#5) entrega só a escala/marcas; ponteiro e seek por clique vêm na #6.
export default function HorizontalTimeline({
  recordings,
  events,
  range,
  onRangeChange,
  endMs,
  dateLabel,
  onPrevDate,
  onNextDate,
  formatTick,
}: HorizontalTimelineProps) {
  const win = timelineWindow(endMs, range)
  const ticks = timelineTicks(win, 7)

  return (
    <div id="horizontal-timeline" className="flex-none bg-surface border border-border rounded-lg px-3 py-2 flex flex-col gap-2">
      {/* Controles: navegação de data + seletor de janela + legenda */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-1">
          <button
            id="timeline-date-prev"
            onClick={onPrevDate}
            title="Dia anterior"
            className="p-1 text-muted hover:text-foreground transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
          <span id="timeline-date-label" className="text-xs text-foreground tabular-nums min-w-[5.5rem] text-center">{dateLabel}</span>
          <button
            id="timeline-date-next"
            onClick={onNextDate}
            title="Próximo dia"
            className="p-1 text-muted hover:text-foreground transition-colors"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>

        <div id="timeline-range" className="flex items-center gap-1">
          {RANGES.map(r => (
            <button
              key={r}
              id={`timeline-range-${r}`}
              onClick={() => onRangeChange(r)}
              className={`px-2 py-0.5 rounded text-xs transition-colors ${
                r === range ? 'bg-primary text-primary-foreground' : 'bg-surface-2 text-muted hover:text-foreground'
              }`}
            >
              {r}
            </button>
          ))}
        </div>

        <div id="timeline-legend" className="flex items-center gap-3 ml-auto">
          {LEGEND.map(l => (
            <span key={l.key} className="flex items-center gap-1.5 text-[11px] text-muted">
              <span className={`w-2 h-2 rounded-full ${CAT_COLOR[l.key]}`} />
              {l.label}
            </span>
          ))}
        </div>
      </div>

      {/* Trilha */}
      <div id="timeline-track" className="relative h-8 rounded bg-surface-2/60 overflow-hidden">
        {/* Faixa de gravação contínua */}
        {recordings.map(rec => {
          const startMs = Date.parse(rec.start)
          const segEnd = startMs + CHUNK_FALLBACK_MS
          if (segEnd < win.startMs || startMs > win.endMs) return null
          const left = timePosFraction(startMs, win)
          const right = timePosFraction(segEnd, win)
          return (
            <span
              key={`rec-${rec.id}`}
              id={`timeline-rec-${rec.id}`}
              className="absolute top-1/2 -translate-y-1/2 h-2 bg-blue-500/70"
              style={{ left: `${left * 100}%`, width: `${Math.max(right - left, 0) * 100}%` }}
            />
          )
        })}

        {/* Marcas de evento por categoria */}
        {events.map(ev => {
          const ms = Date.parse(ev.time)
          if (!isInWindow(ms, win)) return null
          const cat = eventCategory(ev)
          const left = timePosFraction(ms, win)
          return (
            <span
              key={`mark-${ev.id}`}
              id={`timeline-mark-${ev.id}`}
              title={ev.label || 'Movimento'}
              className={`absolute top-0 bottom-0 w-0.5 ${CAT_COLOR[cat] ?? CAT_COLOR.movimento}`}
              style={{ left: `${left * 100}%` }}
            />
          )
        })}
      </div>

      {/* Rótulos de tempo */}
      <div className="relative h-4">
        {ticks.map((t, i) => {
          const left = timePosFraction(t, win)
          return (
            <span
              key={i}
              className="absolute -translate-x-1/2 text-[10px] text-faint tabular-nums"
              style={{ left: `${left * 100}%` }}
            >
              {formatTick(t)}
            </span>
          )
        })}
      </div>
    </div>
  )
}
