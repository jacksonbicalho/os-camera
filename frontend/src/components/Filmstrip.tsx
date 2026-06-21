import { useState, useRef, useEffect } from 'react'
import type { Recording, MotionEvent } from '../pages/cameraUtils'
import { recordingCategory } from '../pages/eventCategory'
import { filmstripSamples, type TimelineWindow } from './timelineScale'
import { ChevronLeft, ChevronRight } from './Icons'

const CHUNK_FALLBACK_MS = 5 * 60_000

// Cor da borda por categoria (mesma legenda da timeline).
const CAT_BORDER: Record<string, string> = {
  continua: 'border-blue-500',
  movimento: 'border-amber-400',
  pessoa: 'border-red-500',
  ia: 'border-violet-500',
  estados: 'border-green-500',
}

// Fundo do rótulo de horário por categoria (a cor envolve também o tempo).
const CAT_BG: Record<string, string> = {
  continua: 'bg-blue-500 text-white',
  movimento: 'bg-amber-400 text-black',
  pessoa: 'bg-red-500 text-white',
  ia: 'bg-violet-500 text-white',
  estados: 'bg-green-500 text-white',
}

interface FilmstripProps {
  recordings: Recording[]
  /** Eventos do dia — coloriam o thumbnail pela categoria do chunk. */
  events: MotionEvent[]
  win: TimelineWindow
  /** Monta a URL do thumbnail (event-frame) para um timestamp (ms). */
  thumbSrc: (ms: number) => string
  /** Formata o horário exibido sob a miniatura. */
  formatTime: (ms: number) => string
  /** Seek para a gravação amostrada (clique na miniatura). */
  onSeek: (rec: Recording, offsetSeconds: number) => void
  /** Gravação tocando — destaca o thumbnail correspondente. */
  activeRecordingId?: number
  /** Vídeo em reprodução — faz a borda do thumbnail ativo piscar. */
  playing?: boolean
}

// Filmstrip — tira com TODAS as miniaturas de gravação da janela (rolável, com
// setas). Cada thumbnail é colorido pela categoria do chunk (legenda) e o da
// gravação ativa é destacado. Sem gravações na janela, não renderiza nada.
export default function Filmstrip({
  recordings,
  events,
  win,
  thumbSrc,
  formatTime,
  onSeek,
  activeRecordingId,
  playing,
}: FilmstripProps) {
  const [failed, setFailed] = useState<Set<number>>(new Set())
  const scrollRef = useRef<HTMLDivElement>(null)

  const samples = filmstripSamples(recordings, win)

  // Rola o filmstrip para centralizar a gravação que está tocando.
  useEffect(() => {
    const c = scrollRef.current
    if (!c || activeRecordingId == null) return
    const el = c.querySelector<HTMLElement>(`#filmstrip-${activeRecordingId}`)
    if (!el) return
    const target = c.scrollLeft + (el.getBoundingClientRect().left - c.getBoundingClientRect().left) - (c.clientWidth - el.clientWidth) / 2
    c.scrollTo({ left: Math.max(0, target), behavior: 'smooth' })
  }, [activeRecordingId, samples.length])

  if (samples.length === 0) return null

  const scrollByDir = (dir: number) => scrollRef.current?.scrollBy({ left: dir * 320, behavior: 'smooth' })

  const arrowClass = 'shrink-0 w-6 h-16 flex items-center justify-center rounded text-muted hover:text-foreground hover:bg-surface-2 transition-colors'

  return (
    <div className="flex-none flex items-center gap-1">
      <button onClick={() => scrollByDir(-1)} title="Anterior" aria-label="Anterior" className={arrowClass}>
        <ChevronLeft className="w-4 h-4" />
      </button>

      <div ref={scrollRef} id="filmstrip" className="flex-1 flex gap-2 overflow-x-auto pb-1 [&::-webkit-scrollbar]:h-1 [&::-webkit-scrollbar-thumb]:bg-surface-2 [&::-webkit-scrollbar-thumb]:rounded-full">
        {samples.map(s => {
          const active = s.rec.id === activeRecordingId
          const cat = recordingCategory(s.rec, events, CHUNK_FALLBACK_MS)
          // Mantém a borda na cor da categoria (não vira azul, pra não conflitar
          // com a legenda). O ativo só se distingue piscando enquanto reproduz.
          const frameClass = `w-28 h-16 rounded border-2 transition-colors bg-surface-2 ${CAT_BORDER[cat] ?? 'border-border'} group-hover:border-primary`
          const blinkStyle = active && playing ? { animation: 'filmstrip-blink 1.1s ease-in-out infinite' } : undefined
          return (
            <button
              key={s.rec.id}
              id={`filmstrip-${s.rec.id}`}
              aria-current={active ? 'true' : undefined}
              onClick={() => onSeek(s.rec, s.offsetSeconds)}
              className="shrink-0 flex flex-col items-center gap-1 group cursor-pointer"
            >
              {failed.has(s.rec.id) ? (
                <div style={blinkStyle} className={`${frameClass} flex items-center justify-center text-[10px] text-faint`}>sem prévia</div>
              ) : (
                <img
                  src={thumbSrc(s.ms)}
                  alt={formatTime(s.ms)}
                  loading="lazy"
                  style={blinkStyle}
                  onError={() => setFailed(prev => new Set(prev).add(s.rec.id))}
                  className={`${frameClass} object-cover`}
                />
              )}
              <span className={`text-[10px] tabular-nums px-1.5 py-0.5 rounded ${CAT_BG[cat] ?? 'bg-surface-2 text-muted'}`}>{formatTime(s.ms)}</span>
            </button>
          )
        })}
      </div>

      <button onClick={() => scrollByDir(1)} title="Próximo" aria-label="Próximo" className={arrowClass}>
        <ChevronRight className="w-4 h-4" />
      </button>
    </div>
  )
}
