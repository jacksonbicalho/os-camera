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

// Width in px reserved for the time ruler on the left
const RULER_W = 22

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
    rangeMin <= 10  ? 2  :
    rangeMin <= 30  ? 5  :
    rangeMin <= 120 ? 15 :
    rangeMin <= 360 ? 30 :
    rangeMin <= 720 ? 60 : 120

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

  if (recsAsc.length === 0) {
    return <div className="relative w-full h-full border-l border-gray-800 bg-gray-900/40" />
  }

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
      {/* Ruler background strip */}
      <div
        className="absolute inset-y-0 left-0 bg-gray-800/60 border-r border-gray-700/50"
        style={{ width: RULER_W }}
      />

      {/* Tick marks + labels */}
      {labels.map(({ f, label }) => (
        <div
          key={label}
          className="absolute inset-x-0 pointer-events-none"
          style={{ top: `${f * 100}%` }}
        >
          {/* Tick extends from ruler into content area */}
          <div
            className="absolute border-t border-gray-500/70"
            style={{ left: RULER_W - 4, right: 0 }}
          />
          {/* Label inside ruler */}
          <span
            className="absolute text-gray-300 leading-none whitespace-nowrap"
            style={{ fontSize: 6.5, left: 2, top: 2 }}
          >
            {label}
          </span>
        </div>
      ))}

      {/* Recording blocks (right of ruler) */}
      {blocks.map(({ rec, startFrac, heightFrac }) => {
        const isActive = activeRecording?.filename === rec.filename
        return (
          <div
            key={rec.filename}
            className={`absolute rounded-[2px] pointer-events-none transition-colors ${
              rec.is_recording
                ? 'bg-red-500/60 border border-red-400/40'
                : isActive
                  ? 'bg-blue-500/80 border border-blue-400/60'
                  : 'bg-blue-800/50 border border-blue-700/30'
            }`}
            style={{
              top:    `${startFrac * 100}%`,
              height: `max(${heightFrac * 100}%, 2px)`,
              left:   RULER_W + 2,
              right:  2,
            }}
          />
        )
      })}

      {/* Motion event dots */}
      {eventDots.map(({ ev, f }, i) => (
        <div
          key={ev.id ?? `${ev.time}-${i}`}
          className="absolute w-1.5 h-1.5 rounded-full pointer-events-none"
          style={{
            top:       `${f * 100}%`,
            right:     6,
            transform: 'translateY(-50%)',
            backgroundColor: ev.color ?? '#fb923c',
          }}
        />
      ))}

      {/* Playhead — horizontal line + left-pointing triangle handle */}
      {activeFrac !== null && (
        <div
          className="absolute inset-x-0 z-20 pointer-events-none"
          style={{ top: `${activeFrac * 100}%` }}
        >
          {/* Line across content area */}
          <div
            className="absolute border-t-2 border-blue-400"
            style={{ left: RULER_W, right: 0 }}
          />
          {/* Triangle handle sitting on the ruler / content boundary */}
          <div
            className="absolute"
            style={{
              left:        RULER_W - 1,
              top:         -5,
              width:       0,
              height:      0,
              borderTop:    '5px solid transparent',
              borderBottom: '5px solid transparent',
              borderLeft:   '7px solid #60a5fa',
            }}
          />
          {/* Small circle at the tip for easier grabbing */}
          <div
            className="absolute rounded-full bg-blue-400"
            style={{ left: RULER_W - 5, top: -3, width: 6, height: 6 }}
          />
        </div>
      )}
    </div>
  )
}
