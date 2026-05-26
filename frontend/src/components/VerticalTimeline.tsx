import React, { useRef, useCallback, useEffect, useState } from 'react'
import type { Recording, MotionEvent } from '../pages/cameraUtils'

interface VerticalTimelineProps {
  recordings: Recording[]
  motionEvents: MotionEvent[]
  activeRecording: Recording | null
  activeTime: string | null
  timezone: string
  onSeek: (recording: Recording, offsetSeconds: number) => void
  onEventClick?: (ev: MotionEvent) => void
  className?: string
  maxHeight?: number
}

const BASE_PX_PER_MIN = 3
const BAR_MAX_W = 44
const LABEL_W = 28
const ZOOM_LEVELS = [1, 2, 4]

export default function VerticalTimeline({
  recordings,
  motionEvents,
  activeRecording,
  activeTime,
  timezone,
  onSeek,
  onEventClick,
  className,
  maxHeight,
}: VerticalTimelineProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const isDragging = useRef(false)
  const [zoom, setZoom] = useState(1)
  const [hoverY, setHoverY] = useState<number | null>(null)
  const zoomRef = useRef(zoom)
  const zoomIdx = ZOOM_LEVELS.indexOf(zoom)

  useEffect(() => {
    zoomRef.current = zoom
  }, [zoom])

  const recsAsc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
  const N = recsAsc.length

  const rangeStartMs = N > 0 ? new Date(recsAsc[0].start).getTime() : 0
  const firstMin = N > 0 ? Math.floor(rangeStartMs / 60_000) : 0

  const lastRecStartMs = N > 0 ? new Date(recsAsc[N - 1].start).getTime() : 0
  const rangeEndMs = lastRecStartMs + 5 * 60_000
  const lastMin = N > 0 ? Math.ceil(rangeEndMs / 60_000) : 0
  const totalMinutes = Math.max(lastMin - firstMin, 1)
  const pxPerMin = BASE_PX_PER_MIN * zoom

  const recRanges = recsAsc.map((rec, i) => {
    const startMs = new Date(rec.start).getTime()
    const endMs = i + 1 < N
      ? new Date(recsAsc[i + 1].start).getTime()
      : startMs + 5 * 60_000
    return {
      rec,
      startMin: Math.floor(startMs / 60_000) - firstMin,
      endMin: Math.ceil(endMs / 60_000) - firstMin,
    }
  })

  const buckets = new Map<number, number>()
  for (const ev of motionEvents) {
    const b = Math.floor(new Date(ev.time).getTime() / 60_000) - firstMin
    if (b >= 0 && b < totalMinutes) {
      buckets.set(b, Math.max(buckets.get(b) ?? 0, ev.score))
    }
  }
  const maxScore = buckets.size > 0 ? Math.max(...buckets.values()) : 1

  const activeMin = activeTime
    ? Math.floor(new Date(activeTime).getTime() / 60_000) - firstMin
    : null

  const fmt = new Intl.DateTimeFormat('pt-BR', {
    timeZone: timezone, hour: '2-digit', minute: '2-digit', hour12: false,
  })

  type TickType = 'hour' | 'half' | 'quarter' | 'five'
  interface Tick { min: number; type: TickType; label?: string }
  const ticks: Tick[] = []
  for (let m = 0; m < totalMinutes; m++) {
    const absMin = firstMin + m
    if (absMin % 60 === 0) {
      ticks.push({ min: m, type: 'hour', label: fmt.format(new Date(absMin * 60_000)) })
    } else if (absMin % 30 === 0) {
      ticks.push({ min: m, type: 'half', label: zoom >= 2 ? fmt.format(new Date(absMin * 60_000)) : undefined })
    } else if (zoom >= 2 && absMin % 15 === 0) {
      ticks.push({ min: m, type: 'quarter' })
    } else if (zoom >= 4 && absMin % 5 === 0) {
      ticks.push({ min: m, type: 'five' })
    }
  }

  const tickStyle: Record<TickType, { color: string; left: number }> = {
    hour:    { color: 'rgba(245,158,11,0.55)', left: 0 },
    half:    { color: 'rgba(245,158,11,0.22)', left: 0 },
    quarter: { color: 'rgba(245,158,11,0.12)', left: LABEL_W },
    five:    { color: 'rgba(245,158,11,0.07)', left: LABEL_W },
  }

  const seekAtY = useCallback((clientY: number) => {
    const container = containerRef.current
    if (!container) return
    const rect = container.getBoundingClientRect()
    const y = clientY - rect.top
    if (y < 0) return
    const px = BASE_PX_PER_MIN * zoomRef.current
    const clickedMin = Math.floor(y / px)
    const targetMs = (firstMin + clickedMin) * 60_000
    if (onEventClick && motionEvents.length > 0) {
      let nearest: MotionEvent | null = null
      let nearestDist = Infinity
      for (const ev of motionEvents) {
        const dist = Math.abs(new Date(ev.time).getTime() - targetMs)
        if (dist < nearestDist) { nearestDist = dist; nearest = ev }
      }
      if (nearest && nearestDist <= 30_000) {
        onEventClick(nearest)
        return
      }
    }
    for (const { rec, startMin, endMin } of recRanges) {
      if (rec.is_recording) continue
      if (clickedMin >= startMin && clickedMin < endMin) {
        const offsetSeconds = (targetMs - new Date(rec.start).getTime()) / 1000
        onSeek(rec, Math.max(0, offsetSeconds))
        return
      }
    }
  }, [firstMin, recRanges, motionEvents, onSeek, onEventClick])

  useEffect(() => {
    const el = scrollRef.current
    if (!el || activeMin === null) return
    const targetY = activeMin * pxPerMin
    const scrollTo = targetY - el.clientHeight / 2
    el.scrollTop = Math.max(0, Math.min(el.scrollHeight - el.clientHeight, scrollTo))
  }, [activeMin, pxPerMin])

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const onTouchStart = (e: TouchEvent) => { seekAtY(e.touches[0].clientY) }
    el.addEventListener('touchstart', onTouchStart, { passive: true })
    return () => el.removeEventListener('touchstart', onTouchStart)
  }, [seekAtY])

  const outerStyle = { maxHeight: maxHeight ?? 'calc(100vh - 10rem)' } as React.CSSProperties

  if (N === 0) {
    return (
      <div className={`w-[72px] shrink-0 rounded-lg bg-zinc-950 border border-zinc-800 ${className ?? ''}`} style={outerStyle} />
    )
  }

  return (
    <div className={`w-[72px] shrink-0 rounded-lg bg-zinc-950 border border-zinc-800 flex flex-col ${className ?? ''}`} style={outerStyle}>
      {/* Zoom controls */}
      <div
        className="flex items-center justify-between px-1 py-1 border-b border-zinc-800 shrink-0"
        onMouseDown={e => e.stopPropagation()}
      >
        <button
          onClick={() => setZoom(z => ZOOM_LEVELS[Math.max(0, ZOOM_LEVELS.indexOf(z) - 1)])}
          disabled={zoomIdx === 0}
          className="w-7 h-7 flex items-center justify-center text-amber-400 hover:text-amber-200 disabled:opacity-25 disabled:cursor-not-allowed transition-colors rounded hover:bg-zinc-800"
          title="Diminuir zoom"
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <circle cx="11" cy="11" r="6" />
            <line x1="7" y1="11" x2="15" y2="11" />
            <line x1="16.5" y1="16.5" x2="21" y2="21" />
          </svg>
        </button>
        <span className="text-[9px] text-amber-400/60 tabular-nums font-medium">{zoom}×</span>
        <button
          onClick={() => setZoom(z => ZOOM_LEVELS[Math.min(ZOOM_LEVELS.length - 1, ZOOM_LEVELS.indexOf(z) + 1)])}
          disabled={zoomIdx === ZOOM_LEVELS.length - 1}
          className="w-7 h-7 flex items-center justify-center text-amber-400 hover:text-amber-200 disabled:opacity-25 disabled:cursor-not-allowed transition-colors rounded hover:bg-zinc-800"
          title="Aumentar zoom"
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <circle cx="11" cy="11" r="6" />
            <line x1="11" y1="8" x2="11" y2="14" />
            <line x1="8" y1="11" x2="14" y2="11" />
            <line x1="16.5" y1="16.5" x2="21" y2="21" />
          </svg>
        </button>
      </div>

      {/* Scrollable area */}
      <div
        ref={scrollRef}
        className="flex-1 min-h-0 overflow-y-auto"
        style={{ scrollbarWidth: 'none' }}
      >
        <div
          ref={containerRef}
          className="relative cursor-pointer"
          style={{ width: 72, height: totalMinutes * pxPerMin }}
          onMouseDown={e => { isDragging.current = true; seekAtY(e.clientY) }}
          onMouseMove={e => {
            const rect = containerRef.current?.getBoundingClientRect()
            if (rect) setHoverY(Math.max(0, e.clientY - rect.top))
            if (isDragging.current) seekAtY(e.clientY)
          }}
          onMouseUp={() => { isDragging.current = false }}
          onMouseLeave={() => { isDragging.current = false; setHoverY(null) }}
        >
          {/* Recording backgrounds */}
          {recRanges.map(({ rec, startMin, endMin }) => {
            const isActive = activeRecording?.filename === rec.filename
            return (
              <div
                key={rec.filename}
                className="absolute left-0 right-0 pointer-events-none"
                style={{
                  top: startMin * pxPerMin,
                  height: (endMin - startMin) * pxPerMin,
                  backgroundColor: rec.is_recording
                    ? 'rgba(239,68,68,0.15)'
                    : isActive
                      ? '#1e3a5f'
                      : '#1f2937',
                }}
              />
            )
          })}

          {/* Motion intensity bars */}
          {Array.from(buckets.entries()).map(([min, score]) => {
            const ratio = score / maxScore
            return (
              <div
                key={min}
                className="absolute pointer-events-none"
                style={{
                  left: LABEL_W,
                  top: min * pxPerMin,
                  width: ratio * BAR_MAX_W,
                  height: pxPerMin,
                  backgroundColor: '#f97316',
                  opacity: Math.max(0.3, ratio),
                }}
              />
            )
          })}

          {/* Tick lines */}
          {ticks.map(({ min, type }) => {
            const { color, left } = tickStyle[type]
            return (
              <div
                key={`tick-${type}-${min}`}
                className="absolute right-0 pointer-events-none"
                style={{ top: min * pxPerMin, left, height: 1, backgroundColor: color }}
              />
            )
          })}

          {/* Tick labels */}
          {ticks.filter(t => t.label).map(({ min, type, label }) => (
            <span
              key={`label-${type}-${min}`}
              className="absolute pointer-events-none leading-none select-none"
              style={{
                fontSize: type === 'hour' ? 8 : 7,
                top: min * pxPerMin + 1,
                left: 1,
                width: LABEL_W - 2,
                overflow: 'hidden',
                whiteSpace: 'nowrap',
                color: type === 'hour' ? 'rgba(245,158,11,0.65)' : 'rgba(245,158,11,0.35)',
              }}
            >
              {label}
            </span>
          ))}

          {/* Active position indicator */}
          {activeMin !== null && activeMin >= 0 && activeMin < totalMinutes && (
            <div
              className="absolute left-0 right-0 pointer-events-none z-20"
              style={{
                top: activeMin * pxPerMin,
                height: 2,
                backgroundColor: '#3b82f6',
                boxShadow: '0 0 4px 1px rgba(59,130,246,0.5)',
              }}
            />
          )}

          {/* Hover time indicator */}
          {hoverY !== null && (() => {
            const hoverMin = Math.floor(hoverY / pxPerMin)
            if (hoverMin < 0 || hoverMin >= totalMinutes) return null
            const hoverMs = (firstMin + hoverMin) * 60_000
            const label = fmt.format(new Date(hoverMs))
            const flipLabel = hoverY > (totalMinutes * pxPerMin) - 14
            return (
              <div className="absolute left-0 right-0 pointer-events-none z-30" style={{ top: hoverY }}>
                <div className="absolute left-0 right-0" style={{ height: 1, backgroundColor: 'rgba(255,255,255,0.55)' }} />
                <span
                  className="absolute select-none leading-none"
                  style={{
                    fontSize: 8,
                    left: 1,
                    top: flipLabel ? -10 : 2,
                    color: 'rgba(255,255,255,0.9)',
                    textShadow: '0 0 3px #000, 0 0 6px #000',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {label}
                </span>
              </div>
            )
          })()}
        </div>
      </div>
    </div>
  )
}
