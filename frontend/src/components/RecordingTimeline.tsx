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

  const blockData = recsAsc.map((rec, i) => {
    const startMs = new Date(rec.start).getTime()
    const nextMs = i + 1 < recsAsc.length
      ? new Date(recsAsc[i + 1].start).getTime()
      : startMs + 5 * 60 * 1000
    return { rec, startMs, endMs: nextMs }
  })

  const rangeStart = blockData.length > 0 ? blockData[0].startMs : 0
  const rangeEnd   = blockData.length > 0 ? blockData[blockData.length - 1].endMs : 1
  const totalMs    = Math.max(rangeEnd - rangeStart, 1)

  // Flip y-axis when list is sorted descending (newest at top)
  const frac = (ms: number) => {
    const f = (ms - rangeStart) / totalMs
    return sortOrder === 'desc' ? 1 - f : f
  }

  const blocks = blockData.map(({ rec, startMs, endMs }) => {
    const heightFrac = (endMs - startMs) / totalMs
    // When desc, the visual top of the block is the flipped end time
    const startFrac = sortOrder === 'desc'
      ? 1 - (endMs - rangeStart) / totalMs
      : (startMs - rangeStart) / totalMs
    return { rec, startFrac, heightFrac }
  })

  const eventDots = motionEvents
    .map(ev => ({ ev, f: frac(new Date(ev.time).getTime()) }))
    .filter(d => d.f >= 0 && d.f <= 1)

  const activeFrac = activeTime !== null
    ? Math.max(0, Math.min(1, frac(new Date(activeTime).getTime())))
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
    const f = (ms - rangeStart) / totalMs
    if (f < 0 || f > 1) continue
    labels.push({ f, label: fmt.format(new Date(ms)) })
  }
  const minorStepMin = Math.max(1, Math.floor(intervalMin / 5))
  const minorStepMs = minorStepMin * 60000
  const firstMinorBoundary = Math.ceil(rangeStart / minorStepMs) * minorStepMs
  const minorTicks: number[] = []
  for (let ms = firstMinorBoundary; ms <= rangeEnd; ms += minorStepMs) {
    if ((ms - firstBoundary) % intervalMs === 0) continue
    const f = (ms - rangeStart) / totalMs
    if (f >= 0 && f <= 1) minorTicks.push(f)
  }

  const seekAtY = useCallback((clientY: number) => {
    const el = containerRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const f = Math.max(0, Math.min(1, (clientY - rect.top) / rect.height))

    let found: typeof blocks[0] | null = null
    for (const b of blocks) {
      if (f >= b.startFrac && f < b.startFrac + b.heightFrac) { found = b; break }
    }
    if (!found) {
      for (let i = blocks.length - 1; i >= 0; i--) {
        if (blocks[i].startFrac <= f) { found = blocks[i]; break }
      }
    }
    if (!found || found.rec.is_recording) return

    const offsetSeconds = Math.max(0, (f - found.startFrac) * totalMs / 1000)
    onSeek(found.rec, offsetSeconds)
  }, [blocks, totalMs, onSeek])

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
          {/* Tick extends from ruler into content area */}
          <div className="absolute border-t border-zinc-500/45" style={{ left: RULER_W - 2, right: 0 }} />
          {/* Label inside ruler */}
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

      {/* Playhead — horizontal line + left-pointing triangle handle */}
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
