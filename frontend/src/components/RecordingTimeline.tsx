import { useRef, useCallback, useEffect } from 'react'
import type { Recording, MotionEvent } from '../pages/cameraUtils'

interface Props {
  recordings: Recording[]
  motionEvents: MotionEvent[]
  activeRecording: Recording | null
  activeTime: string | null   // ISO UTC: event time (if event active) or recording start
  timezone: string
  onSeek: (recording: Recording, offsetSeconds: number) => void
}

function timeOfDayFraction(isoString: string, timezone: string): number {
  const d = new Date(isoString)
  const timeStr = new Intl.DateTimeFormat('en-GB', {
    timeZone: timezone,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(d)
  const [h, m, s] = timeStr.split(':').map(Number)
  return (h * 3600 + m * 60 + s) / 86400
}

// Narrow vertical strip meant to be embedded in a flex container.
// Takes its height from the parent via align-self: stretch (flex default).
export default function RecordingTimeline({ recordings, motionEvents, activeRecording, activeTime, timezone, onSeek }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const isDragging = useRef(false)

  const recsAsc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))

  const blocks = recsAsc.map((rec, i) => {
    const startFrac = timeOfDayFraction(rec.start, timezone)
    const nextStart = i + 1 < recsAsc.length
      ? new Date(recsAsc[i + 1].start).getTime()
      : new Date(rec.start).getTime() + 5 * 60 * 1000
    const durationSec = (nextStart - new Date(rec.start).getTime()) / 1000
    const heightFrac = Math.min(durationSec / 86400, 1 - startFrac)
    return { rec, startFrac, heightFrac }
  })

  const eventDots = motionEvents.map(ev => ({
    ev,
    frac: timeOfDayFraction(ev.time, timezone),
  }))

  const activeFrac = activeTime ? timeOfDayFraction(activeTime, timezone) : null

  const seekAtY = useCallback((clientY: number) => {
    const el = containerRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const frac = Math.max(0, Math.min(1, (clientY - rect.top) / rect.height))

    let found: typeof blocks[0] | null = null
    for (const block of blocks) {
      if (frac >= block.startFrac && frac < block.startFrac + block.heightFrac) {
        found = block
        break
      }
    }
    if (!found) {
      for (let i = blocks.length - 1; i >= 0; i--) {
        if (blocks[i].startFrac <= frac) { found = blocks[i]; break }
      }
    }
    if (!found || found.rec.is_recording) return

    const offsetSeconds = Math.max(0, (frac - found.startFrac) * 86400)
    onSeek(found.rec, offsetSeconds)
  }, [blocks, onSeek])

  // Native touch listeners to allow preventDefault (React synthetic touch is passive)
  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const onTouchStart = (e: TouchEvent) => {
      isDragging.current = true
      seekAtY(e.touches[0].clientY)
    }
    const onTouchMove = (e: TouchEvent) => {
      if (!isDragging.current) return
      e.preventDefault()
      seekAtY(e.touches[0].clientY)
    }
    const onTouchEnd = () => { isDragging.current = false }
    el.addEventListener('touchstart', onTouchStart, { passive: true })
    el.addEventListener('touchmove', onTouchMove, { passive: false })
    el.addEventListener('touchend', onTouchEnd)
    return () => {
      el.removeEventListener('touchstart', onTouchStart)
      el.removeEventListener('touchmove', onTouchMove)
      el.removeEventListener('touchend', onTouchEnd)
    }
  }, [seekAtY])

  return (
    <div
      ref={containerRef}
      className="relative w-10 shrink-0 border-l border-gray-800 bg-gray-800/20 cursor-pointer select-none"
      title="Linha do tempo — clique ou arraste para navegar"
      onMouseDown={e => { isDragging.current = true; seekAtY(e.clientY) }}
      onMouseMove={e => { if (isDragging.current) seekAtY(e.clientY) }}
      onMouseUp={() => { isDragging.current = false }}
      onMouseLeave={() => { isDragging.current = false }}
    >
      {/* Hour marks at 0h, 6h, 12h, 18h */}
      {[0, 6, 12, 18].map(h => (
        <div
          key={h}
          className="absolute inset-x-0 border-t border-gray-700/50 pointer-events-none"
          style={{ top: `${(h / 24) * 100}%` }}
        >
          {h === 12 && (
            <span className="absolute left-0.5 text-gray-600 leading-none" style={{ fontSize: 8 }}>12</span>
          )}
        </div>
      ))}

      {/* Recording blocks */}
      {blocks.map(({ rec, startFrac, heightFrac }) => {
        const isActive = activeRecording?.filename === rec.filename
        return (
          <div
            key={rec.filename}
            className={`absolute inset-x-0.5 rounded-sm pointer-events-none transition-colors ${
              rec.is_recording
                ? 'bg-red-600/70'
                : isActive
                  ? 'bg-blue-500'
                  : 'bg-blue-700/50'
            }`}
            style={{
              top: `${startFrac * 100}%`,
              height: `max(${heightFrac * 100}%, 2px)`,
            }}
          />
        )
      })}

      {/* Motion event dots */}
      {eventDots.map(({ ev, frac }, i) => (
        <div
          key={ev.id ?? `${ev.time}-${i}`}
          className="absolute w-1.5 h-1.5 rounded-full pointer-events-none"
          style={{
            top: `${frac * 100}%`,
            left: '50%',
            transform: 'translate(-50%, -50%)',
            backgroundColor: ev.color ?? '#fb923c',
          }}
        />
      ))}

      {/* Active position cursor */}
      {activeFrac !== null && (
        <div
          className="absolute inset-x-0 border-t-2 border-blue-400 pointer-events-none"
          style={{ top: `${activeFrac * 100}%` }}
        />
      )}
    </div>
  )
}
