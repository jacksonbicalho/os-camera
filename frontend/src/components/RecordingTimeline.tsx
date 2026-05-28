import { useRef, useCallback, useEffect, useState } from 'react'
import type { Recording, MotionEvent } from '../pages/cameraUtils'

interface Props {
  recordings: Recording[]
  motionEvents: MotionEvent[]
  activeRecording: Recording | null
  activeTime: string | null
  timezone: string
  onSeek: (recording: Recording, offsetSeconds: number) => void
}

const TOTAL_H = 72
const RULER_H = 26
const SEG_TOP = 34
const SEG_H   = 30

const ZOOM_LEVELS = [1, 2, 4, 8]

export default function RecordingTimeline({ recordings, motionEvents, activeRecording, activeTime, timezone, onSeek }: Props) {
  const outerRef     = useRef<HTMLDivElement>(null)
  const scrollRef    = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const isDragging   = useRef(false)
  const [zoom, setZoom] = useState(1)
  const zoomIdx = ZOOM_LEVELS.indexOf(zoom)

  const recsAsc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
  const N = recsAsc.length

  const blocks = recsAsc.map((rec, i) => {
    const startMs = new Date(rec.start).getTime()
    const endMs = i + 1 < recsAsc.length
      ? new Date(recsAsc[i + 1].start).getTime()
      : startMs + 5 * 60 * 1000
    return { rec, startMs, endMs, startFrac: i / N, widthFrac: 1 / N }
  })

  const rangeStart = blocks.length > 0 ? blocks[0].startMs : 0
  const rangeEnd   = blocks.length > 0 ? blocks[blocks.length - 1].endMs : 1
  const totalMs    = Math.max(rangeEnd - rangeStart, 1)

  const timeToFrac = (ms: number): number => {
    for (let i = 0; i < blocks.length; i++) {
      const { startMs, endMs } = blocks[i]
      if (ms <= endMs) {
        const span = endMs - startMs
        const within = span > 0 ? Math.max(0, Math.min(1, (ms - startMs) / span)) : 0
        return (i + within) / N
      }
    }
    return 1
  }

  const eventDots = motionEvents
    .map(ev => ({ ev, f: timeToFrac(new Date(ev.time).getTime()) }))
    .filter(d => d.f >= 0 && d.f <= 1)

  const activeFrac = activeTime !== null
    ? Math.max(0, Math.min(1, timeToFrac(new Date(activeTime).getTime())))
    : null

  // Label interval ajustado pelo zoom: mais zoom → mais labels
  const rangeMin = totalMs / 60000
  const baseIntervalMin =
    rangeMin <= 10  ? 1  :
    rangeMin <= 30  ? 2  :
    rangeMin <= 120 ? 5  :
    rangeMin <= 360 ? 10 :
    rangeMin <= 720 ? 15 : 30
  const intervalMin = Math.max(1, Math.floor(baseIntervalMin / zoom))

  const intervalMs    = intervalMin * 60000
  const firstBoundary = Math.ceil(rangeStart / intervalMs) * intervalMs
  const fmt = new Intl.DateTimeFormat('pt-BR', {
    timeZone: timezone, hour: '2-digit', minute: '2-digit', hour12: false,
  })
  const labels: { f: number; label: string }[] = []
  for (let ms = firstBoundary; ms <= rangeEnd; ms += intervalMs) {
    labels.push({ f: timeToFrac(ms), label: fmt.format(new Date(ms)) })
  }
  const minorStepMin = Math.max(1, Math.floor(intervalMin / 5))
  const minorStepMs  = minorStepMin * 60000
  const firstMinorBoundary = Math.ceil(rangeStart / minorStepMs) * minorStepMs
  const minorTicks: number[] = []
  for (let ms = firstMinorBoundary; ms <= rangeEnd; ms += minorStepMs) {
    if ((ms - firstBoundary) % intervalMs === 0) continue
    minorTicks.push(timeToFrac(ms))
  }

  const seekAtX = useCallback((clientX: number) => {
    const el = containerRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const f = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width))
    const n = blocks.length
    if (n === 0) return
    const blockIdx = Math.min(n - 1, Math.floor(f * n))
    const withinFrac = f * n - blockIdx
    const { rec, startMs, endMs } = blocks[blockIdx]
    if (rec.is_recording) return
    const offsetSeconds = Math.max(0, withinFrac * (endMs - startMs) / 1000)
    onSeek(rec, offsetSeconds)
  }, [blocks, onSeek])

  // Mantém a agulha centrada no scroll quando o zoom ou posição ativa muda
  useEffect(() => {
    const el = scrollRef.current
    if (!el || activeFrac === null) return
    const target = activeFrac * el.scrollWidth - el.clientWidth / 2
    el.scrollLeft = Math.max(0, Math.min(el.scrollWidth - el.clientWidth, target))
  }, [activeFrac, zoom])


  const activeLabel = activeTime ? fmt.format(new Date(activeTime)) : null

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === '+' || e.key === '=') {
      e.preventDefault()
      setZoom(z => ZOOM_LEVELS[Math.min(ZOOM_LEVELS.length - 1, ZOOM_LEVELS.indexOf(z) + 1)])
      return
    }
    if (e.key === '-') {
      e.preventDefault()
      setZoom(z => ZOOM_LEVELS[Math.max(0, ZOOM_LEVELS.indexOf(z) - 1)])
      return
    }
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const currentIdx = activeRecording
      ? blocks.findIndex(b => b.rec.filename === activeRecording.filename)
      : -1
    const targetIdx = e.key === 'ArrowRight' ? currentIdx + 1 : currentIdx - 1
    if (targetIdx < 0 || targetIdx >= blocks.length) return
    const { rec } = blocks[targetIdx]
    if (!rec.is_recording) onSeek(rec, 0)
  }

  if (N === 0) {
    return (
      <div className="w-full rounded-lg bg-zinc-950 border border-zinc-800" style={{ height: TOTAL_H }} />
    )
  }

  return (
    <div
      ref={outerRef}
      tabIndex={0}
      className="relative w-full rounded-lg bg-zinc-950 border border-zinc-800 select-none focus:outline-none focus:ring-1 focus:ring-amber-500/50"
      style={{ height: TOTAL_H }}
      title="Linha do tempo — clique ou arraste · ← → gravação · +/− zoom"
      onKeyDown={handleKeyDown}
    >
      {/* Botões de zoom */}
      <div
        className="absolute top-1 right-1 z-40 flex items-center gap-0.5"
        onMouseDown={e => e.stopPropagation()}
      >
        <button
          onClick={() => setZoom(z => ZOOM_LEVELS[Math.max(0, ZOOM_LEVELS.indexOf(z) - 1)])}
          disabled={zoomIdx === 0}
          className="w-5 h-5 flex items-center justify-center rounded text-amber-400/70 hover:text-amber-300 hover:bg-amber-500/10 disabled:opacity-30 disabled:cursor-not-allowed text-xs leading-none transition-colors"
          title="Diminuir zoom (−)"
        >−</button>
        <span className="text-[9px] text-amber-400/50 tabular-nums w-5 text-center">{zoom}×</span>
        <button
          onClick={() => setZoom(z => ZOOM_LEVELS[Math.min(ZOOM_LEVELS.length - 1, ZOOM_LEVELS.indexOf(z) + 1)])}
          disabled={zoomIdx === ZOOM_LEVELS.length - 1}
          className="w-5 h-5 flex items-center justify-center rounded text-amber-400/70 hover:text-amber-300 hover:bg-amber-500/10 disabled:opacity-30 disabled:cursor-not-allowed text-xs leading-none transition-colors"
          title="Aumentar zoom (+)"
        >+</button>
      </div>

      {/* Área rolável */}
      <div
        ref={scrollRef}
        className="w-full h-full overflow-x-auto overflow-y-hidden"
        style={{ scrollbarWidth: 'none' }}
      >
        {/* Conteúdo escalado */}
        <div
          ref={containerRef}
          className="relative h-full cursor-pointer"
          style={{ width: `${zoom * 100}%`, minWidth: '100%' }}
          onMouseDown={e => { isDragging.current = true; seekAtX(e.clientX); outerRef.current?.focus() }}
          onMouseMove={e => { if (isDragging.current) seekAtX(e.clientX) }}
          onMouseUp={() => { isDragging.current = false }}
          onMouseLeave={() => { isDragging.current = false }}
        >
          {/* Régua */}
          <div className="absolute left-0 right-0 top-0 bg-zinc-900 border-b border-zinc-700/50" style={{ height: RULER_H }} />

          {/* Labels e ticks principais */}
          {labels.map(({ f, label }) => (
            <div
              key={label}
              className="absolute top-0 bottom-0 pointer-events-none"
              style={{ left: `${f * 100}%` }}
            >
              <div className="absolute top-0 w-px bg-amber-500/50" style={{ height: RULER_H }} />
              <div className="absolute w-px bg-amber-500/10" style={{ top: RULER_H, bottom: 0 }} />
              <span
                className="absolute text-amber-400/80 leading-none whitespace-nowrap"
                style={{ fontSize: 9, top: 4, transform: 'translateX(-50%)' }}
              >
                {label}
              </span>
            </div>
          ))}

          {/* Ticks menores */}
          {minorTicks.map((f, i) => (
            <div
              key={`m-${i}`}
              className="absolute top-0 pointer-events-none w-px bg-amber-500/20"
              style={{ left: `${f * 100}%`, height: RULER_H * 0.5 }}
            />
          ))}

          {/* Blocos de gravação */}
          {blocks.map(({ rec, startFrac, widthFrac }) => {
            const isActive = activeRecording?.filename === rec.filename
            return (
              <div
                key={rec.filename}
                className={`absolute rounded-sm pointer-events-none transition-colors ${
                  rec.is_recording
                    ? 'bg-red-400/80'
                    : isActive
                      ? 'bg-amber-200/90'
                      : 'bg-amber-500/50'
                }`}
                style={{
                  left:   `${startFrac * 100}%`,
                  width:  `max(${widthFrac * 100}%, 2px)`,
                  top:    SEG_TOP,
                  height: SEG_H,
                }}
              />
            )
          })}

          {/* Dots de eventos de movimento */}
          {eventDots.map(({ ev, f }, i) => (
            <div
              key={ev.id ?? `${ev.time}-${i}`}
              className="absolute rounded-full pointer-events-none"
              style={{
                left:            `${f * 100}%`,
                top:             SEG_TOP - 7,
                width:           5,
                height:          5,
                transform:       'translateX(-50%)',
                backgroundColor: ev.color ?? '#fb923c',
              }}
            />
          ))}

          {/* Agulha */}
          {activeFrac !== null && (
            <div
              className="absolute top-0 bottom-0 z-20 pointer-events-none"
              style={{
                left:            `${activeFrac * 100}%`,
                width:           2,
                transform:       'translateX(-50%)',
                backgroundColor: '#f97316',
                boxShadow:       '0 0 6px 1px rgba(249,115,22,0.5)',
              }}
            />
          )}

          {/* Label de tempo ativo */}
          {activeFrac !== null && activeLabel && (
            <div
              className="absolute z-30 pointer-events-none px-1.5 py-0.5 rounded bg-orange-500/90 text-[10px] font-semibold text-white whitespace-nowrap"
              style={{
                top:       RULER_H + 2,
                left:      `${activeFrac * 100}%`,
                transform: 'translateX(-50%)',
              }}
            >
              {activeLabel}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
