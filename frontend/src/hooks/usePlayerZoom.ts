import { useCallback, useEffect, useRef, useState } from 'react'
import {
  IDENTITY,
  isZoomed as isZoomedState,
  panBy,
  transformStyle,
  zoomAtPoint,
  type ZoomState,
} from '../pages/playerZoom'

const WHEEL_FACTOR = 1.15
const DRAG_THRESHOLD = 3

export interface PlayerZoom {
  // setContainer: callback ref for the wrapper div; (re)binds the non-passive
  // wheel listener whenever the node changes (e.g. live ↔ recording switch).
  setContainer: (node: HTMLDivElement | null) => void
  onPointerDown: (e: React.PointerEvent) => void
  onPointerMove: (e: React.PointerEvent) => void
  onPointerUp: (e: React.PointerEvent) => void
  isZoomed: boolean
  scale: number
  reset: () => void
  // consumeDrag returns (and clears) whether the last pointer interaction was a
  // pan — used to suppress the click that would otherwise toggle playback.
  consumeDrag: () => boolean
}

// usePlayerZoom wires scroll-to-zoom (at the cursor) and drag-to-pan onto a video
// player. The transform is applied imperatively to the <video> returned by
// getVideoEl so overlays and controls in the wrapper stay put.
export function usePlayerZoom(getVideoEl: () => HTMLVideoElement | null): PlayerZoom {
  const [zoom, setZoom] = useState<ZoomState>(IDENTITY)
  const nodeRef = useRef<HTMLDivElement | null>(null)
  const cleanupRef = useRef<(() => void) | null>(null)
  const drag = useRef<{ x: number; y: number; moved: boolean } | null>(null)
  const draggedRef = useRef(false)

  // Apply the current transform to whichever video is active.
  useEffect(() => {
    const v = getVideoEl()
    if (!v) return
    v.style.transformOrigin = '0 0'
    v.style.transform = transformStyle(zoom)
  }, [zoom, getVideoEl])

  const setContainer = useCallback((node: HTMLDivElement | null) => {
    cleanupRef.current?.()
    cleanupRef.current = null
    nodeRef.current = node
    if (!node) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      const rect = node.getBoundingClientRect()
      const cx = e.clientX - rect.left
      const cy = e.clientY - rect.top
      const factor = e.deltaY < 0 ? WHEEL_FACTOR : 1 / WHEEL_FACTOR
      setZoom(z => zoomAtPoint(z, factor, cx, cy, rect.width, rect.height))
    }
    node.addEventListener('wheel', onWheel, { passive: false })
    cleanupRef.current = () => node.removeEventListener('wheel', onWheel)
  }, [])

  useEffect(() => () => cleanupRef.current?.(), [])

  const onPointerDown = useCallback((e: React.PointerEvent) => {
    if (!isZoomedState(zoom)) return
    // Não inicia pan sobre botões (reset, play/pause) — deixa o clique passar.
    if ((e.target as HTMLElement).closest('button')) return
    drag.current = { x: e.clientX, y: e.clientY, moved: false }
    e.currentTarget.setPointerCapture?.(e.pointerId)
  }, [zoom])

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    const d = drag.current
    if (!d) return
    const dx = e.clientX - d.x
    const dy = e.clientY - d.y
    if (Math.abs(dx) > DRAG_THRESHOLD || Math.abs(dy) > DRAG_THRESHOLD) d.moved = true
    d.x = e.clientX
    d.y = e.clientY
    const rect = nodeRef.current?.getBoundingClientRect()
    if (!rect) return
    setZoom(z => panBy(z, dx, dy, rect.width, rect.height))
  }, [])

  const onPointerUp = useCallback((e: React.PointerEvent) => {
    if (drag.current?.moved) draggedRef.current = true
    drag.current = null
    e.currentTarget.releasePointerCapture?.(e.pointerId)
  }, [])

  const reset = useCallback(() => setZoom(IDENTITY), [])

  const consumeDrag = useCallback(() => {
    const d = draggedRef.current
    draggedRef.current = false
    return d
  }, [])

  return {
    setContainer,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    isZoomed: isZoomedState(zoom),
    scale: zoom.scale,
    reset,
    consumeDrag,
  }
}
