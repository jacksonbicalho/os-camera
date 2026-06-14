import React, { useRef, useCallback, useEffect, useMemo, useState } from 'react'
import type { Recording, MotionEvent } from '../pages/cameraUtils'
import { ZoomIn, ZoomOut } from './Icons'

interface VerticalTimelineProps {
  recordings: Recording[]
  motionEvents: MotionEvent[]
  activeRecording: Recording | null
  activeTime: string | null
  timezone: string
  sortOrder?: 'asc' | 'desc'
  onSeek: (recording: Recording, offsetSeconds: number) => void
  // Lightweight preview while dragging the pointer (seek the frame, no panel/play side
  // effects). Falls back to onSeek when not provided.
  onScrub?: (recording: Recording, offsetSeconds: number) => void
  // Called when the pointer is over a position with no recording (a gap), with the UTC
  // timestamp (ms) under the pointer — lets the player show a "no recording" message.
  onGap?: (timestampMs: number) => void
  onEventClick?: (ev: MotionEvent) => void
  className?: string
  maxHeight?: number
}

const BASE_PX_PER_MIN = 3
const BAR_MAX_W = 44
const LABEL_W = 56 // largura da coluna de rótulo — comporta "HH:MM:SS" legível
const TL_W = LABEL_W + BAR_MAX_W + 2 // largura total da régua
const MIN_ACTIVE_PX = 26 // altura mínima do bloco da gravação ativa (visível em qualquer zoom, acima da banda do ponteiro)
const ZOOM_LEVELS = [1, 2, 4, 8, 16, 32, 64]
const CHUNK_FALLBACK_MS = 5 * 60_000
const DAY_MINUTES = 24 * 60

// UTC ms of local midnight (00:00 in `tz`) for the day containing `sampleMs`.
function localMidnightMs(sampleMs: number, tz: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: tz, hourCycle: 'h23',
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).formatToParts(new Date(sampleMs))
  const get = (t: string) => Number(parts.find(p => p.type === t)?.value)
  const localAsUTC = Date.UTC(get('year'), get('month') - 1, get('day'), get('hour'), get('minute'), get('second'))
  const offsetMs = localAsUTC - sampleMs
  const midnightAsUTC = Date.UTC(get('year'), get('month') - 1, get('day'), 0, 0, 0)
  return midnightAsUTC - offsetMs
}

export default function VerticalTimeline({
  recordings,
  motionEvents,
  activeRecording,
  activeTime,
  timezone,
  sortOrder = 'desc',
  onSeek,
  onScrub,
  onGap,
  onEventClick,
  className,
  maxHeight,
}: VerticalTimelineProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const isDragging = useRef(false)
  const [zoom, setZoom] = useState(1)
  const [dragging, setDragging] = useState(false)
  const [dragY, setDragY] = useState<number | null>(null)
  // Position the pointer was dropped on a gap (no recording) — it rests there until a
  // real recording becomes active again. `since` is the active recording at drop time;
  // when the active recording changes the pin is ignored (derived, no effect).
  const [pinned, setPinned] = useState<{ ms: number; since: string | null } | null>(null)
  const activeFilenameRef = useRef<string | null>(null)
  const [viewportH, setViewportH] = useState(0)
  const [scrollTop, setScrollTop] = useState(0)
  // Effective px/min (incl. fit scaling) shared with the pointer-event handlers
  // via a ref so they don't seek with a stale, unscaled value.
  const pxPerMinRef = useRef(0)

  // Measure the scroll viewport so the content can fill the full height ("fit").
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const update = () => setViewportH(el.clientHeight)
    update()
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(update)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  const zoomIdx = ZOOM_LEVELS.indexOf(zoom)

  const recsAsc = useMemo(
    () => [...recordings].sort((a, b) => a.filename.localeCompare(b.filename)),
    [recordings],
  )
  const N = recsAsc.length

  // Typical chunk length, inferred from the spacing between consecutive recordings
  // (median delta). Used to size each chunk's real coverage so genuine gaps between
  // recordings show up instead of being stretched to the next chunk.
  const chunkMs = useMemo(() => {
    if (recsAsc.length < 2) return CHUNK_FALLBACK_MS
    const deltas: number[] = []
    for (let i = 1; i < recsAsc.length; i++) {
      deltas.push(new Date(recsAsc[i].start).getTime() - new Date(recsAsc[i - 1].start).getTime())
    }
    deltas.sort((a, b) => a - b)
    return deltas[Math.floor((deltas.length - 1) / 2)] || CHUNK_FALLBACK_MS
  }, [recsAsc])

  // The ruler spans the full local day (00:00–24:00) of the recordings.
  const dayStartMs = N > 0 ? localMidnightMs(new Date(recsAsc[0].start).getTime(), timezone) : 0
  const firstMin = N > 0 ? Math.round(dayStartMs / 60_000) : 0
  const lastRecStartMs = N > 0 ? new Date(recsAsc[N - 1].start).getTime() : 0
  const totalMinutes = DAY_MINUTES
  // Start expanded: at zoom 1 the content fills the full viewport height (fit);
  // higher zoom levels scale up from there (content taller than the box → scrolls).
  const fitPxPerMin = viewportH > 0 ? viewportH / totalMinutes : 0
  const pxPerMin = Math.max(BASE_PX_PER_MIN, fitPxPerMin) * zoom
  useEffect(() => { pxPerMinRef.current = pxPerMin }, [pxPerMin])

  const recRanges = useMemo(() => recsAsc.map((rec, i) => {
    const startMs = new Date(rec.start).getTime()
    // Cap each chunk at its real length; only stretch to the next chunk when they are
    // contiguous. A clearly larger interval leaves an uncovered gap.
    let endMs: number
    if (i + 1 < N) {
      const nextStart = new Date(recsAsc[i + 1].start).getTime()
      endMs = nextStart - startMs > chunkMs * 1.5 ? startMs + chunkMs : nextStart
    } else {
      endMs = startMs + chunkMs
    }
    return {
      rec,
      startMin: Math.floor(startMs / 60_000) - firstMin,
      endMin: Math.ceil(endMs / 60_000) - firstMin,
    }
  }), [recsAsc, N, firstMin, chunkMs])

  const buckets = new Map<number, number>()
  for (const ev of motionEvents) {
    const b = Math.floor(new Date(ev.time).getTime() / 60_000) - firstMin
    if (b >= 0 && b < totalMinutes) {
      buckets.set(b, Math.max(buckets.get(b) ?? 0, ev.score))
    }
  }
  const maxScore = buckets.size > 0 ? Math.max(...buckets.values()) : 1


  // Single mapping shared by ticks, bars, the pointer and seeking — must match
  // rangeToY (recording blocks) so the pointer's time lines up with what's drawn.
  const minToY = (min: number) =>
    sortOrder === 'desc' ? (totalMinutes - min) * pxPerMin : min * pxPerMin
  const rangeToY = (startMin: number, endMin: number) =>
    sortOrder === 'desc' ? (totalMinutes - endMin) * pxPerMin : startMin * pxPerMin

  const fmtSec = new Intl.DateTimeFormat('pt-BR', {
    timeZone: timezone, hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  })

  type TickType = 'hour' | 'half' | 'quarter' | 'five' | 'minute' | 'second'
  interface Tick { min: number; type: TickType; label?: string }
  const ticks: Tick[] = []
  for (let m = 0; m < totalMinutes; m++) {
    const absMin = firstMin + m
    if (absMin % 60 === 0) {
      ticks.push({ min: m, type: 'hour', label: fmtSec.format(new Date(absMin * 60_000)) })
    } else if (absMin % 30 === 0) {
      ticks.push({ min: m, type: 'half', label: zoom >= 2 ? fmtSec.format(new Date(absMin * 60_000)) : undefined })
    } else if (zoom >= 2 && absMin % 15 === 0) {
      ticks.push({ min: m, type: 'quarter', label: zoom >= 8 ? fmtSec.format(new Date(absMin * 60_000)) : undefined })
    } else if (zoom >= 4 && absMin % 5 === 0) {
      ticks.push({ min: m, type: 'five', label: zoom >= 8 ? fmtSec.format(new Date(absMin * 60_000)) : undefined })
    } else if (zoom >= 8) {
      // Every minute once zoomed in — turns the timeline into a fine ruler so the
      // pointer can be dragged to a precise second within a chunk.
      ticks.push({ min: m, type: 'minute', label: zoom >= 16 ? fmtSec.format(new Date(absMin * 60_000)) : undefined })
    }
  }

  // Marcas de SEGUNDO entre os minutos: aparecem assim que houver espaço legível
  // — critério ADAPTATIVO por pixels/segundo, não por um nível de zoom fixo. Pega
  // o passo mais fino (1/5/10/15/30s) cujo rótulo ainda fica legível. Geradas só
  // na janela visível para não criar milhares de elementos no dia todo.
  const MIN_LABEL_PX = 14
  const pxPerSec = pxPerMin / 60
  let secStep = 0
  for (const step of [1, 5, 10, 15, 30]) {
    if (step * pxPerSec >= MIN_LABEL_PX) { secStep = step; break }
  }
  if (secStep > 0) {
    const winPx = viewportH > 0 ? viewportH : 1200 // fallback quando o viewport ainda não foi medido
    const a = sortOrder === 'desc' ? totalMinutes - (scrollTop + winPx) / pxPerMin : scrollTop / pxPerMin
    const b = sortOrder === 'desc' ? totalMinutes - scrollTop / pxPerMin : (scrollTop + winPx) / pxPerMin
    const fromM = Math.max(0, Math.floor(Math.min(a, b)) - 1)
    const toM = Math.min(totalMinutes, Math.ceil(Math.max(a, b)) + 1)
    for (let m = fromM; m < toM; m++) {
      const baseMs = (firstMin + m) * 60_000
      for (let s = secStep; s < 60; s += secStep) {
        ticks.push({ min: m + s / 60, type: 'second', label: fmtSec.format(new Date(baseMs + s * 1000)) })
      }
    }
  }

  const tickStyle: Record<TickType, { color: string; left: number }> = {
    hour:    { color: 'rgba(245,158,11,0.55)', left: 0 },
    half:    { color: 'rgba(245,158,11,0.22)', left: 0 },
    quarter: { color: 'rgba(245,158,11,0.12)', left: LABEL_W },
    five:    { color: 'rgba(245,158,11,0.07)', left: LABEL_W },
    minute:  { color: 'rgba(245,158,11,0.05)', left: LABEL_W },
    second:  { color: 'rgba(245,158,11,0.10)', left: LABEL_W },
  }

  const seekAtY = useCallback((clientY: number) => {
    const container = containerRef.current
    if (!container) return
    const rect = container.getBoundingClientRect()
    const y = clientY - rect.top
    if (y < 0) return
    const px = pxPerMinRef.current
    const clickedMin = sortOrder === 'desc' ? totalMinutes - y / px : y / px
    const targetMs = (firstMin + clickedMin) * 60_000
    if (onEventClick) {
      if (motionEvents.length > 0) {
        let nearest: MotionEvent | null = null
        let nearestDist = Infinity
        for (const ev of motionEvents) {
          const dist = Math.abs(new Date(ev.time).getTime() - targetMs)
          if (dist < nearestDist) { nearestDist = dist; nearest = ev }
        }
        if (nearest) onEventClick(nearest)
      }
      return
    }
    for (const { rec, startMin, endMin } of recRanges) {
      if (rec.is_recording) continue
      if (clickedMin >= startMin && clickedMin < endMin) {
        const offsetSeconds = (targetMs - new Date(rec.start).getTime()) / 1000
        onSeek(rec, Math.max(0, offsetSeconds))
        return
      }
    }
  }, [firstMin, totalMinutes, sortOrder, recRanges, motionEvents, onSeek, onEventClick])

  // Resolve a vertical cursor position to the time under the pointer and the recording
  // there (if any). The pointer is free — it does NOT snap; gaps return rec = null.
  const targetAtY = useCallback((clientY: number): { y: number; ms: number; rec: Recording | null; offsetSeconds: number } | null => {
    const container = containerRef.current
    if (!container) return null
    const px = pxPerMinRef.current || BASE_PX_PER_MIN
    const rect = container.getBoundingClientRect()
    const y = Math.max(0, Math.min(totalMinutes * px, clientY - rect.top))
    const minFloat = sortOrder === 'desc' ? (totalMinutes - y / px) : y / px
    const ms = (firstMin + minFloat) * 60_000
    const hit = recRanges.find(r => !r.rec.is_recording && minFloat >= r.startMin && minFloat < r.endMin)
    const rec = hit ? hit.rec : null
    const offsetSeconds = hit ? Math.max(0, (ms - new Date(hit.rec.start).getTime()) / 1000) : 0
    return { y, ms, rec, offsetSeconds }
  }, [recRanges, totalMinutes, sortOrder, firstMin])

  useEffect(() => {
    if (!dragging) return
    // Free scrub: the pointer follows the cursor anywhere. Over a recording it previews
    // the frame; over a gap it reports the time so the player can show "no recording".
    const preview = onScrub ?? onSeek
    let lastY: number | null = null
    let raf = 0

    const apply = (clientY: number) => {
      const t = targetAtY(clientY)
      if (!t) return
      setDragY(t.y)
      if (t.rec) preview(t.rec, t.offsetSeconds)
      else onGap?.(t.ms)
    }

    const onMove = (e: MouseEvent) => { lastY = e.clientY; apply(e.clientY) }

    // While the cursor sits near the top/bottom edge of the viewport, keep scrolling so
    // the pointer can be dragged past the visible area on the expanded ruler.
    const EDGE = 30
    const STEP = 14
    const tick = () => {
      const sc = scrollRef.current
      if (sc && lastY !== null) {
        const r = sc.getBoundingClientRect()
        if (lastY < r.top + EDGE && sc.scrollTop > 0) {
          sc.scrollTop = Math.max(0, sc.scrollTop - STEP)
          apply(lastY)
        } else if (lastY > r.bottom - EDGE && sc.scrollTop < sc.scrollHeight - sc.clientHeight) {
          sc.scrollTop = Math.min(sc.scrollHeight - sc.clientHeight, sc.scrollTop + STEP)
          apply(lastY)
        }
      }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)

    const onUp = (e: MouseEvent) => {
      const t = targetAtY(e.clientY)
      if (t) {
        if (t.rec) { onSeek(t.rec, t.offsetSeconds); setPinned(null) }
        else { onGap?.(t.ms); setPinned({ ms: t.ms, since: activeFilenameRef.current }) }
      }
      setDragging(false)
      setDragY(null)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      cancelAnimationFrame(raf)
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [dragging, targetAtY, onSeek, onScrub, onGap])

  const activeFilename = activeRecording?.filename ?? null
  useEffect(() => { activeFilenameRef.current = activeFilename }, [activeFilename])
  // The pin is only honored while the active recording hasn't changed since the drop.
  const pinnedMs = pinned && pinned.since === activeFilename ? pinned.ms : null

  // Keep the pointer centered: re-scrolls when the pointer's resting position changes
  // (active recording, pinned gap, or the initial default) AND when the zoom or the
  // measured viewport change — so the pointer is always on-screen on the 24h ruler.
  const activeMsForScroll = activeTime ? new Date(activeTime).getTime() : null
  const scrollMs = pinnedMs ?? activeMsForScroll ?? (N > 0 ? lastRecStartMs : null)
  useEffect(() => {
    const el = scrollRef.current
    if (!el || scrollMs === null || dragging) return
    const minFloat = scrollMs / 60_000 - firstMin
    const targetY = sortOrder === 'desc' ? (totalMinutes - minFloat) * pxPerMin : minFloat * pxPerMin
    const scrollTo = targetY - el.clientHeight / 2
    el.scrollTop = Math.max(0, Math.min(el.scrollHeight - el.clientHeight, scrollTo))
  }, [scrollMs, pxPerMin, totalMinutes, sortOrder, firstMin, dragging, viewportH])


  // Draggable pointer position: while dragging follows the cursor (dragY); otherwise
  // rests on the active time with sub-minute precision (for the HH:MM:SS readout).
  const activeMsForPointer = activeTime ? new Date(activeTime).getTime() : null
  let pointerY: number | null = null
  let pointerMs: number | null = null
  if (dragY !== null) {
    pointerY = dragY
    const minFloat = sortOrder === 'desc' ? (totalMinutes - dragY / pxPerMin) : dragY / pxPerMin
    pointerMs = (firstMin + minFloat) * 60_000
  } else if (pinnedMs !== null) {
    const minFloat = pinnedMs / 60_000 - firstMin
    pointerY = sortOrder === 'desc' ? (totalMinutes - minFloat) * pxPerMin : minFloat * pxPerMin
    pointerMs = pinnedMs
  } else if (activeMsForPointer !== null) {
    const minFloat = activeMsForPointer / 60_000 - firstMin
    pointerY = sortOrder === 'desc' ? (totalMinutes - minFloat) * pxPerMin : minFloat * pxPerMin
    pointerMs = activeMsForPointer
  } else if (N > 0) {
    // No active playback yet (e.g. on initial load): default to the newest recording
    // so the pointer is always visible and ready to drag.
    const minFloat = lastRecStartMs / 60_000 - firstMin
    pointerY = sortOrder === 'desc' ? (totalMinutes - minFloat) * pxPerMin : minFloat * pxPerMin
    pointerMs = lastRecStartMs
  }

  // Default: fill the layout column (h-full class), respecting any footer. When a
  // maxHeight is provided, cap to it instead.
  const outerStyle = (maxHeight !== undefined ? { maxHeight } : {}) as React.CSSProperties

  if (N === 0) {
    return (
      <div id="vertical-timeline" className={`w-[102px] shrink-0 h-full rounded-lg bg-background border border-border ${className ?? ''}`} style={outerStyle} />
    )
  }

  return (
    <div id="vertical-timeline" className={`w-[102px] shrink-0 h-full rounded-lg bg-background border border-border flex flex-col ${className ?? ''}`} style={outerStyle}>
      {/* Zoom controls */}
      <div
        className="flex items-center justify-between px-1 py-1 border-b border-border shrink-0"
        onMouseDown={e => e.stopPropagation()}
      >
        <button
          onClick={() => setZoom(z => ZOOM_LEVELS[Math.max(0, ZOOM_LEVELS.indexOf(z) - 1)])}
          disabled={zoomIdx === 0}
          className="w-7 h-7 flex items-center justify-center text-amber-400 hover:text-amber-200 disabled:opacity-25 disabled:cursor-not-allowed transition-colors rounded hover:bg-surface-2"
          title="Diminuir zoom"
        >
          <ZoomOut className="w-4 h-4" />
        </button>
        <span className="text-[9px] text-amber-400/60 tabular-nums font-medium">{zoom}×</span>
        <button
          onClick={() => setZoom(z => ZOOM_LEVELS[Math.min(ZOOM_LEVELS.length - 1, ZOOM_LEVELS.indexOf(z) + 1)])}
          disabled={zoomIdx === ZOOM_LEVELS.length - 1}
          className="w-7 h-7 flex items-center justify-center text-amber-400 hover:text-amber-200 disabled:opacity-25 disabled:cursor-not-allowed transition-colors rounded hover:bg-surface-2"
          title="Aumentar zoom"
        >
          <ZoomIn className="w-4 h-4" />
        </button>
      </div>

      {/* Scrollable area */}
      <div
        ref={scrollRef}
        className="flex-1 min-h-0 overflow-y-auto"
        style={{ scrollbarWidth: 'none' }}
        onScroll={e => setScrollTop(e.currentTarget.scrollTop)}
      >
        <div
          ref={containerRef}
          className="relative cursor-pointer"
          style={{ width: TL_W, height: totalMinutes * pxPerMin }}
          onMouseDown={e => { isDragging.current = true; seekAtY(e.clientY) }}
          onMouseMove={e => { if (isDragging.current) seekAtY(e.clientY) }}
          onMouseUp={() => { isDragging.current = false }}
          onMouseLeave={() => { isDragging.current = false }}
        >
          {/* Recording backgrounds — a ativa é desenhada por último para ficar por
              cima dos chunks-irmãos do mesmo minuto (bloco azul sempre visível). */}
          {[...recRanges]
            .sort((a, b) => Number(a.rec.filename === activeFilename) - Number(b.rec.filename === activeFilename))
            .map(({ rec, startMin, endMin }) => {
            const isActive = activeRecording?.filename === rec.filename
            const rawH = (endMin - startMin) * pxPerMin
            // a gravação ativa tem altura mínima para ficar visível em qualquer
            // zoom (no zoom baixo a fatia seria fina demais e some sob o ponteiro).
            const h = isActive ? Math.max(rawH, MIN_ACTIVE_PX) : rawH
            return (
              <div
                key={rec.filename}
                data-rec={rec.filename}
                className="absolute left-0 right-0 pointer-events-none"
                style={{
                  top: rangeToY(startMin, endMin),
                  height: h,
                  backgroundColor: rec.is_recording
                    ? 'rgba(239,68,68,0.15)'
                    : isActive
                      ? 'rgba(37,99,235,0.35)'
                      : 'var(--color-surface)',
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
                  top: Math.min(minToY(min), minToY(min + 1)),
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
                style={{ top: minToY(min), left, height: type === 'hour' ? 2 : 1, backgroundColor: color }}
              />
            )
          })}

          {/* Tick labels */}
          {ticks.filter(t => t.label).map(({ min, type, label }) => (
            <span
              key={`label-${type}-${min}`}
              className="absolute pointer-events-none leading-none select-none tabular-nums"
              style={{
                fontSize: type === 'hour' ? 11 : 10,
                fontWeight: type === 'hour' ? 600 : 400,
                top: minToY(min) + 1,
                left: 2,
                width: LABEL_W - 3,
                overflow: 'hidden',
                whiteSpace: 'nowrap',
                color: type === 'hour' ? 'rgb(252,211,77)' : 'rgba(252,211,77,0.75)',
              }}
            >
              {label}
            </span>
          ))}

          {/* Draggable pointer / scrubber — full-width red band (Frigate style). The band
              is clamped to stay fully on the ruler at the edges; the time stays exact. */}
          {pointerY !== null && pointerMs !== null && (() => {
            const BAND_H = 20
            let bandTop = pointerY - BAND_H / 2
            // Keep the band inside the visible viewport so it stays on-screen while the
            // pointer is dragged past the edges (the displayed time still tracks dragY).
            if (viewportH > 0) {
              bandTop = Math.max(scrollTop, Math.min(scrollTop + viewportH - BAND_H, bandTop))
            }
            bandTop = Math.max(0, Math.min(totalMinutes * pxPerMin - BAND_H, bandTop))
            return (
              <div
                id="timeline-pointer"
                className="absolute left-0 right-0 z-30 cursor-grab active:cursor-grabbing flex items-center justify-center rounded shadow-md"
                style={{
                  top: bandTop,
                  height: BAND_H,
                  backgroundColor: '#dc2626',
                  transition: dragging ? 'none' : 'top 0.08s ease-out',
                }}
                onMouseDown={e => {
                  e.stopPropagation()
                  setDragY(pointerY)
                  setDragging(true)
                }}
              >
                <span
                  id="timeline-pointer-time"
                  className="text-white tabular-nums font-semibold leading-none select-none whitespace-nowrap"
                  style={{ fontSize: 12 }}
                >
                  {fmtSec.format(new Date(pointerMs))}
                </span>
              </div>
            )
          })()}
        </div>
      </div>
    </div>
  )
}
