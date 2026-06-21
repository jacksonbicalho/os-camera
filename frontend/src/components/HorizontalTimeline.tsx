import { useRef, useEffect, useState } from 'react'
import type { Recording, MotionEvent } from '../pages/cameraUtils'
import { eventCategory } from '../pages/eventCategory'
import {
  timelineWindow,
  timePosFraction,
  isInWindow,
  timelineTicks,
  posToTime,
  recordingAtMs,
  type TimelineRange,
} from './timelineScale'
import DatePicker from './DatePicker'

const RANGES: TimelineRange[] = ['1h', '6h', '24h']

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
  /** Data selecionada (abre o calendário em popover ao clicar). */
  selectedDate: Date
  onSelectDate: (d: Date) => void
  /** Formata um timestamp (ms) num rótulo de tick (ex: HH:MM). */
  formatTick: (ms: number) => string
  /** Posição de reprodução atual (ms absolutos) — desenha o ponteiro. */
  playheadMs?: number
  /** Seek para uma gravação num offset (clique/arraste sobre gravação). */
  onSeek?: (rec: Recording, offsetSeconds: number) => void
  /** Scrub contínuo (arraste). Cai em onSeek quando ausente. */
  onScrub?: (rec: Recording, offsetSeconds: number) => void
  /** Posição numa lacuna (sem gravação no instante). */
  onGap?: (ms: number) => void
  /** Duração típica de um chunk de gravação (ms), para mapear ms → gravação. */
  chunkMs?: number
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
  selectedDate,
  onSelectDate,
  formatTick,
  playheadMs,
  onSeek,
  onScrub,
  onGap,
  chunkMs = CHUNK_FALLBACK_MS,
}: HorizontalTimelineProps) {
  const win = timelineWindow(endMs, range)
  const ticks = timelineTicks(win, 7)
  const trackRef = useRef<HTMLDivElement>(null)
  const [dragging, setDragging] = useState(false)

  function seekAtClientX(clientX: number, scrub = false) {
    const el = trackRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    if (rect.width <= 0) return
    const ms = posToTime((clientX - rect.left) / rect.width, win)
    const hit = recordingAtMs(recordings, ms, chunkMs)
    if (hit) {
      const fn = scrub && onScrub ? onScrub : onSeek
      fn?.(hit.rec, hit.offsetSeconds)
    } else {
      onGap?.(ms)
    }
  }

  useEffect(() => {
    if (!dragging) return
    const onMove = (e: MouseEvent) => seekAtClientX(e.clientX, true)
    const onUp = () => setDragging(false)
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
    // seekAtClientX é estável o bastante para o ciclo de drag; deps mínimas.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dragging])

  return (
    <div id="horizontal-timeline" className="flex-none bg-surface border border-border rounded-lg px-3 py-2 flex flex-col gap-2">
      {/* Controles: navegação de data + seletor de janela + legenda */}
      <div className="flex items-center gap-3 flex-wrap">
        {/* Data — popover do calendário unificado */}
        <DatePicker id="timeline-date" value={selectedDate} onChange={onSelectDate} disableFuture openUp />

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

      {/* Trilha + ponteiro (ponteiro fica fora do overflow para não cortar o rótulo) */}
      <div className="relative">
      <div
        id="timeline-track"
        ref={trackRef}
        onClick={(e) => seekAtClientX(e.clientX)}
        onMouseDown={(e) => { setDragging(true); seekAtClientX(e.clientX, true) }}
        className="relative h-8 rounded bg-surface-2/60 overflow-hidden cursor-pointer select-none"
      >
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

        {/* Ponteiro de reprodução — destacado, sobre a trilha sem clipping */}
        {playheadMs != null && isInWindow(playheadMs, win) && (
          <div
            id="timeline-pointer"
            className="absolute top-0 h-8 w-[2px] bg-white shadow-[0_0_8px_2px_rgba(255,255,255,0.45)] z-20 pointer-events-none"
            style={{ left: `${timePosFraction(playheadMs, win) * 100}%` }}
          >
            {/* knob no topo — alça de arraste (maior, com área de clique generosa) */}
            <button
              type="button"
              aria-label="Arrastar ponteiro"
              onMouseDown={(e) => { e.stopPropagation(); setDragging(true) }}
              className="absolute -top-2.5 left-1/2 -translate-x-1/2 w-6 h-6 flex items-center justify-center pointer-events-auto cursor-grab active:cursor-grabbing"
            >
              <span className="w-4 h-4 rounded-full bg-white ring-2 ring-primary shadow-md" />
            </button>
            {/* rótulo de horário abaixo da trilha */}
            <span className="absolute top-full mt-1 left-1/2 -translate-x-1/2 px-1.5 py-0.5 rounded bg-primary text-primary-foreground text-[10px] font-semibold tabular-nums whitespace-nowrap shadow-md">
              {formatTick(playheadMs)}
            </span>
          </div>
        )}
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
