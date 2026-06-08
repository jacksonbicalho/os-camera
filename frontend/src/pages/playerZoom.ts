// Pure math for digital zoom on a video player. The video fills its container at
// scale 1; the transform uses a top-left origin so the model is simple:
//
//   transform: translate(offsetX, offsetY) scale(scale)
//
// `offset` is in container pixels and kept in [-(scale-1)*size, 0] so the scaled
// video always covers the container (no empty gap at the edges).

export interface ZoomState {
  scale: number
  offsetX: number
  offsetY: number
}

export const MIN_SCALE = 1
export const MAX_SCALE = 8

export const IDENTITY: ZoomState = { scale: 1, offsetX: 0, offsetY: 0 }

export function clampScale(scale: number): number {
  if (scale < MIN_SCALE) return MIN_SCALE
  if (scale > MAX_SCALE) return MAX_SCALE
  return scale
}

function clampOffset(offset: number, scale: number, size: number): number {
  const min = -(scale - 1) * size
  const clamped = offset > 0 ? 0 : offset < min ? min : offset
  return clamped === 0 ? 0 : clamped // normalize -0 → 0
}

export function clampState(state: ZoomState, width: number, height: number): ZoomState {
  const scale = clampScale(state.scale)
  return {
    scale,
    offsetX: clampOffset(state.offsetX, scale, width),
    offsetY: clampOffset(state.offsetY, scale, height),
  }
}

// zoomAtPoint multiplies the current scale by `factor`, keeping the content point
// under (cx, cy) — container coordinates — fixed on screen.
export function zoomAtPoint(
  state: ZoomState,
  factor: number,
  cx: number,
  cy: number,
  width: number,
  height: number,
): ZoomState {
  const newScale = clampScale(state.scale * factor)
  const offsetX = cx - ((cx - state.offsetX) / state.scale) * newScale
  const offsetY = cy - ((cy - state.offsetY) / state.scale) * newScale
  return clampState({ scale: newScale, offsetX, offsetY }, width, height)
}

// panBy shifts the offset by (dx, dy), clamped to the edges.
export function panBy(state: ZoomState, dx: number, dy: number, width: number, height: number): ZoomState {
  return clampState(
    { scale: state.scale, offsetX: state.offsetX + dx, offsetY: state.offsetY + dy },
    width,
    height,
  )
}

export function transformStyle(state: ZoomState): string {
  return `translate(${state.offsetX}px, ${state.offsetY}px) scale(${state.scale})`
}

export function isZoomed(state: ZoomState): boolean {
  return state.scale > MIN_SCALE
}
