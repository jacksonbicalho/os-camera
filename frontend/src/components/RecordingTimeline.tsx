import { useRef, useCallback, useEffect } from 'react'
import type { Recording, MotionEvent } from '../pages/cameraUtils'

interface Props {
  recordings: Recording[]
  motionEvents: MotionEvent[]
  activeRecording: Recording | null
  activeTime: string | null
  timezone: string
  sortOrder: 'asc' | 'desc'
  onSeek: (recording: Recording, offsetSeconds: number) => void
}

const RULER_W = 26
const CENTER_X = 0.5

export default function RecordingTimeline({ recordings, motionEvents, activeRecording, activeTime, timezone, sortOrder, onSeek }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const isDragging = useRef(false)

  const recsAsc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
  const N = recsAsc.length

  // Each block knows its recording, its wall-clock span, and its equal-height
  // visual position (startFrac = i/N, heightFrac = 1/N).
  const blocks = recsAsc.map((rec, i) => {
    const startMs = new Date(rec.start).getTime()
    const endMs = i + 1 < recsAsc.length
      ? new Date(recsAsc[i + 1].start).getTime()
      : startMs + 5 * 60 * 1000
    const heightFrac = 1 / N
    const startFrac = sortOrder === 'desc' ? (N - 1 - i) / N : i / N
    return { rec, startMs, endMs, startFrac, heightFrac }
  })

  const rangeStart = blocks.length > 0 ? blocks[0].startMs : 0
  const rangeEnd   = blocks.length > 0 ? blocks[blocks.length - 1].endMs : 1
  const totalMs    = Math.max(rangeEnd - rangeStart, 1)

  // Maps a timestamp to a vertical fraction using equal-height block layout.
  // Each of N recordings occupies 1/N of the height; position within a block
  // is interpolated linearly between that block's startMs and endMs.
  const timeToFrac = (ms: number): number => {
    for (let i = 0; i < blocks.length; i++) {
      const { startMs, endMs } = blocks[i]
      if (ms <= endMs) {
        const span = endMs - startMs
        const within = span > 0 ? Math.max(0, Math.min(1, (ms - startMs) / span)) : 0
        const ascFrac = (i + within) / N
        return sortOrder === 'desc' ? 1 - ascFrac : ascFrac
      }
    }
    return sortOrder === 'desc' ? 0 : 1
  }

  const eventDots = motionEvents
    .map(ev => ({ ev, f: timeToFrac(new Date(ev.time).getTime()) }))
    .filter(d => d.f >= 0 && d.f <= 1)

  const activeFrac = activeTime !== null
    ? Math.max(0, Math.min(1, timeToFrac(new Date(activeTime).getTime())))
    : null

  // Adaptive label interval (minutes)
  const rangeMin = totalMs / 60000
  const intervalMin =
    rangeMin <= 10  ? 1  :
    rangeMin <= 30  ? 2  :
    rangeMin <= 120 ? 5  :
    rangeMin <= 360 ? 10 :
    rangeMin <= 720 ? 15 : 30

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
  const minorStepMs = minorStepMin * 60000
  const firstMinorBoundary = Math.ceil(rangeStart / minorStepMs) * minorStepMs
  const minorTicks: number[] = []
  for (let ms = firstMinorBoundary; ms <= rangeEnd; ms += minorStepMs) {
    if ((ms - firstBoundary) % intervalMs === 0) continue
    minorTicks.push(timeToFrac(ms))
  }

  const seekAtY = useCallback((clientY: number) => {
    const el = containerRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const f = Math.max(0, Math.min(1, (clientY - rect.top) / rect.height))
    const ascF = sortOrder === 'desc' ? 1 - f : f
    const n = blocks.length
    if (n === 0) return
    const blockIdx = Math.min(n - 1, Math.floor(ascF * n))
    const withinFrac = ascF * n - blockIdx
    const { rec, startMs, endMs } = blocks[blockIdx]
    if (rec.is_recording) return
    const offsetSeconds = Math.max(0, withinFrac * (endMs - startMs) / 1000)
    onSeek(rec, offsetSeconds)
  }, [blocks, sortOrder, onSeek])

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const onTouchStart = (e: TouchEvent) => { isDragging.current = true; seekAtY(e.touches[0].clientY) }
    const onTouchMove  = (e: TouchEvent) => {
      if (!isDragging.current) return
      e.preventDefault()
      seekAtY(e.touches[0].clientY)
    }
    const onTouchEnd = () => { isDragging.current = false }
    el.addEventListener('touchstart', onTouchStart, { passive: true })
    el.addEventListener('touchmove',  onTouchMove,  { passive: false })
    el.addEventListener('touchend',   onTouchEnd)
    return () => {
      el.removeEventListener('touchstart', onTouchStart)
      el.removeEventListener('touchmove',  onTouchMove)
      el.removeEventListener('touchend',   onTouchEnd)
    }
  }, [seekAtY])

  const activeLabel = activeTime ? fmt.format(new Date(activeTime)) : null

  if (recsAsc.length === 0) return <div className="relative w-full h-full border-l border-gray-800 bg-gray-900/40" />

  return (
    <div
      ref={containerRef}
      className="relative w-full h-full border-l border-gray-800 bg-gray-900/40 cursor-pointer select-none overflow-hidden"
      title="Linha do tempo — clique ou arraste para navegar"
      onMouseDown={e => { isDragging.current = true; seekAtY(e.clientY) }}
      onMouseMove={e => { if (isDragging.current) seekAtY(e.clientY) }}
      onMouseUp={() => { isDragging.current = false }}
      onMouseLeave={() => { isDragging.current = false }}
    >
      <div className="absolute inset-y-0 left-0 bg-zinc-800/65 border-r border-zinc-600/45" style={{ width: RULER_W }} />
      <div className="absolute inset-y-0 border-l border-amber-300/80" style={{ left: `calc(${CENTER_X * 100}% - 0.5px)` }} />

      {/* Tick marks + labels */}
      {labels.map(({ f, label }) => (
        <div
          key={label}
          className="absolute inset-x-0 pointer-events-none"
          style={{ top: `${f * 100}%` }}
        >
          <div className="absolute border-t border-zinc-500/45" style={{ left: RULER_W - 2, right: 0 }} />
          <span
            className="absolute text-zinc-300 leading-none whitespace-nowrap"
            style={{ fontSize: 6.5, left: 2, top: 2 }}
          >
            {label}
          </span>
        </div>
      ))}

      {/* Minor ticks for denser time scale */}
      {minorTicks.map((f, i) => (
        <div key={`minor-${i}`} className="absolute inset-x-0 pointer-events-none" style={{ top: `${f * 100}%` }}>
          <div className="absolute border-t border-zinc-600/30" style={{ left: RULER_W - 1, right: 0 }} />
        </div>
      ))}

      {/* Recording marks around center line */}
      {blocks.map(({ rec, startFrac, heightFrac }) => {
        const isActive = activeRecording?.filename === rec.filename
        return (
          <div
            key={rec.filename}
            className={`absolute rounded-full pointer-events-none transition-colors ${
              rec.is_recording
                ? 'bg-red-400/90'
                : isActive
                  ? 'bg-amber-200/95'
                  : 'bg-amber-300/85'
            }`}
            style={{
              top:    `${startFrac * 100}%`,
              height: `max(${heightFrac * 100}%, 1px)`,
              left:   `calc(${CENTER_X * 100}% - ${isActive ? 20 : 14}px)`,
              width:  `${isActive ? 40 : 28}px`,
            }}
          />
        )
      })}

      {/* Motion event dots */}
      {eventDots.map(({ ev, f }, i) => (
        <div
          key={ev.id ?? `${ev.time}-${i}`}
          className="absolute h-[2px] rounded-full pointer-events-none"
          style={{
            top:       `${f * 100}%`,
            left:      `calc(${CENTER_X * 100}% - ${6 + Math.round(Math.max(0, Math.min(1, ev.score)) * 12)}px)`,
            width:     `${(6 + Math.round(Math.max(0, Math.min(1, ev.score)) * 12)) * 2}px`,
            height:    `${1 + Math.round(Math.max(0, Math.min(1, ev.score)) * 2)}px`,
            transform: 'translateY(-50%)',
            backgroundColor: ev.color ?? '#facc15',
          }}
        />
      ))}

      {/* Playhead */}
      {activeFrac !== null && (
        <div
          className="absolute inset-x-0 z-20 pointer-events-none"
          style={{ top: `${activeFrac * 100}%` }}
        >
          <div className="absolute border-t border-amber-200/70" style={{ left: RULER_W, right: 0 }} />
        </div>
      )}

      {activeLabel && (
        <div className="absolute top-1 left-1/2 -translate-x-1/2 z-30 px-2 py-0.5 rounded-full bg-red-700/90 text-[11px] font-semibold text-white pointer-events-none transition-all duration-150 ease-out">
          {activeLabel}
        </div>
      )}
    </div>
  )
}
