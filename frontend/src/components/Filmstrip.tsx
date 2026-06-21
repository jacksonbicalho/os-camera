import { useState } from 'react'
import type { Recording } from '../pages/cameraUtils'
import { filmstripSamples, type TimelineWindow } from './timelineScale'

interface FilmstripProps {
  recordings: Recording[]
  win: TimelineWindow
  /** Monta a URL do thumbnail (event-frame) para um timestamp (ms). */
  thumbSrc: (ms: number) => string
  /** Formata o horário exibido sob a miniatura. */
  formatTime: (ms: number) => string
  /** Seek para a gravação amostrada (clique na miniatura). */
  onSeek: (rec: Recording, offsetSeconds: number) => void
  count?: number
}

// Filmstrip — tira de miniaturas amostradas ao longo da janela da timeline
// (redesign do Escopo B). Reusa o endpoint event-frame (via thumbSrc) e o seek da
// timeline. Sem gravações na janela, não renderiza nada.
export default function Filmstrip({
  recordings,
  win,
  thumbSrc,
  formatTime,
  onSeek,
  count = 10,
}: FilmstripProps) {
  const samples = filmstripSamples(recordings, win, count)
  const [failed, setFailed] = useState<Set<number>>(new Set())
  if (samples.length === 0) return null

  return (
    <div id="filmstrip" className="flex-none flex gap-2 overflow-x-auto pb-1 [&::-webkit-scrollbar]:h-1 [&::-webkit-scrollbar-thumb]:bg-surface-2 [&::-webkit-scrollbar-thumb]:rounded-full">
      {samples.map(s => (
        <button
          key={s.rec.id}
          id={`filmstrip-${s.rec.id}`}
          onClick={() => onSeek(s.rec, s.offsetSeconds)}
          className="shrink-0 flex flex-col items-center gap-1 group cursor-pointer"
        >
          {failed.has(s.rec.id) ? (
            <div className="w-28 h-16 rounded border border-border bg-surface-2 flex items-center justify-center text-[10px] text-faint">
              sem prévia
            </div>
          ) : (
            <img
              src={thumbSrc(s.ms)}
              alt={formatTime(s.ms)}
              loading="lazy"
              onError={() => setFailed(prev => new Set(prev).add(s.rec.id))}
              className="w-28 h-16 object-cover rounded border border-border group-hover:border-primary transition-colors bg-surface-2"
            />
          )}
          <span className="text-[10px] text-muted tabular-nums">{formatTime(s.ms)}</span>
        </button>
      ))}
    </div>
  )
}
